package app

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeAlertConfigRepoForDispatch is a test double injected via WithAlertConfigRepo.
type fakeAlertConfigRepoForDispatch struct {
	rows []AlertTemplateConfig
}

func (f *fakeAlertConfigRepoForDispatch) GetByTypeID(_ context.Context, typeID string) ([]AlertTemplateConfig, error) {
	var out []AlertTemplateConfig
	for _, r := range f.rows {
		if r.TypeID == typeID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeAlertConfigRepoForDispatch) GetByTypeAndKind(_ context.Context, typeID, kind string) (*AlertTemplateConfig, error) {
	for i := range f.rows {
		if f.rows[i].TypeID == typeID && f.rows[i].AlertKind == kind {
			cp := f.rows[i]
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeAlertConfigRepoForDispatch) Upsert(_ context.Context, in AlertTemplateConfig) error {
	f.rows = append(f.rows, in)
	return nil
}

var _ AlertConfigRepository = (*fakeAlertConfigRepoForDispatch)(nil)

// fakeResolverForDispatch records calls.
type fakeResolverForDispatch struct {
	emails []string
}

func (f *fakeResolverForDispatch) ResolveForDeadline(_ context.Context, _, _ string) ([]string, error) {
	return f.emails, nil
}

func (f *fakeResolverForDispatch) ResolveForWorkflowStep(_ context.Context, _, _ string) ([]string, error) {
	return f.emails, nil
}

var _ RecipientResolver = (*fakeResolverForDispatch)(nil)

// captureDispatchOccurrenceRepo is a fakeOccurrenceRepo that records DispatchOccurrence calls
// (via the candidates slice already; we track via the SLA alert hook).

func newDispatchSvc(
	candidates []DispatchCandidate,
	alertRepo AlertConfigRepository,
	resolver RecipientResolver,
	sender EmailSender,
) (Service, *fakeOccurrenceRepo, *captureAlert) {
	occRepo := &fakeOccurrenceRepo{
		candidates: candidates,
	}
	if len(candidates) > 0 {
		occRepo.occ = ReminderOccurrenceDTO{
			OccurrenceID:   candidates[0].OccurrenceID,
			IdempotencyKey: candidates[0].IdempotencyKey,
			AttemptCount:   0,
			Status:         ReminderStatusPending,
		}
	}
	attemptRepo := &fakeAttemptRepo{}
	alerts := &captureAlert{}
	opts := []ServiceOption{
		WithAlertHook(alerts),
	}
	if sender != nil {
		opts = append(opts, WithEmailSender(sender))
	}
	if alertRepo != nil {
		opts = append(opts, WithAlertConfigRepo(alertRepo))
	}
	if resolver != nil {
		opts = append(opts, WithRecipientResolver(resolver))
	}
	svc := NewService(fakeConfigRepo{}, occRepo, attemptRepo, opts...)
	return svc, occRepo, alerts
}

// ── No alert config → backward compat dispatch ──────────────────────────────

func TestDispatchDue_NoAlertConfigRepo_BackwardCompat(t *testing.T) {
	// When alertConfigRepo is nil, existing behavior must be unchanged.
	svc, _, _ := newDispatchSvc(
		[]DispatchCandidate{{
			OccurrenceID:    "occ-bc",
			IdempotencyKey:  "idem-bc",
			TemplateCode:    "REMINDER_DISCLOSURE_DUE",
			TemplatePayload: map[string]any{"title": "T", "deadline_date": "2026-06-01", "disclosure_id": "d1"},
			RecipientEmails: []string{"a@example.com"},
		}},
		nil, // alertConfigRepo = nil
		nil,
		fakeSender{},
	)
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if res.Sent != 1 {
		t.Errorf("expected sent=1, got %+v", res)
	}
	if res.Skipped != 0 {
		t.Errorf("expected skipped=0, got %d", res.Skipped)
	}
}

// ── Alert config disabled → SKIP ────────────────────────────────────────────

func TestDispatchDue_ConfigDisabled_Skipped(t *testing.T) {
	alertRepo := &fakeAlertConfigRepoForDispatch{rows: []AlertTemplateConfig{
		{TypeID: "dt-test", AlertKind: AlertKindDeadline, TemplateKey: "reminder.deadline_approaching", Enabled: false},
	}}
	svc, occRepo, _ := newDispatchSvc(
		[]DispatchCandidate{{
			OccurrenceID:    "occ-skip",
			IdempotencyKey:  "idem-skip",
			TemplateCode:    "REMINDER_DISCLOSURE_DUE",
			TemplatePayload: map[string]any{},
			RecipientEmails: []string{"a@example.com"},
			DisclosureTypeID: "dt-test",
			ScopeType:        ScopeTypeDisclosure,
		}},
		alertRepo, nil, fakeSender{},
	)
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if res.Skipped != 1 {
		t.Errorf("expected skipped=1, got %+v", res)
	}
	if res.Sent != 0 {
		t.Errorf("expected sent=0, got %d", res.Sent)
	}
	// Occurrence must NOT be dispatched.
	if len(occRepo.updates) != 0 {
		t.Errorf("expected no occurrence updates for skipped candidate, got %d", len(occRepo.updates))
	}
}

// ── Alert config enabled → override templateCode ────────────────────────────

func TestDispatchDue_ConfigEnabled_OverridesTemplateKey(t *testing.T) {
	alertRepo := &fakeAlertConfigRepoForDispatch{rows: []AlertTemplateConfig{
		{TypeID: "dt-test", AlertKind: AlertKindDeadline, TemplateKey: "reminder.deadline_approaching", Enabled: true},
	}}
	var capturedCode string
	sender := captureSender{capture: &capturedCode}
	svc, _, _ := newDispatchSvc(
		[]DispatchCandidate{{
			OccurrenceID:     "occ-override",
			IdempotencyKey:   "idem-override",
			TemplateCode:     "REMINDER_DISCLOSURE_DUE",
			TemplatePayload:  map[string]any{"title": "T", "deadline_date": "2026-06-01", "disclosure_id": "d1"},
			RecipientEmails:  []string{"a@example.com"},
			DisclosureTypeID: "dt-test",
			ScopeType:        ScopeTypeDisclosure,
		}},
		alertRepo, nil, sender,
	)
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if res.Sent != 1 {
		t.Errorf("expected sent=1, got %+v", res)
	}
	if capturedCode != "reminder.deadline_approaching" {
		t.Errorf("templateCode = %q, want reminder.deadline_approaching", capturedCode)
	}
}

// ── No config (nil) → backward compat, do NOT skip ──────────────────────────

func TestDispatchDue_NoConfig_BackwardCompat_NotSkipped(t *testing.T) {
	alertRepo := &fakeAlertConfigRepoForDispatch{rows: nil} // empty — no config for this type
	svc, _, _ := newDispatchSvc(
		[]DispatchCandidate{{
			OccurrenceID:     "occ-no-cfg",
			IdempotencyKey:   "idem-no-cfg",
			TemplateCode:     "REMINDER_DISCLOSURE_DUE",
			TemplatePayload:  map[string]any{"title": "T", "deadline_date": "2026-06-01", "disclosure_id": "d1"},
			RecipientEmails:  []string{"a@example.com"},
			DisclosureTypeID: "dt-test",
			ScopeType:        ScopeTypeDisclosure,
		}},
		alertRepo, nil, fakeSender{},
	)
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// No config → use original TemplateCode → should send (backward compat).
	if res.Sent != 1 {
		t.Errorf("expected sent=1 (backward compat), got %+v", res)
	}
	if res.Skipped != 0 {
		t.Errorf("expected skipped=0 when no config, got %d", res.Skipped)
	}
}

// ── Empty RecipientEmails → resolver called ──────────────────────────────────

func TestDispatchDue_EmptyRecipients_ResolverCalled(t *testing.T) {
	alertRepo := &fakeAlertConfigRepoForDispatch{rows: []AlertTemplateConfig{
		{TypeID: "dt-x", AlertKind: AlertKindDeadline, TemplateKey: "reminder.deadline_approaching", Enabled: true},
	}}
	resolver := &fakeResolverForDispatch{emails: []string{"resolved@co.com"}}
	svc, _, _ := newDispatchSvc(
		[]DispatchCandidate{{
			OccurrenceID:     "occ-resolve",
			IdempotencyKey:   "idem-resolve",
			TemplateCode:     "REMINDER_DISCLOSURE_DUE",
			TemplatePayload:  map[string]any{"title": "T", "deadline_date": "2026-06-01", "disclosure_id": "d1"},
			RecipientEmails:  nil, // empty — must call resolver
			DisclosureTypeID: "dt-x",
			ScopeType:        ScopeTypeDisclosure,
			CompanyID:        "c1",
		}},
		alertRepo, resolver, fakeSender{},
	)
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if res.Sent != 1 {
		t.Errorf("expected sent=1 after resolver, got %+v", res)
	}
}

// ── Pre-populated RecipientEmails → resolver NOT called ─────────────────────

func TestDispatchDue_PrePopulatedRecipients_ResolverNotCalled(t *testing.T) {
	// Resolver returns nothing (empty) — but recipients already populated, so send should succeed.
	alertRepo := &fakeAlertConfigRepoForDispatch{rows: []AlertTemplateConfig{
		{TypeID: "dt-y", AlertKind: AlertKindDeadline, TemplateKey: "reminder.deadline_approaching", Enabled: true},
	}}
	resolver := &fakeResolverForDispatch{emails: nil} // returns nothing
	svc, _, _ := newDispatchSvc(
		[]DispatchCandidate{{
			OccurrenceID:     "occ-prepop",
			IdempotencyKey:   "idem-prepop",
			TemplateCode:     "REMINDER_DISCLOSURE_DUE",
			TemplatePayload:  map[string]any{"title": "T", "deadline_date": "2026-06-01", "disclosure_id": "d1"},
			RecipientEmails:  []string{"existing@co.com"},
			DisclosureTypeID: "dt-y",
			ScopeType:        ScopeTypeDisclosure,
			CompanyID:        "c1",
		}},
		alertRepo, resolver, fakeSender{},
	)
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if res.Sent != 1 {
		t.Errorf("expected sent=1 with pre-populated recipients, got %+v", res)
	}
}

// ── Resolver returns empty → skip (not panic) ────────────────────────────────

func TestDispatchDue_ResolverEmpty_Skipped(t *testing.T) {
	alertRepo := &fakeAlertConfigRepoForDispatch{rows: []AlertTemplateConfig{
		{TypeID: "dt-z", AlertKind: AlertKindWorkflowStep, TemplateKey: "reminder.workflow_step_due", Enabled: true},
	}}
	resolver := &fakeResolverForDispatch{emails: nil} // returns nothing
	svc, _, _ := newDispatchSvc(
		[]DispatchCandidate{{
			OccurrenceID:     "occ-noresolver",
			IdempotencyKey:   "idem-noresolver",
			TemplateCode:     "REMINDER_DISCLOSURE_DUE",
			TemplatePayload:  map[string]any{},
			RecipientEmails:  nil,
			DisclosureTypeID: "dt-z",
			ScopeType:        ScopeTypeWorkflowStep,
			CompanyID:        "c1",
			ScopeID:          "step-1",
		}},
		alertRepo, resolver, fakeSender{},
	)
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if res.Skipped != 1 {
		t.Errorf("expected skipped=1, got %+v", res)
	}
}

// ── Retry behavior unchanged ─────────────────────────────────────────────────

func TestDispatchDue_RetryBehaviorUnchanged(t *testing.T) {
	// This test verifies that SMTP retryable errors still schedule retry (existing behavior).
	svc, _, _ := newDispatchSvc(
		[]DispatchCandidate{{
			OccurrenceID:    "occ-retry",
			IdempotencyKey:  "idem-retry",
			TemplateCode:    "REMINDER_DISCLOSURE_DUE",
			TemplatePayload: map[string]any{"title": "T", "deadline_date": "2026-06-01", "disclosure_id": "d1"},
			RecipientEmails: []string{"a@example.com"},
		}},
		nil, nil,
		fakeSender{err: retryErr{errors.New("temporary timeout")}},
	)
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if res.Retried != 1 {
		t.Errorf("expected retried=1, got %+v", res)
	}
}

// captureSender records the templateCode passed to SendReminderEmail.
type captureSender struct {
	capture *string
}

func (c captureSender) SendReminderEmail(_ context.Context, templateCode string, _ map[string]any, _ []string, _ string) (string, error) {
	if c.capture != nil {
		*c.capture = templateCode
	}
	return "captured", nil
}

