package app

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"
)

type fakeNotificationRulesReader struct {
	byCompany map[string]*AlertChannelPrefsDocument
	err       error
}

func (f *fakeNotificationRulesReader) GetCompanyAlertPrefs(_ context.Context, companyID string) (*AlertChannelPrefsDocument, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.byCompany == nil {
		return nil, nil
	}
	return f.byCompany[companyID], nil
}

func validPrefsDoc(companyID string, emailEnabled bool) *AlertChannelPrefsDocument {
	offset7 := 7
	return &AlertChannelPrefsDocument{
		RuleCode:   AlertChannelPrefsRuleCode,
		CompanyID:  companyID,
		Status:     "active",
		Version:    1,
		EventScope: []string{"deadline", "workflow"},
		Channels: map[string]ChannelPref{
			"email":  {Enabled: emailEnabled},
			"in_app": {Enabled: true},
			"zalo":   {Enabled: false},
			"sms":    {Enabled: false},
		},
		Schedules: []SchedulePref{
			{OffsetDays: &offset7, Enabled: true},
		},
		RecipientPolicies: []string{"department_focal"},
	}
}

func validPrefsDocNoPolicies(companyID string, emailEnabled bool) *AlertChannelPrefsDocument {
	doc := validPrefsDoc(companyID, emailEnabled)
	doc.RecipientPolicies = nil
	return doc
}

func evalInputWithSchedule(companyID, eventType string) EvaluateInput {
	now := time.Now().UTC()
	due := now.Add(7 * 24 * time.Hour)
	return EvaluateInput{
		CompanyID:   companyID,
		EventType:   eventType,
		Channel:     "email",
		DueAt:       due,
		ScheduledAt: now,
		AsOf:        now,
	}
}

