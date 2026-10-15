package app

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	companyaccessapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
)

// H7 (notification-rules-runtime-h7-2026-10-15): company alert prefs must decide delivery without
// dropping reminders. Q-23 missing row → today's behaviour; AHA-08 email/in-app evaluated separately;
// Q-24 company schedule gates deadline reminders only; RMC-08 department_focal maps company departments.

type countingEmailSender struct {
	mu    sync.Mutex
	calls int
}

func (c *countingEmailSender) SendReminderEmail(_ context.Context, _ string, _ map[string]any, _ []string, _ string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return "msg-h7", nil
}

func (c *countingEmailSender) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

type recordingInApp struct {
	mu      sync.Mutex
	calls   int
	done    chan struct{}
	created int   // what DeliverForReminderDispatch reports (default 1)
	err     error // error returned by DeliverForReminderDispatch
}

func newRecordingInApp() *recordingInApp {
	return &recordingInApp{done: make(chan struct{}, 8), created: 1}
}

func (r *recordingInApp) CreateForReminderDispatch(_ context.Context, _ DispatchCandidate) error {
	r.mu.Lock()
	r.calls++
	r.mu.Unlock()
	r.done <- struct{}{}
	return nil
}

func (r *recordingInApp) DeliverForReminderDispatch(ctx context.Context, c DispatchCandidate) (int, error) {
	_ = r.CreateForReminderDispatch(ctx, c)
	return r.created, r.err
}

// legacyInApp can create but not report delivery (no InAppNotificationDeliverer).
type legacyInApp struct{ calls int }

func (l *legacyInApp) CreateForReminderDispatch(context.Context, DispatchCandidate) error {
	l.calls++
	return nil
}

