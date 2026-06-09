package app

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestService_RequeueStaleDispatching_ReturnsCountAndPassesCutoff(t *testing.T) {
	occRepo := &fakeOccurrenceRepo{requeued: 3}
	svc := NewService(fakeConfigRepo{}, occRepo, &fakeAttemptRepo{})

	cutoff := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	n, err := svc.RequeueStaleDispatching(context.Background(), cutoff)
	if err != nil {
		t.Fatalf("RequeueStaleDispatching: %v", err)
	}
	if n != 3 {
		t.Fatalf("count = %d, want 3", n)
	}
	if !occRepo.requeueCalled {
		t.Fatalf("repo RequeueStaleDispatching not called")
	}
	if !occRepo.requeueOlder.Equal(cutoff) {
		t.Fatalf("cutoff = %s, want %s", occRepo.requeueOlder, cutoff)
	}
}

func TestService_RequeueStaleDispatching_PropagatesError(t *testing.T) {
	occRepo := &fakeOccurrenceRepo{requeueErr: errors.New("db down")}
	svc := NewService(fakeConfigRepo{}, occRepo, &fakeAttemptRepo{})

	n, err := svc.RequeueStaleDispatching(context.Background(), time.Now().UTC())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if n != 0 {
		t.Fatalf("count on error = %d, want 0", n)
	}
}

type fakeConfigRepo struct{}

func (fakeConfigRepo) UpsertByScope(context.Context, ReminderConfigDTO) (*ReminderConfigDTO, error) {
	return &ReminderConfigDTO{}, nil
}
func (fakeConfigRepo) GetByScope(context.Context, ScopeType, string) (*ReminderConfigDTO, error) {
	return &ReminderConfigDTO{}, nil
}

type fakeOccurrenceRepo struct {
	occ          ReminderOccurrenceDTO
	candidates   []DispatchCandidate
	updates      []DispatchResultInput
	requeued      int
	requeueErr    error
	requeueOlder  time.Time
	requeueCalled bool
}

func (f *fakeOccurrenceRepo) ListHistoryByDisclosure(context.Context, string, HistoryQuery) (*ReminderHistoryPage, error) {
	return &ReminderHistoryPage{}, nil
}
func (f *fakeOccurrenceRepo) ClaimForDispatch(context.Context, string) (*ReminderOccurrenceDTO, error) {
	occ := f.occ
	occ.Status = ReminderStatusDispatching
	return &occ, nil
}
func (f *fakeOccurrenceRepo) UpdateDispatchResult(_ context.Context, in DispatchResultInput) error {
	f.updates = append(f.updates, in)
	return nil
}
func (f *fakeOccurrenceRepo) SeedOccurrence(_ context.Context, in ReminderOccurrenceDTO) (*ReminderOccurrenceDTO, error) {
	f.occ = in
	return &in, nil
}
func (f *fakeOccurrenceRepo) MaterializeDueOccurrences(context.Context, time.Time) (int, error) {
	return 0, nil
}
func (f *fakeOccurrenceRepo) ListDispatchCandidates(context.Context, time.Time, int) ([]DispatchCandidate, error) {
	return f.candidates, nil
}
func (f *fakeOccurrenceRepo) RequeueStaleDispatching(_ context.Context, olderThan time.Time) (int, error) {
	f.requeueCalled = true
	f.requeueOlder = olderThan
	if f.requeueErr != nil {
		return 0, f.requeueErr
	}
	return f.requeued, nil
}

type fakeAttemptRepo struct {
	attempts []ReminderDeliveryAttemptDTO
}

func (f *fakeAttemptRepo) InsertAttempt(_ context.Context, in ReminderDeliveryAttemptDTO) error {
	f.attempts = append(f.attempts, in)
	return nil
}

type fakeSender struct {
	err error
}

func (f fakeSender) SendReminderEmail(context.Context, string, map[string]any, []string, string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return "provider-msg-1", nil
}

type captureAlert struct {
	codes []string
}

func (c *captureAlert) Notify(_ context.Context, alertCode string, _ map[string]string, _ map[string]any) {
	c.codes = append(c.codes, alertCode)
}

