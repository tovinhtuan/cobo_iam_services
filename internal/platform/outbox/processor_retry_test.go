package outbox_test

import (
	"context"
	"testing"
	"time"

	platformoutbox "github.com/cobo/cobo_iam_services/internal/platform/outbox"
	outboxinmem "github.com/cobo/cobo_iam_services/internal/platform/outbox/inmemory"
)

// retryAtErr is a handler error that implements outbox.RetryScheduler, pinning
// the next attempt time the way the email dispatch handler's retryAfterError does.
type retryAtErr struct {
	at time.Time
}

func (e retryAtErr) Error() string      { return "transient: scheduled retry" }
func (e retryAtErr) RetryAt() time.Time { return e.at }

func mustInsert(t *testing.T, repo *outboxinmem.Repository, id string, at time.Time) {
	t.Helper()
	if err := repo.Insert(context.Background(), platformoutbox.InsertParams{
		EventID:       id,
		AggregateType: "test",
		AggregateID:   id,
		EventType:     "test.event",
		PayloadJSON:   []byte(`{}`),
		AvailableAt:   at,
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
}

// eligibleAt reports whether the event would be locked at probe time WITHOUT
// mutating state on a miss. A hit DOES mutate (locks to processing) — callers
// must treat a true result as terminal for the probed row.
func eligibleAt(t *testing.T, repo *outboxinmem.Repository, probe time.Time) bool {
	t.Helper()
	got, err := repo.LockPendingBatch(context.Background(), 10, probe)
	if err != nil {
		t.Fatalf("lock: %v", err)
	}
	return len(got) > 0
}

func TestProcessor_HonorsRetryAt(t *testing.T) {
	repo := outboxinmem.NewRepository()
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	p := platformoutbox.NewProcessor(repo, 10).WithClock(func() time.Time { return now })

	p.Register("test.event", platformoutbox.HandlerFunc(func(_ context.Context, _ platformoutbox.QueuedEvent) error {
		return retryAtErr{at: now.Add(1 * time.Hour)}
	}))

	mustInsert(t, repo, "evt-1", now)
	if err := p.Tick(context.Background()); err != nil {
		t.Fatalf("tick: %v", err)
	}

	// The application pinned available_at to now+1h. The processor's default
	// exponential backoff (~1-2s) must NOT win: the row stays ineligible well
	// past the seconds-scale window that caused the Batch 2B silent-drop defect.
	if eligibleAt(t, repo, now.Add(59*time.Minute)) {
		t.Fatalf("event eligible at now+59m — RetryAt() not honored (regressed to fast backoff)")
	}
	if !eligibleAt(t, repo, now.Add(1*time.Hour+time.Second)) {
		t.Fatalf("event NOT eligible at now+1h — available_at not pinned to RetryAt()")
	}
}

func TestProcessor_FallsBackToExponentialBackoffWithoutRetryScheduler(t *testing.T) {
	repo := outboxinmem.NewRepository()
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	p := platformoutbox.NewProcessor(repo, 10).WithClock(func() time.Time { return now })

	p.Register("test.event", platformoutbox.HandlerFunc(func(_ context.Context, _ platformoutbox.QueuedEvent) error {
		return context.DeadlineExceeded // plain error, no RetryScheduler
	}))

	mustInsert(t, repo, "evt-1", now)
	if err := p.Tick(context.Background()); err != nil {
		t.Fatalf("tick: %v", err)
	}
	// Default backoff for the first retry is ~1s (+<=25% jitter): eligible within
	// a few seconds, definitely by now+1m.
	if !eligibleAt(t, repo, now.Add(time.Minute)) {
		t.Fatalf("plain-error event not eligible at now+1m — fallback backoff broken")
	}
}

func TestProcessor_RequeueStaleProcessing(t *testing.T) {
	repo := outboxinmem.NewRepository()
	t0 := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	p := platformoutbox.NewProcessor(repo, 10)

	mustInsert(t, repo, "evt-stuck", t0)
	// Lock it -> status processing, available_at stays at t0 (simulates a worker
	// that crashed mid-Handle).
	if got, _ := repo.LockPendingBatch(context.Background(), 10, t0); len(got) != 1 {
		t.Fatalf("expected to lock 1, got %d", len(got))
	}

	// Fresh lock (timeout not yet elapsed) must NOT requeue.
	if n, err := p.RequeueStaleProcessing(context.Background(), t0.Add(-time.Minute)); err != nil || n != 0 {
		t.Fatalf("premature requeue n=%d err=%v", n, err)
	}
	// After the visibility timeout, the orphaned row is requeued.
	n, err := p.RequeueStaleProcessing(context.Background(), t0.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("requeue: %v", err)
	}
	if n != 1 {
		t.Fatalf("requeued = %d, want 1", n)
	}
	// Requeued row is pending and immediately eligible again (retry budget NOT
	// consumed — a crash is not a delivery failure).
	if !eligibleAt(t, repo, t0) {
		t.Fatalf("requeued event not eligible — reaper did not reset status")
	}
}
