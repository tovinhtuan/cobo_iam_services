package inmemory

import (
	"context"
	"testing"
	"time"

	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
)

func occ(id string, status reminderapp.ReminderStatus) reminderapp.ReminderOccurrenceDTO {
	return reminderapp.ReminderOccurrenceDTO{
		OccurrenceID:   id,
		IdempotencyKey: id,
		ScopeType:      reminderapp.ScopeTypeDisclosure,
		ScopeID:        id,
		DisclosureID:   id,
		ScheduledAt:    time.Now().UTC(),
		Status:         status,
	}
}

func TestRequeueStaleDispatching_RequeuesOnlyStale(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()

	_, _ = r.SeedOccurrence(ctx, occ("stale", reminderapp.ReminderStatusDispatching))
	_, _ = r.SeedOccurrence(ctx, occ("fresh", reminderapp.ReminderStatusDispatching))
	_, _ = r.SeedOccurrence(ctx, occ("sent", reminderapp.ReminderStatusSent))
	_, _ = r.SeedOccurrence(ctx, occ("failed", reminderapp.ReminderStatusFailed))

	now := time.Now().UTC()
	// stale was last touched 1h ago; fresh just now.
	r.updatedAt["stale"] = now.Add(-1 * time.Hour)
	r.updatedAt["fresh"] = now
	r.updatedAt["sent"] = now.Add(-2 * time.Hour)
	r.updatedAt["failed"] = now.Add(-2 * time.Hour)

	cutoff := now.Add(-5 * time.Minute)
	n, err := r.RequeueStaleDispatching(ctx, cutoff)
	if err != nil {
		t.Fatalf("RequeueStaleDispatching: %v", err)
	}
	if n != 1 {
		t.Fatalf("requeued count = %d, want 1", n)
	}

	if got := r.occurrences["stale"].Status; got != reminderapp.ReminderStatusPending {
		t.Fatalf("stale status = %s, want PENDING", got)
	}
	if got := r.occurrences["fresh"].Status; got != reminderapp.ReminderStatusDispatching {
		t.Fatalf("fresh status = %s, want DISPATCHING (not stale)", got)
	}
	if got := r.occurrences["sent"].Status; got != reminderapp.ReminderStatusSent {
		t.Fatalf("sent status = %s, want SENT (untouched)", got)
	}
	if got := r.occurrences["failed"].Status; got != reminderapp.ReminderStatusFailed {
		t.Fatalf("failed status = %s, want FAILED (untouched)", got)
	}
}

func TestRequeueStaleDispatching_NoneStale(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	_, _ = r.SeedOccurrence(ctx, occ("d1", reminderapp.ReminderStatusDispatching))
	r.updatedAt["d1"] = time.Now().UTC()

	n, err := r.RequeueStaleDispatching(ctx, time.Now().UTC().Add(-5*time.Minute))
	if err != nil {
		t.Fatalf("RequeueStaleDispatching: %v", err)
	}
	if n != 0 {
		t.Fatalf("requeued count = %d, want 0", n)
	}
}