func TestDispatchOccurrence_RetryableErrorSchedulesRetry(t *testing.T) {
	occRepo := &fakeOccurrenceRepo{
		occ: ReminderOccurrenceDTO{
			OccurrenceID:   "occ-1",
			IdempotencyKey: "idem-1",
			AttemptCount:   2,
			Status:         ReminderStatusPending,
		},
	}
	attemptRepo := &fakeAttemptRepo{}
	alerts := &captureAlert{}
	svc := NewService(fakeConfigRepo{}, occRepo, attemptRepo,
		WithEmailSender(fakeSender{err: retryErr{errors.New("temporary timeout")}}),
		WithAlertHook(alerts),
	)

	resp, err := svc.DispatchOccurrence(context.Background(), DispatchOccurrenceRequest{
		OccurrenceID:    "occ-1",
		IdempotencyKey:  "idem-1",
		TemplateCode:    "REMINDER_DISCLOSURE_DUE",
		TemplatePayload: map[string]any{},
		RecipientEmails: []string{"a@example.com"},
	})
	if err != nil {
		t.Fatalf("DispatchOccurrence error = %v", err)
	}
	if resp.Status != ReminderStatusRetryScheduled {
		t.Fatalf("expected RETRY_SCHEDULED, got %s", resp.Status)
	}
	if len(attemptRepo.attempts) != 1 || attemptRepo.attempts[0].Status != ReminderStatusRetryScheduled {
		t.Fatalf("expected one retry attempt, got %+v", attemptRepo.attempts)
	}
	if len(occRepo.updates) != 1 || occRepo.updates[0].Status != ReminderStatusRetryScheduled {
		t.Fatalf("expected retry update, got %+v", occRepo.updates)
	}
	if len(alerts.codes) == 0 || alerts.codes[0] != "REMINDER_RETRY_THRESHOLD" {
		t.Fatalf("expected REMINDER_RETRY_THRESHOLD alert, got %+v", alerts.codes)
	}
}

func TestDispatchOccurrence_PermanentErrorFails(t *testing.T) {
	occRepo := &fakeOccurrenceRepo{
		occ: ReminderOccurrenceDTO{
			OccurrenceID:   "occ-2",
			IdempotencyKey: "idem-2",
			AttemptCount:   0,
			Status:         ReminderStatusPending,
		},
	}
	attemptRepo := &fakeAttemptRepo{}
	svc := NewService(fakeConfigRepo{}, occRepo, attemptRepo,
		WithEmailSender(fakeSender{err: errors.New("550 invalid recipient")}),
	)

	resp, err := svc.DispatchOccurrence(context.Background(), DispatchOccurrenceRequest{
		OccurrenceID:    "occ-2",
		IdempotencyKey:  "idem-2",
		TemplateCode:    "REMINDER_DISCLOSURE_DUE",
		TemplatePayload: map[string]any{},
		RecipientEmails: []string{"a@example.com"},
	})
	if err != nil {
		t.Fatalf("DispatchOccurrence error = %v", err)
	}
	if resp.Status != ReminderStatusFailed {
		t.Fatalf("expected FAILED, got %s", resp.Status)
	}
}

func TestDispatchDueOccurrences_SLABreachAlert(t *testing.T) {
	now := time.Now().UTC()
	occRepo := &fakeOccurrenceRepo{
		occ: ReminderOccurrenceDTO{
			OccurrenceID:   "occ-3",
			IdempotencyKey: "idem-3",
			AttemptCount:   0,
			Status:         ReminderStatusPending,
		},
		candidates: []DispatchCandidate{
			{
				OccurrenceID:    "occ-3",
				IdempotencyKey:  "idem-3",
				TemplateCode:    "REMINDER_DISCLOSURE_DUE",
				TemplatePayload: map[string]any{},
				RecipientEmails: []string{"a@example.com"},
				ScheduledAt:     now.Add(-11 * time.Minute),
			},
		},
	}
	attemptRepo := &fakeAttemptRepo{}
	alerts := &captureAlert{}
	svc := NewService(fakeConfigRepo{}, occRepo, attemptRepo,
		WithEmailSender(fakeSender{}),
		WithAlertHook(alerts),
	)

	res, err := svc.DispatchDueOccurrences(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("DispatchDueOccurrences error = %v", err)
	}
	if res.Sent != 1 {
		t.Fatalf("expected sent=1, got %+v", res)
	}
	found := false
	for _, code := range alerts.codes {
		if code == "REMINDER_SLA_BREACH" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected REMINDER_SLA_BREACH alert, got %+v", alerts.codes)
	}
}

type retryErr struct{ error }

func (retryErr) Temporary() bool { return true }