func (r *recordingInApp) waitCalls(t *testing.T, want int) int {
	t.Helper()
	deadline := time.After(500 * time.Millisecond)
	for got := 0; got < want; got++ {
		select {
		case <-r.done:
		case <-deadline:
			r.mu.Lock()
			defer r.mu.Unlock()
			return r.calls
		}
	}
	// Give an unexpected extra goroutine a moment to show up.
	time.Sleep(50 * time.Millisecond)
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func companySchedules(offsets ...int) []SchedulePref {
	out := make([]SchedulePref, 0, len(offsets))
	for i := range offsets {
		o := offsets[i]
		out = append(out, SchedulePref{OffsetDays: &o, Enabled: true})
	}
	return out
}

func prefsDoc(companyID string, email, inApp bool, offsets ...int) *AlertChannelPrefsDocument {
	return &AlertChannelPrefsDocument{
		RuleCode:   AlertChannelPrefsRuleCode,
		CompanyID:  companyID,
		Status:     "active",
		Version:    1,
		EventScope: []string{"deadline", "workflow"},
		Channels: map[string]ChannelPref{
			"email":  {Enabled: email},
			"in_app": {Enabled: inApp},
			"zalo":   {Enabled: false},
			"sms":    {Enabled: false},
		},
		Schedules: companySchedules(offsets...),
	}
}

func dispatchSvc(t *testing.T, reader NotificationRulesReader, consumer bool, c DispatchCandidate, sender EmailSender, inApp InAppNotificationCreator, extra ...ServiceOption) Service {
	t.Helper()
	opts := []ServiceOption{
		WithEmailSender(sender),
		WithNotificationRulesFoundation(reader, NewNotificationRulesEvaluator(reader, consumer)),
	}
	if inApp != nil {
		opts = append(opts, WithInAppCreator(inApp))
	}
	opts = append(opts, extra...)
	return NewService(fakeConfigRepo{}, &fakeOccurrenceRepo{
		candidates: []DispatchCandidate{c},
		occ: ReminderOccurrenceDTO{
			OccurrenceID:   c.OccurrenceID,
			IdempotencyKey: c.IdempotencyKey,
			Status:         ReminderStatusPending,
		},
	}, &fakeAttemptRepo{}, opts...)
}

func deadlineCandidate(id string, daysBefore int) DispatchCandidate {
	now := time.Now().UTC()
	return DispatchCandidate{
		OccurrenceID:    id,
		IdempotencyKey:  id,
		TemplateCode:    "reminder.deadline",
		RecipientEmails: []string{"user@example.com"},
		CompanyID:       "c1",
		ScheduledAt:     now,
		DeadlineAt:      now.Add(time.Duration(daysBefore) * 24 * time.Hour),
	}
}

// T1 (Q-23): no saved row → evaluator applies the defaults instead of RULE_MISSING.
func TestNotificationRulesEvaluator_NoRowAppliesDefaults(t *testing.T) {
	eval := NewNotificationRulesEvaluator(&fakeNotificationRulesReader{}, true)
	in := evalInputWithSchedule("c1", "deadline") // offset 7
	dec, err := eval.Evaluate(context.Background(), in)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !dec.Allowed || dec.SkipReason != "" {
		t.Fatalf("no row must not block delivery: allowed=%v skip=%q", dec.Allowed, dec.SkipReason)
	}
	if dec.AuditMetadata["defaults_applied"] != true {
		t.Fatalf("defaults_applied audit flag missing: %+v", dec.AuditMetadata)
	}
	// D-H7-1: a company that never saved prefs keeps today's delivery — any offset, no recipient filter.
	in.DueAt = in.ScheduledAt.Add(5 * 24 * time.Hour)
	dec, err = eval.Evaluate(context.Background(), in)
	if err != nil || !dec.Allowed {
		t.Fatalf("default prefs must not gate a custom offset: allowed=%v skip=%q err=%v", dec.Allowed, dec.SkipReason, err)
	}
	if len(dec.RecipientPolicies) != 0 {
		t.Fatalf("default prefs must not filter recipients, got policies %v", dec.RecipientPolicies)
	}
}

// T3 (Q-24): workflow-step milestones (3 and 1 days) are not gated by the company schedule;
// deadline reminders still are.
func TestNotificationRulesEvaluator_WorkflowOffset1_NotGatedByCompanySchedules(t *testing.T) {
	reader := &fakeNotificationRulesReader{byCompany: map[string]*AlertChannelPrefsDocument{
		"c1": prefsDoc("c1", true, true, 30, 7, 3, 0),
	}}
	eval := NewNotificationRulesEvaluator(reader, true)
	now := time.Now().UTC()
	workflow := EvaluateInput{CompanyID: "c1", EventType: "workflow", Channel: "email", ScheduledAt: now, DueAt: now.Add(24 * time.Hour), AsOf: now}
	dec, err := eval.Evaluate(context.Background(), workflow)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !dec.Allowed {
		t.Fatalf("workflow offset 1 must be allowed, skip=%q", dec.SkipReason)
	}
	deadline := workflow
	deadline.EventType = "deadline"
	dec, err = eval.Evaluate(context.Background(), deadline)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if dec.Allowed || dec.SkipReason != SkipReasonRuleScheduleNotMatched {
		t.Fatalf("deadline offset 1 must stay gated: allowed=%v skip=%q", dec.Allowed, dec.SkipReason)
	}
}

// T5 (AHA-08): in-app only → no SMTP call, one in-app notification, occurrence SENT.
func TestDispatchDue_FlagOn_InAppOnly_NoEmailOneInApp(t *testing.T) {
	reader := &fakeNotificationRulesReader{byCompany: map[string]*AlertChannelPrefsDocument{
		"c1": prefsDoc("c1", false, true, 7),
	}}
	sender := &countingEmailSender{}
	inApp := newRecordingInApp()
	svc := dispatchSvc(t, reader, true, deadlineCandidate("occ-inapp", 7), sender, inApp)
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("DispatchDueOccurrences: %v", err)
	}
	if res.Sent != 1 || res.Skipped != 0 {
		t.Fatalf("expected sent=1 skipped=0, got %+v", res)
	}
	if got := sender.count(); got != 0 {
		t.Fatalf("email must not be sent for in-app only prefs, sender calls=%d", got)
	}
	if got := inApp.waitCalls(t, 1); got != 1 {
		t.Fatalf("expected 1 in-app notification, got %d", got)
	}
}

