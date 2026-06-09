package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	"github.com/cobo/cobo_iam_services/internal/notification/infra/inmemory"
	notificationregistry "github.com/cobo/cobo_iam_services/internal/notification/infra/registry"
	platformoutbox "github.com/cobo/cobo_iam_services/internal/platform/outbox"
	outboxinmem "github.com/cobo/cobo_iam_services/internal/platform/outbox/inmemory"
)

// recorderMetrics captures DeliveryMetrics calls for assertions.
type recorderMetrics struct {
	calls []string // outcome values, in order
}

func (m *recorderMetrics) RecordDelivery(outcome, _ string) { m.calls = append(m.calls, outcome) }

var verifyVars = map[string]any{
	"otp_code": "424242", "expiry_minutes": 15,
	"support_email": "support@cobo.vn", "website_url": "https://app.example.com",
}

// TestHandler_TransientReturnsRetryScheduler proves the handler's transient error
// implements outbox.RetryScheduler and pins RetryAt() to the application backoff
// (attempt 1 -> +1m), which is what makes the processor honor the schedule.
func TestHandler_TransientReturnsRetryScheduler(t *testing.T) {
	f := newHandlerFixture(t, 0)
	f.adapter.results = []fakeAdapterResult{{err: errors.New("451 try again later")}}

	err := f.handler.Handle(context.Background(), mustPayload(t, "n-handler", verifyVars))
	if err == nil {
		t.Fatalf("expected transient error")
	}
	var rs platformoutbox.RetryScheduler
	if !errors.As(err, &rs) {
		t.Fatalf("transient error does not implement outbox.RetryScheduler: %T", err)
	}
	// fixture clock is 2026-05-22T12:00:00Z; first backoff is 1m.
	want := time.Date(2026, 5, 22, 12, 1, 0, 0, time.UTC)
	if got := rs.RetryAt(); !got.Equal(want) {
		t.Fatalf("RetryAt() = %s, want %s (now + EmailRetryBackoff[0])", got, want)
	}
}

// TestHandler_DeliveryMetricsOutcomes asserts the metric seam fires the right
// outcome for sent / retry_scheduled / failed_permanent.
func TestHandler_DeliveryMetricsOutcomes(t *testing.T) {
	cases := []struct {
		name   string
		result fakeAdapterResult
		want   string
	}{
		{"sent", fakeAdapterResult{}, notificationapp.DeliveryOutcomeSent},
		{"transient", fakeAdapterResult{err: errors.New("421 service not available")}, notificationapp.DeliveryOutcomeRetryScheduled},
		{"permanent", fakeAdapterResult{err: errors.New("550 mailbox unavailable")}, notificationapp.DeliveryOutcomeFailedPermanent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newHandlerFixture(t, 0)
			rec := &recorderMetrics{}
			f.handler.WithMetrics(rec)
			if tc.result.err != nil {
				f.adapter.results = []fakeAdapterResult{tc.result}
			}
			_ = f.handler.Handle(context.Background(), mustPayload(t, "n-handler", verifyVars))
			if len(rec.calls) != 1 || rec.calls[0] != tc.want {
				t.Fatalf("metrics calls = %v, want [%s]", rec.calls, tc.want)
			}
		})
	}
}

// TestProcessorHandler_RetryHonorsApplicationSchedule is the integration proof:
// processor + real EmailDispatchHandler over an in-memory outbox. A transient
// failure must NOT be retried within seconds (the old defect); it becomes
// eligible only after the 1-minute application backoff, then succeeds.
func TestProcessorHandler_RetryHonorsApplicationSchedule(t *testing.T) {
	ctx := context.Background()
	base := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	clk := base
	clock := func() time.Time { return clk }

	notifRepo := inmemory.NewEmailNotificationRepository()
	attemptRepo := inmemory.NewEmailDeliveryAttemptRepository()
	adapter := &fakeAdapter{results: []fakeAdapterResult{{err: errors.New("421 service not available")}}}

	handler := notificationapp.NewEmailDispatchHandler(
		notifRepo, attemptRepo,
		notificationregistry.NewEmbedRegistry(),
		notificationapp.NewEmailRenderer(),
		adapter, &seqIDGen{}, clock, 0,
	)

	notif := &notificationapp.EmailNotification{
		EmailNotificationID:    "n-int",
		RecipientEmail:         "user@example.com",
		TemplateKey:            "auth.email_verification",
		Locale:                 "vi",
		Status:                 notificationapp.EmailStatusPending,
		IdempotencyKey:         "idem-int",
		VariablesJSONSanitized: `{}`,
		CreatedAt:              base,
		UpdatedAt:              base,
	}
	if err := notifRepo.InsertNotification(ctx, notif); err != nil {
		t.Fatalf("seed notif: %v", err)
	}

	outboxRepo := outboxinmem.NewRepository()
	processor := platformoutbox.NewProcessor(outboxRepo, 10).WithClock(clock)
	processor.Register(notificationapp.EmailDispatchOutboxEventType, platformoutbox.HandlerFunc(func(ctx context.Context, e platformoutbox.QueuedEvent) error {
		return handler.Handle(ctx, e.PayloadJSON)
	}))

	payload, _ := json.Marshal(notificationapp.EmailDispatchOutboxPayload{NotificationID: "n-int", Variables: verifyVars})
	if err := outboxRepo.Insert(ctx, platformoutbox.InsertParams{
		EventID: "evt-int", AggregateType: "email", AggregateID: "n-int",
		EventType: notificationapp.EmailDispatchOutboxEventType, PayloadJSON: payload, AvailableAt: base,
	}); err != nil {
		t.Fatalf("insert outbox: %v", err)
	}

	// Tick 1 @ base: send fails transiently -> retry scheduled at base+1m.
	if err := processor.Tick(ctx); err != nil {
		t.Fatalf("tick1: %v", err)
	}
	if len(adapter.calls) != 1 {
		t.Fatalf("after tick1 adapter calls = %d, want 1", len(adapter.calls))
	}
	if got, _ := notifRepo.GetByID(ctx, "n-int"); got.Status != notificationapp.EmailStatusRetry {
		t.Fatalf("after tick1 status = %q, want retry", got.Status)
	}

	// Tick 2 @ base+30s: event must still be ineligible (the old fast backoff
	// would have re-sent within ~1-2s). Adapter MUST NOT be called again.
	clk = base.Add(30 * time.Second)
	if err := processor.Tick(ctx); err != nil {
		t.Fatalf("tick2: %v", err)
	}
	if len(adapter.calls) != 1 {
		t.Fatalf("after tick2(+30s) adapter calls = %d, want 1 (retried too early)", len(adapter.calls))
	}

	// Tick 3 @ base+61s: now eligible; second attempt succeeds.
	clk = base.Add(61 * time.Second)
	if err := processor.Tick(ctx); err != nil {
		t.Fatalf("tick3: %v", err)
	}
	if len(adapter.calls) != 2 {
		t.Fatalf("after tick3(+61s) adapter calls = %d, want 2", len(adapter.calls))
	}
	if got, _ := notifRepo.GetByID(ctx, "n-int"); got.Status != notificationapp.EmailStatusSent {
		t.Fatalf("after tick3 status = %q, want sent", got.Status)
	}
}
