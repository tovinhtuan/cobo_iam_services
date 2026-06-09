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

// payloadCaptureSender records the full template payload for Fix 3 assertions.
type payloadCaptureSender struct {
	payload map[string]any
}

func (p *payloadCaptureSender) SendReminderEmail(_ context.Context, _ string, payload map[string]any, _ []string, _ string) (string, error) {
	cp := make(map[string]any, len(payload))
	for k, v := range payload {
		cp[k] = v
	}
	p.payload = cp
	return "captured-payload", nil
}

// ── Fix 3: portal_url deep-link built from scope when action_url absent ─────────

func newDispatchSvcWithURL(
	baseURL string,
	candidates []DispatchCandidate,
	sender EmailSender,
) Service {
	occRepo := &fakeOccurrenceRepo{candidates: candidates}
	if len(candidates) > 0 {
		occRepo.occ = ReminderOccurrenceDTO{
			OccurrenceID:   candidates[0].OccurrenceID,
			IdempotencyKey: candidates[0].IdempotencyKey,
			Status:         ReminderStatusPending,
		}
	}
	opts := []ServiceOption{
		WithPublicWebBaseURL(baseURL),
	}
	if sender != nil {
		opts = append(opts, WithEmailSender(sender))
	}
	return NewService(fakeConfigRepo{}, occRepo, &fakeAttemptRepo{}, opts...)
}

// TestFix3_DisclosureScope_PortalURLBuiltFromScopeID asserts that when no
// action_url is stored in the occurrence payload, prepareDispatch constructs an
// absolute portal_url from the DISCLOSURE scope's ScopeID (= disclosure_id).
func TestFix3_DisclosureScope_PortalURLBuiltFromScopeID(t *testing.T) {
	cs := &payloadCaptureSender{}
	svc := newDispatchSvcWithURL(
		"https://portal.cobo.vn",
		[]DispatchCandidate{{
			OccurrenceID:    "occ-f3a",
			IdempotencyKey:  "idem-f3a",
			TemplateCode:    "reminder.deadline_approaching",
			TemplatePayload: map[string]any{
				"disclosure_title": "Báo cáo tài chính Q1",
				"due_date":         "30/06/2026",
				"company_name":     "ACME Corp",
				// no action_url, no portal_url in payload
			},
			RecipientEmails: []string{"a@example.com"},
			ScopeType:       ScopeTypeDisclosure,
			ScopeID:         "disc-abc123", // this IS the disclosure_id for DISCLOSURE scope
		}},
		cs,
	)

	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if res.Sent != 1 {
		t.Fatalf("expected sent=1, got %+v", res)
	}

	portalURL, _ := cs.payload["portal_url"].(string)
	if portalURL == "" {
		t.Fatalf("portal_url is empty — Fix 3 requires a deep-link to be built from scope when action_url is absent")
	}
	if portalURL == "https://portal.cobo.vn" {
		t.Fatalf("portal_url = %q is the bare base URL — Fix 3 requires a deep-link path (e.g. /app/disclosures/disc-abc123)", portalURL)
	}
	if !contains(portalURL, "disc-abc123") {
		t.Fatalf("portal_url = %q does not contain the disclosure ID — Fix 3 requires the scope ID in the deep-link", portalURL)
	}
}

// TestFix3_WorkflowStepScope_PortalURLBuiltFromDisclosureIDInPayload asserts
// that for WORKFLOW_STEP scope, portal_url uses the disclosure_id stored in
// the payload (not the step_id which is a meaningless deep-link target).
func TestFix3_WorkflowStepScope_PortalURLBuiltFromDisclosureIDInPayload(t *testing.T) {
	cs := &payloadCaptureSender{}
	svc := newDispatchSvcWithURL(
		"https://portal.cobo.vn",
		[]DispatchCandidate{{
			OccurrenceID:    "occ-f3b",
			IdempotencyKey:  "idem-f3b",
			TemplateCode:    "reminder.workflow_step_due",
			TemplatePayload: map[string]any{
				"disclosure_id":    "disc-xyz789",
				"disclosure_title": "Báo cáo thường niên",
				"step_name":        "Phê duyệt nội bộ",
				"due_date":         "15/07/2026",
				"company_name":     "Corp B",
				// no action_url, no portal_url in payload
			},
			RecipientEmails: []string{"b@example.com"},
			ScopeType:       ScopeTypeWorkflowStep,
			ScopeID:         "step-wf-001", // step_id only; disclosure_id is in payload
		}},
		cs,
	)

	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if res.Sent != 1 {
		t.Fatalf("expected sent=1, got %+v", res)
	}

	portalURL, _ := cs.payload["portal_url"].(string)
	if portalURL == "" {
		t.Fatalf("portal_url is empty — Fix 3 requires deep-link for workflow_step scope")
	}
	if !contains(portalURL, "disc-xyz789") {
		t.Fatalf("portal_url = %q does not contain the disclosure ID %q — Fix 3 must use disclosure_id, not step_id", portalURL, "disc-xyz789")
	}
	if contains(portalURL, "step-wf-001") {
		t.Fatalf("portal_url = %q contains the step_id — deep-link must target the disclosure, not the step", portalURL)
	}
}

// TestFix3_ActionURLIsAbsoluteWhenBaseURLSet asserts that when action_url IS
// present in the payload, portal_url is built as the absolute URL (base + relative).
func TestFix3_ActionURLIsAbsoluteWhenBaseURLSet(t *testing.T) {
	cs := &payloadCaptureSender{}
	svc := newDispatchSvcWithURL(
		"https://portal.cobo.vn",
		[]DispatchCandidate{{
			OccurrenceID:    "occ-f3c",
			IdempotencyKey:  "idem-f3c",
			TemplateCode:    "reminder.deadline_approaching",
			TemplatePayload: map[string]any{
				"disclosure_title": "Annual report",
				"due_date":         "31/12/2026",
				"company_name":     "Corp C",
				"action_url":       "/app/disclosures/disc-stored",
			},
			RecipientEmails: []string{"c@example.com"},
			ScopeType:       ScopeTypeDisclosure,
			ScopeID:         "disc-stored",
		}},
		cs,
	)

	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if res.Sent != 1 {
		t.Fatalf("expected sent=1, got %+v", res)
	}

	portalURL, _ := cs.payload["portal_url"].(string)
	const want = "https://portal.cobo.vn/app/disclosures/disc-stored"
	if portalURL != want {
		t.Fatalf("portal_url = %q, want %q — Fix 3 requires action_url to be made absolute", portalURL, want)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i+len(substr) <= len(s); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