// T6 (AHA-08): in-app disabled → email only.
func TestDispatchDue_FlagOn_InAppDisabled_NoInApp(t *testing.T) {
	reader := &fakeNotificationRulesReader{byCompany: map[string]*AlertChannelPrefsDocument{
		"c1": prefsDoc("c1", true, false, 7),
	}}
	sender := &countingEmailSender{}
	inApp := newRecordingInApp()
	svc := dispatchSvc(t, reader, true, deadlineCandidate("occ-email", 7), sender, inApp)
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("DispatchDueOccurrences: %v", err)
	}
	if res.Sent != 1 || sender.count() != 1 {
		t.Fatalf("expected one email, got res=%+v sender=%d", res, sender.count())
	}
	if got := inApp.waitCalls(t, 0); got != 0 {
		t.Fatalf("in-app disabled must not create in-app notifications, got %d", got)
	}
}

// T7 (Q-23): consumer on, company without a row → reminder is still sent.
func TestDispatchDue_FlagOn_NoRow_SendsWithDefaults(t *testing.T) {
	sender := &countingEmailSender{}
	inApp := newRecordingInApp()
	svc := dispatchSvc(t, &fakeNotificationRulesReader{}, true, deadlineCandidate("occ-norow", 5), sender, inApp)
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("DispatchDueOccurrences: %v", err)
	}
	if res.Sent != 1 || sender.count() != 1 {
		t.Fatalf("missing prefs must keep today's delivery, got res=%+v sender=%d", res, sender.count())
	}
	if got := inApp.waitCalls(t, 1); got != 1 {
		t.Fatalf("expected 1 in-app notification, got %d", got)
	}
}

// T8 (RMC-08): department_focal maps the global step department onto the company department.
func TestApplyRecipientPolicies_DepartmentFocalResolvesCompanyDepartment(t *testing.T) {
	querier := &fakeMembershipQuerier{
		companyDepartments: map[string]string{"c1:legal": "d_legal_c1"},
		departmentEmails:   map[string][]string{"c1:d_legal_c1": {"focal@co.com"}},
	}
	steps := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-legal": {StepID: "step-legal", DepartmentID: "legal"},
	}}
	c := DispatchCandidate{CompanyID: "c1", ScopeType: ScopeTypeWorkflowStep, ScopeID: "step-legal", WorkflowInstanceID: "wi-1"}
	filtered, err := applyRecipientPolicies(context.Background(), querier, nil, steps, nil, "c1",
		[]string{"focal@co.com", "other@co.com"}, []string{"department_focal"}, c)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(filtered) != 1 || filtered[0] != "focal@co.com" {
		t.Fatalf("department_focal must resolve the company department, got %v", filtered)
	}
}

type recordingMetrics struct {
	mu        sync.Mutex
	counters  []map[string]string
	inAppOnly []string
}

func (m *recordingMetrics) IncCounter(name string, tags map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch name {
	case shadowMetric:
		cp := map[string]string{}
		for k, v := range tags {
			cp[k] = v
		}
		m.counters = append(m.counters, cp)
	case inAppOnlyMetric:
		m.inAppOnly = append(m.inAppOnly, tags["result"])
	}
}

func (m *recordingMetrics) inAppOnlyResults() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.inAppOnly...)
}

func (m *recordingMetrics) ObserveLatency(string, int64, map[string]string) {}

func (m *recordingMetrics) shadow() []map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]map[string]string(nil), m.counters...)
}

