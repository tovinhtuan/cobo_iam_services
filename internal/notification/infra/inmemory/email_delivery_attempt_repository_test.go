package inmemory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	"github.com/cobo/cobo_iam_services/internal/notification/infra/inmemory"
)

func TestEmailDeliveryAttemptRepository_InsertAndCount(t *testing.T) {
	repo := inmemory.NewEmailDeliveryAttemptRepository()
	ctx := context.Background()
	for i := 1; i <= 3; i++ {
		err := repo.InsertAttempt(ctx, &notificationapp.DeliveryAttempt{
			DeliveryAttemptID: idForAttempt(i),
			NotificationID:    "n-1",
			AttemptNo:         i,
			Provider:          "smtp",
			Status:            notificationapp.AttemptStatusRetry,
			StartedAt:         time.Now(),
			FinishedAt:        time.Now(),
		})
		if err != nil {
			t.Fatalf("InsertAttempt(%d) error = %v", i, err)
		}
	}
	got, err := repo.CountByNotificationID(ctx, "n-1")
	if err != nil {
		t.Fatalf("Count error = %v", err)
	}
	if got != 3 {
		t.Fatalf("count = %d, want 3", got)
	}
	snap := repo.Snapshot("n-1")
	if len(snap) != 3 {
		t.Fatalf("snapshot len = %d", len(snap))
	}
	for i, a := range snap {
		if a.AttemptNo != i+1 {
			t.Fatalf("snapshot order broken at %d: %+v", i, a)
		}
	}
}

func TestEmailDeliveryAttemptRepository_DuplicateAttemptRejected(t *testing.T) {
	repo := inmemory.NewEmailDeliveryAttemptRepository()
	ctx := context.Background()
	now := time.Now()
	first := &notificationapp.DeliveryAttempt{
		DeliveryAttemptID: "att-1",
		NotificationID:    "n-1",
		AttemptNo:         1,
		StartedAt:         now,
		FinishedAt:        now,
	}
	if err := repo.InsertAttempt(ctx, first); err != nil {
		t.Fatalf("first Insert: %v", err)
	}
	dup := &notificationapp.DeliveryAttempt{
		DeliveryAttemptID: "att-2",
		NotificationID:    "n-1",
		AttemptNo:         1, // same attempt_no
		StartedAt:         now,
		FinishedAt:        now,
	}
	err := repo.InsertAttempt(ctx, dup)
	if !errors.Is(err, notificationapp.ErrRepositoryPersist) {
		t.Fatalf("duplicate accept: err = %v", err)
	}
	// Repo must still report exactly one attempt for n-1.
	got, _ := repo.CountByNotificationID(ctx, "n-1")
	if got != 1 {
		t.Fatalf("count after duplicate = %d, want 1", got)
	}
}

func TestEmailDeliveryAttemptRepository_NilInputRejected(t *testing.T) {
	repo := inmemory.NewEmailDeliveryAttemptRepository()
	if err := repo.InsertAttempt(context.Background(), nil); !errors.Is(err, notificationapp.ErrRepositoryPersist) {
		t.Fatalf("nil attempt: err = %v", err)
	}
}

func idForAttempt(i int) string {
	switch i {
	case 1:
		return "att-1"
	case 2:
		return "att-2"
	default:
		return "att-3"
	}
}