func TestNotificationRulesEvaluator_FlagOff(t *testing.T) {
	reader := &fakeNotificationRulesReader{
		byCompany: map[string]*AlertChannelPrefsDocument{
			"c1": validPrefsDoc("c1", true),
		},
	}
	eval := NewNotificationRulesEvaluator(reader, false)
	dec, err := eval.Evaluate(context.Background(), EvaluateInput{
		CompanyID: "c1",
		EventType: "deadline",
		Channel:   "email",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if dec.SkipReason != SkipReasonConsumerDisabled {
		t.Fatalf("skip_reason=%q want %q", dec.SkipReason, SkipReasonConsumerDisabled)
	}
	if dec.Allowed {
		t.Fatal("allowed must be false when consumer disabled")
	}
}

func TestNotificationRulesEvaluator_NoRules(t *testing.T) {
	eval := NewNotificationRulesEvaluator(&fakeNotificationRulesReader{}, true)
	dec, err := eval.Evaluate(context.Background(), EvaluateInput{CompanyID: "c1", EventType: "deadline", Channel: "email"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if dec.Layer1Applied {
		t.Fatal("layer1_applied should be false without row")
	}
	if dec.Allowed {
		t.Fatal("allowed should be false without prefs row")
	}
	if dec.SkipReason != SkipReasonRuleMissing {
		t.Fatalf("skip_reason=%q want %q", dec.SkipReason, SkipReasonRuleMissing)
	}
}

func TestNotificationRulesEvaluator_DisabledChannel(t *testing.T) {
	eval := NewNotificationRulesEvaluator(&fakeNotificationRulesReader{
		byCompany: map[string]*AlertChannelPrefsDocument{
			"c1": validPrefsDoc("c1", false),
		},
	}, true)
	dec, err := eval.Evaluate(context.Background(), EvaluateInput{CompanyID: "c1", EventType: "deadline", Channel: "email"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if dec.Allowed {
		t.Fatal("allowed must be false")
	}
	if dec.SkipReason != SkipReasonChannelDisabled {
		t.Fatalf("skip_reason=%q want %q", dec.SkipReason, SkipReasonChannelDisabled)
	}
}

func TestNotificationRulesEvaluator_EnabledChannel(t *testing.T) {
	eval := NewNotificationRulesEvaluator(&fakeNotificationRulesReader{
		byCompany: map[string]*AlertChannelPrefsDocument{
			"c1": validPrefsDoc("c1", true),
		},
	}, true)
	in := evalInputWithSchedule("c1", "deadline")
	in.IdempotencyKey = "occ-1"
	dec, err := eval.Evaluate(context.Background(), in)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !dec.Allowed {
		t.Fatalf("allowed=false skip=%q", dec.SkipReason)
	}
	if dec.IdempotencyKey != "occ-1" {
		t.Fatalf("idempotency key=%q", dec.IdempotencyKey)
	}
	if _, ok := dec.AuditMetadata["rule_code"]; !ok {
		t.Fatal("audit metadata missing rule_code")
	}
}

func TestNotificationRulesEvaluator_EventScopeExcluded(t *testing.T) {
	doc := validPrefsDoc("c1", true)
	doc.EventScope = []string{"workflow"}
	eval := NewNotificationRulesEvaluator(&fakeNotificationRulesReader{
		byCompany: map[string]*AlertChannelPrefsDocument{"c1": doc},
	}, true)
	dec, err := eval.Evaluate(context.Background(), EvaluateInput{CompanyID: "c1", EventType: "deadline", Channel: "email"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if dec.SkipReason != SkipReasonEventScopeExcluded {
		t.Fatalf("skip_reason=%q", dec.SkipReason)
	}
}

func TestNotificationRulesEvaluator_MalformedReaderError(t *testing.T) {
	eval := NewNotificationRulesEvaluator(&fakeNotificationRulesReader{err: errors.New("db down")}, true)
	_, err := eval.Evaluate(context.Background(), EvaluateInput{CompanyID: "c1", EventType: "deadline", Channel: "email"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNotificationRulesEvaluator_EmptyRecipientPolicyNoPanic(t *testing.T) {
	doc := validPrefsDoc("c1", true)
	doc.RecipientPolicies = nil
	eval := NewNotificationRulesEvaluator(&fakeNotificationRulesReader{
		byCompany: map[string]*AlertChannelPrefsDocument{"c1": doc},
	}, true)
	dec, err := eval.Evaluate(context.Background(), evalInputWithSchedule("c1", "deadline"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !dec.Allowed {
		t.Fatalf("allowed=false skip=%q", dec.SkipReason)
	}
}

func TestNotificationRulesEvaluator_AuditMetadataNoSecrets(t *testing.T) {
	eval := NewNotificationRulesEvaluator(&fakeNotificationRulesReader{
		byCompany: map[string]*AlertChannelPrefsDocument{
			"c1": validPrefsDoc("c1", true),
		},
	}, true)
	dec, _ := eval.Evaluate(context.Background(), evalInputWithSchedule("c1", "deadline"))
	for k, v := range dec.AuditMetadata {
		kl := strings.ToLower(k)
		if strings.Contains(kl, "password") || strings.Contains(kl, "token") || strings.Contains(kl, "secret") {
			t.Fatalf("unexpected sensitive metadata key %q", k)
		}
		if s, ok := v.(string); ok {
			sl := strings.ToLower(s)
			if strings.Contains(sl, "password") || strings.Contains(sl, "bearer") {
				t.Fatalf("unexpected sensitive metadata value in %q", k)
			}
		}
	}
}

func TestParseAlertChannelPrefsDocument_DeterministicChannels(t *testing.T) {
	payload := map[string]any{
		"version": 1,
		"channels": map[string]any{
			"email":  map[string]any{"enabled": true},
			"in_app": map[string]any{"enabled": false},
		},
		"schedules": []any{
			map[string]any{"offset_days": 7, "enabled": true},
			map[string]any{"offset_days": 0, "enabled": true},
		},
		"recipient_policies": []any{"company_admin", "assignee"},
	}
	doc, err := ParseAlertChannelPrefsDocument("c1", AlertChannelPrefsRuleCode, "active", payload)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	active := activeChannelsFromDoc(doc)
	sort.Strings(active)
	if len(active) != 1 || active[0] != "email" {
		t.Fatalf("active=%v", active)
	}
}

func TestPrepareDispatch_FlagOff_IgnoresNotificationRules(t *testing.T) {
	reader := &fakeNotificationRulesReader{
		byCompany: map[string]*AlertChannelPrefsDocument{
			"c1": validPrefsDoc("c1", false),
		},
	}
	eval := NewNotificationRulesEvaluator(reader, false)
	svc := NewService(nil, nil, nil,
		WithNotificationRulesFoundation(reader, eval),
	).(*service)

	c := DispatchCandidate{
		CompanyID:        "c1",
		DisclosureTypeID: "dt1",
		TemplateCode:     "reminder.deadline",
		RecipientEmails:  []string{"a@example.com"},
		ScheduledAt:      time.Now().UTC(),
	}
	out := svc.prepareDispatch(context.Background(), c, time.Now().UTC())
	if out.skip {
		t.Fatal("prepareDispatch should not skip when consumer flag is OFF")
	}
}

func TestDispatchDue_BackwardCompat_WithNotificationRulesFoundationWired(t *testing.T) {
	reader := &fakeNotificationRulesReader{
		byCompany: map[string]*AlertChannelPrefsDocument{
			"c1": validPrefsDoc("c1", false),
		},
	}
	eval := NewNotificationRulesEvaluator(reader, false)
	alertRepo := &fakeAlertConfigRepoForDispatch{rows: []AlertTemplateConfig{
		{TypeID: "dt1", AlertKind: string(AlertKindDeadline), Enabled: true, TemplateKey: "tpl"},
	}}
	candidates := []DispatchCandidate{{
		OccurrenceID:     "occ1",
		IdempotencyKey:   "occ1",
		DisclosureTypeID: "dt1",
		RecipientEmails:  []string{"user@example.com"},
		TemplateCode:     "legacy",
		ScheduledAt:      time.Now().UTC().Add(-time.Minute),
	}}
	svc := NewService(fakeConfigRepo{}, &fakeOccurrenceRepo{
		candidates: candidates,
		occ: ReminderOccurrenceDTO{
			OccurrenceID:   "occ1",
			IdempotencyKey: "occ1",
			Status:         ReminderStatusPending,
		},
	}, &fakeAttemptRepo{},
		WithAlertConfigRepo(alertRepo),
		WithEmailSender(stubEmailSender{}),
		WithNotificationRulesFoundation(reader, eval),
	)
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("DispatchDueOccurrences: %v", err)
	}
	if res.Processed == 0 {
		t.Fatal("expected processed > 0")
	}
	if res.Sent == 0 && res.Skipped > 0 {
		t.Fatal("notification_rules must not skip dispatch when consumer flag is OFF")
	}
}

type stubEmailSender struct{}

func (stubEmailSender) SendReminderEmail(_ context.Context, _ string, _ map[string]any, _ []string, _ string) (string, error) {
	return "msg-1", nil
}