// In-app only needs an in-app channel: without an in-app creator, email keeps the reminder.
func TestDispatchDue_FlagOn_InAppOnly_WithoutInAppCreator_SendsEmail(t *testing.T) {
	reader := &fakeNotificationRulesReader{byCompany: map[string]*AlertChannelPrefsDocument{
		"c1": prefsDoc("c1", false, true, 7),
	}}
	sender := &countingEmailSender{}
	svc := dispatchSvc(t, reader, true, deadlineCandidate("occ-noinapp", 7), sender, nil)
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("DispatchDueOccurrences: %v", err)
	}
	if res.Sent != 1 || sender.count() != 1 {
		t.Fatalf("expected the email fallback, got res=%+v sender=%d", res, sender.count())
	}
}

// T9: consumer off + shadow on → delivery exactly as today, the difference is recorded.
func TestDispatchDue_Shadow_ConsumerOff_DeliveryUnchanged(t *testing.T) {
	reader := &fakeNotificationRulesReader{byCompany: map[string]*AlertChannelPrefsDocument{
		"c1": prefsDoc("c1", false, true, 7), // prefs would drop email
	}}
	sender := &countingEmailSender{}
	inApp := newRecordingInApp()
	metrics := &recordingMetrics{}
	var logBuf strings.Builder
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	svc := dispatchSvc(t, reader, false, deadlineCandidate("occ-shadow", 7), sender, inApp,
		WithNotificationRulesShadow(reader, true), WithMetrics(metrics), WithDispatchLogger(logger))
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("DispatchDueOccurrences: %v", err)
	}
	if res.Sent != 1 || sender.count() != 1 {
		t.Fatalf("shadow must not change delivery: res=%+v sender=%d", res, sender.count())
	}
	if got := inApp.waitCalls(t, 1); got != 1 {
		t.Fatalf("in-app as today expected 1, got %d", got)
	}
	got := metrics.shadow()
	if len(got) != 1 || got[0]["outcome"] != ShadowOutcomeWouldSkipEmail {
		t.Fatalf("shadow outcome = %v, want %s", got, ShadowOutcomeWouldSkipEmail)
	}
	logged := logBuf.String()
	if !strings.Contains(logged, `"msg":"notification_rules_shadow"`) || !strings.Contains(logged, `"outcome":"would_skip_email"`) {
		t.Fatalf("shadow log missing: %s", logged)
	}
	if strings.Contains(logged, "@") {
		t.Fatalf("shadow log must not contain email addresses: %s", logged)
	}
}

func TestDispatchDue_Shadow_NoRow_Same(t *testing.T) {
	reader := &fakeNotificationRulesReader{}
	metrics := &recordingMetrics{}
	svc := dispatchSvc(t, reader, false, deadlineCandidate("occ-shadow-same", 5), &countingEmailSender{}, newRecordingInApp(),
		WithNotificationRulesShadow(reader, true), WithMetrics(metrics))
	if _, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10); err != nil {
		t.Fatalf("DispatchDueOccurrences: %v", err)
	}
	got := metrics.shadow()
	if len(got) != 1 || got[0]["outcome"] != ShadowOutcomeSame {
		t.Fatalf("a company without prefs must keep the same delivery, shadow=%v", got)
	}
}

// T9: a failing shadow evaluation never touches delivery.
func TestDispatchDue_Shadow_EvaluatorError_DeliveryUnchanged(t *testing.T) {
	runtimeReader := &fakeNotificationRulesReader{}
	shadowReader := &fakeNotificationRulesReader{err: errors.New("db down")}
	sender := &countingEmailSender{}
	metrics := &recordingMetrics{}
	svc := dispatchSvc(t, runtimeReader, false, deadlineCandidate("occ-shadow-err", 7), sender, newRecordingInApp(),
		WithNotificationRulesShadow(shadowReader, true), WithMetrics(metrics))
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("DispatchDueOccurrences: %v", err)
	}
	if res.Sent != 1 || sender.count() != 1 {
		t.Fatalf("shadow error must not change delivery: res=%+v sender=%d", res, sender.count())
	}
	got := metrics.shadow()
	if len(got) != 1 || got[0]["outcome"] != ShadowOutcomeError {
		t.Fatalf("shadow outcome = %v, want %s", got, ShadowOutcomeError)
	}
}

