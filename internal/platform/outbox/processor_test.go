package outbox_test

import (
	"context"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/platform/outbox"
	outboxinmem "github.com/cobo/cobo_iam_services/internal/platform/outbox/inmemory"
)

func TestProcessor_unknownEventType_markedProcessed(t *testing.T) {
	ctx := context.Background()
	repo := outboxinmem.NewRepository()
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	_ = repo.Insert(ctx, outbox.InsertParams{
		EventID:       "e-unknown",
		AggregateType: "system",
		AggregateID:   "a1",
		EventType:     "no.handler.registered",
		PayloadJSON:   []byte(`{}`),
		AvailableAt:   now,
	})

	p := outbox.NewProcessor(repo, 10)
	p.Tick(ctx)

	// Second tick should not return same event (processed).
	got, err := repo.LockPendingBatch(ctx, 10, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range got {
		if e.EventID == "e-unknown" {
			t.Fatalf("expected unknown event consumed, still pending: %+v", e)
		}
	}
}

func TestProcessor_handlerSuccess_thenNotRedelivered(t *testing.T) {
	ctx := context.Background()
	repo := outboxinmem.NewRepository()
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	_ = repo.Insert(ctx, outbox.InsertParams{
		EventID:       "e-ok",
		AggregateType: "system",
		AggregateID:   "a1",
		EventType:     "notification.dispatch",
		PayloadJSON:   []byte(`{"x":1}`),
		AvailableAt:   now,
	})

	var saw bool
	p := outbox.NewProcessor(repo, 10)
	p.Register("notification.dispatch", outbox.HandlerFunc(func(ctx context.Context, event outbox.QueuedEvent) error {
		saw = true
		if event.EventID != "e-ok" {
			t.Fatalf("event id=%q", event.EventID)
		}
		return nil
	}))
	if err := p.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if !saw {
		t.Fatal("handler not invoked")
	}

	got, err := repo.LockPendingBatch(ctx, 10, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range got {
		if e.EventID == "e-ok" {
			t.Fatal("event should be processed")
		}
	}
}