// T10: once the consumer decides delivery, shadow does not run.
func TestDispatchDue_Shadow_ConsumerOn_NotRecorded(t *testing.T) {
	reader := &fakeNotificationRulesReader{}
	metrics := &recordingMetrics{}
	svc := dispatchSvc(t, reader, true, deadlineCandidate("occ-shadow-on", 7), &countingEmailSender{}, newRecordingInApp(),
		WithNotificationRulesShadow(reader, true), WithMetrics(metrics))
	if _, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10); err != nil {
		t.Fatalf("DispatchDueOccurrences: %v", err)
	}
	if got := metrics.shadow(); len(got) != 0 {
		t.Fatalf("shadow must be off while the consumer is enabled, got %v", got)
	}
}

func TestClassifyShadow(t *testing.T) {
	send := func(r ...string) DispatchDecisionResult {
		return DispatchDecisionResult{Recipients: r, AllowInApp: true}
	}
	cases := []struct {
		name           string
		actual, shadow DispatchDecisionResult
		outcome        string
		reason         string
	}{
		{"same", send("a@x", "b@x"), send("B@x", "a@x"), ShadowOutcomeSame, "none"},
		{"fewer", send("a@x", "b@x"), send("a@x"), ShadowOutcomeRecipientsChanged, "fewer"},
		{"different", send("a@x"), send("c@x"), ShadowOutcomeRecipientsChanged, "different"},
		{"would skip", send("a@x"), DispatchDecisionResult{Skip: true, SkipReason: SkipReasonRuleScheduleNotMatched}, ShadowOutcomeWouldSkip, SkipReasonRuleScheduleNotMatched},
		{"both skip", DispatchDecisionResult{Skip: true, SkipReason: SkipReasonRecipientEmpty}, DispatchDecisionResult{Skip: true, SkipReason: SkipReasonRecipientEmpty}, ShadowOutcomeBothSkip, SkipReasonRecipientEmpty},
		{"in-app dropped", send("a@x"), DispatchDecisionResult{Recipients: []string{"a@x"}}, ShadowOutcomeWouldSkipInApp, SkipReasonChannelDisabled},
		{"error", send("a@x"), DispatchDecisionResult{EvalErr: errors.New("x")}, ShadowOutcomeError, "eval_error"},
	}
	for _, tc := range cases {
		outcome, reason := classifyShadow(tc.actual, tc.shadow)
		if outcome != tc.outcome || reason != tc.reason {
			t.Errorf("%s: got %s/%s want %s/%s", tc.name, outcome, reason, tc.outcome, tc.reason)
		}
	}
}

// Parity with companyaccess.DefaultAlertChannelPrefsPayload (what the UI shows as defaults), except
// recipient policies which are intentionally empty here (D-H7-1).
func TestDefaultAlertChannelPrefsDocument_ParityWithCompanyAccessDefaults(t *testing.T) {
	payload := companyaccessapp.DefaultAlertChannelPrefsPayload("m1")
	ui, err := ParseAlertChannelPrefsDocument("c1", AlertChannelPrefsRuleCode, "active", payload)
	if err != nil {
		t.Fatalf("parse companyaccess defaults: %v", err)
	}
	def := defaultAlertChannelPrefsDocument("c1")
	for _, ch := range []string{"email", "in_app", "zalo", "sms"} {
		if ui.Channels[ch].Enabled != def.Channels[ch].Enabled {
			t.Errorf("channel %s: ui=%v runtime=%v", ch, ui.Channels[ch].Enabled, def.Channels[ch].Enabled)
		}
	}
	if strings.Join(ui.EventScope, ",") != strings.Join(def.EventScope, ",") {
		t.Errorf("event scope: ui=%v runtime=%v", ui.EventScope, def.EventScope)
	}
	if len(ui.Schedules) != len(def.Schedules) {
		t.Fatalf("schedules: ui=%d runtime=%d", len(ui.Schedules), len(def.Schedules))
	}
	for i := range ui.Schedules {
		u, d := ui.Schedules[i], def.Schedules[i]
		if u.Kind != d.Kind || u.Enabled != d.Enabled || u.PremiumOnly != d.PremiumOnly || (u.OffsetDays == nil) != (d.OffsetDays == nil) ||
			(u.OffsetDays != nil && *u.OffsetDays != *d.OffsetDays) {
			t.Errorf("schedule %d differs: ui=%+v runtime=%+v", i, u, d)
		}
	}
	if len(ui.RecipientPolicies) == 0 || len(def.RecipientPolicies) != 0 {
		t.Errorf("D-H7-1: UI defaults list policies (%v), runtime defaults must not filter (%v)", ui.RecipientPolicies, def.RecipientPolicies)
	}
}

// PERF-01: in-app only is delivered synchronously; nothing created → email fallback, never lost.
func TestDispatchDue_FlagOn_InAppOnly_NothingCreated_FallsBackToEmail(t *testing.T) {
	for _, tc := range []struct {
		name    string
		created int
		err     error
	}{
		{"no matching user", 0, nil},
		{"write error", 0, errors.New("insert failed")},
		{"partial error", 1, errors.New("one insert failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &fakeNotificationRulesReader{byCompany: map[string]*AlertChannelPrefsDocument{
				"c1": prefsDoc("c1", false, true, 7),
			}}
			sender := &countingEmailSender{}
			inApp := newRecordingInApp()
			inApp.created, inApp.err = tc.created, tc.err
			metrics := &recordingMetrics{}
			svc := dispatchSvc(t, reader, true, deadlineCandidate("occ-fallback", 7), sender, inApp, WithMetrics(metrics))
			res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
			if err != nil {
				t.Fatalf("DispatchDueOccurrences: %v", err)
			}
			if res.Sent != 1 || sender.count() != 1 {
				t.Fatalf("expected the email fallback, got res=%+v sender=%d", res, sender.count())
			}
			if got := metrics.inAppOnlyResults(); len(got) != 1 || got[0] != "email_fallback" {
				t.Fatalf("in-app-only result = %v, want [email_fallback]", got)
			}
		})
	}
}

// PERF-01: a creator that cannot report delivery never gets in-app-only reminders.
func TestDispatchDue_FlagOn_InAppOnly_CreatorWithoutDelivery_SendsEmail(t *testing.T) {
	reader := &fakeNotificationRulesReader{byCompany: map[string]*AlertChannelPrefsDocument{
		"c1": prefsDoc("c1", false, true, 7),
	}}
	sender := &countingEmailSender{}
	legacy := &legacyInApp{}
	svc := dispatchSvc(t, reader, true, deadlineCandidate("occ-legacy", 7), sender, legacy)
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("DispatchDueOccurrences: %v", err)
	}
	if res.Sent != 1 || sender.count() != 1 {
		t.Fatalf("expected email, got res=%+v sender=%d", res, sender.count())
	}
}

// PERF-02: a skipped row (stays PENDING, listed again next tick) is not shadow-counted.
func TestDispatchDue_Shadow_SkippedRowNotCounted(t *testing.T) {
	reader := &fakeNotificationRulesReader{}
	metrics := &recordingMetrics{}
	c := deadlineCandidate("occ-shadow-skip", 7)
	c.RecipientEmails = nil // no recipients and no resolver → RECIPIENT_EMPTY skip
	svc := dispatchSvc(t, reader, false, c, &countingEmailSender{}, newRecordingInApp(),
		WithNotificationRulesShadow(reader, true), WithMetrics(metrics))
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("DispatchDueOccurrences: %v", err)
	}
	if res.Skipped != 1 {
		t.Fatalf("expected a skip, got %+v", res)
	}
	if got := metrics.shadow(); len(got) != 0 {
		t.Fatalf("skipped rows must not be shadow-counted, got %v", got)
	}
}
