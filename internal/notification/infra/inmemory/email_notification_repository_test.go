package inmemory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
	"github.com/cobo/cobo_iam_services/internal/notification/infra/inmemory"
)

func TestEmailNotificationRepository_InsertAndLookup(t *testing.T) {
	repo := inmemory.NewEmailNotificationRepository()
	ctx := context.Background()
	n := &notificationapp.EmailNotification{
		EmailNotificationID: "n-1",
		RecipientEmail:      "a@example.com",
		TemplateKey:         "auth.email_verification",
		Locale:              "vi",
		Status:              notificationapp.EmailStatusPending,
		IdempotencyKey:      "idem-1",
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}
	if err := repo.InsertNotification(ctx, n); err != nil {
		t.Fatalf("InsertNotification error = %v", err)
	}
	got, err := repo.GetByID(ctx, "n-1")
	if err != nil {
		t.Fatalf("GetByID error = %v", err)
	}
	if got == nil || got.EmailNotificationID != "n-1" {
		t.Fatalf("GetByID got = %+v", got)
	}
	found, err := repo.FindByIdempotencyKey(ctx, "idem-1")
	if err != nil {
		t.Fatalf("FindByIdempotencyKey error = %v", err)
	}
	if found == nil || found.EmailNotificationID != "n-1" {
		t.Fatalf("FindByIdempotencyKey got = %+v", found)
	}
}

func TestEmailNotificationRepository_DuplicateIdempotencyKeyRejected(t *testing.T) {
	repo := inmemory.NewEmailNotificationRepository()
	ctx := context.Background()
	first := &notificationapp.EmailNotification{
		EmailNotificationID: "n-1",
		IdempotencyKey:      "idem-shared",
	}
	dup := &notificationapp.EmailNotification{
		EmailNotificationID: "n-2",
		IdempotencyKey:      "idem-shared",
	}
	if err := repo.InsertNotification(ctx, first); err != nil {
		t.Fatalf("first InsertNotification error = %v", err)
	}
	err := repo.InsertNotification(ctx, dup)
	if !errors.Is(err, notificationapp.ErrAlreadyDispatched) {
		t.Fatalf("expected ErrAlreadyDispatched, got %v", err)
	}
	// First record must still be retrievable; duplicate must not exist.
	got, _ := repo.GetByID(ctx, "n-2")
	if got != nil {
		t.Fatalf("duplicate row was persisted: %+v", got)
	}
}

func TestEmailNotificationRepository_FindByIdempotencyKeyNotFound(t *testing.T) {
	repo := inmemory.NewEmailNotificationRepository()
	got, err := repo.FindByIdempotencyKey(context.Background(), "missing")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for missing key, got %+v", got)
	}
}

func TestEmailNotificationRepository_StatusTransitions(t *testing.T) {
	repo := inmemory.NewEmailNotificationRepository()
	ctx := context.Background()
	now := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)
	n := &notificationapp.EmailNotification{
		EmailNotificationID: "n-status",
		IdempotencyKey:      "idem-status",
		Status:              notificationapp.EmailStatusPending,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := repo.InsertNotification(ctx, n); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	if err := repo.MarkSending(ctx, "n-status", now.Add(time.Minute)); err != nil {
		t.Fatalf("MarkSending: %v", err)
	}
	if got, _ := repo.GetByID(ctx, "n-status"); got.Status != notificationapp.EmailStatusSending {
		t.Fatalf("status after MarkSending = %q", got.Status)
	}

	if err := repo.MarkRetry(ctx, "n-status", "transient_smtp", "redacted", now.Add(2*time.Minute)); err != nil {
		t.Fatalf("MarkRetry: %v", err)
	}
	got, _ := repo.GetByID(ctx, "n-status")
	if got.Status != notificationapp.EmailStatusRetry {
		t.Fatalf("status after MarkRetry = %q", got.Status)
	}
	if got.LastErrorCode != "transient_smtp" || got.LastErrorMessageRedacted != "redacted" {
		t.Fatalf("retry error metadata not set: %+v", got)
	}

	if err := repo.MarkSent(ctx, "n-status", now.Add(3*time.Minute)); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}
	got, _ = repo.GetByID(ctx, "n-status")
	if got.Status != notificationapp.EmailStatusSent || got.SentAt == nil {
		t.Fatalf("status after MarkSent = %q sent_at = %v", got.Status, got.SentAt)
	}

	if err := repo.MarkFailedPermanent(ctx, "n-status", "perm_550", "user not found", now.Add(4*time.Minute)); err != nil {
		t.Fatalf("MarkFailedPermanent: %v", err)
	}
	got, _ = repo.GetByID(ctx, "n-status")
	if got.Status != notificationapp.EmailStatusFailedPermanent {
		t.Fatalf("status after MarkFailedPermanent = %q", got.Status)
	}
}

func TestEmailNotificationRepository_MarkOnMissingID(t *testing.T) {
	repo := inmemory.NewEmailNotificationRepository()
	ctx := context.Background()
	now := time.Now()
	for _, call := range []func() error{
		func() error { return repo.MarkSending(ctx, "ghost", now) },
		func() error { return repo.MarkSent(ctx, "ghost", now) },
		func() error { return repo.MarkRetry(ctx, "ghost", "x", "y", now) },
		func() error { return repo.MarkFailedPermanent(ctx, "ghost", "x", "y", now) },
	} {
		if err := call(); err == nil {
			t.Fatalf("expected error for missing notification id")
		}
	}
}

func TestEmailNotificationRepository_SnapshotIsCopy(t *testing.T) {
	repo := inmemory.NewEmailNotificationRepository()
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_ = repo.InsertNotification(ctx, &notificationapp.EmailNotification{
			EmailNotificationID: idForIndex(i),
			IdempotencyKey:      "idem-" + idForIndex(i),
			Status:              notificationapp.EmailStatusPending,
		})
	}
	snap := repo.Snapshot()
	if len(snap) != 3 {
		t.Fatalf("snapshot len = %d", len(snap))
	}
	// Mutating the snapshot must not affect the repo.
	snap[0].Status = "should_not_propagate"
	got, _ := repo.GetByID(ctx, snap[0].EmailNotificationID)
	if got.Status != notificationapp.EmailStatusPending {
		t.Fatalf("snapshot mutation leaked into repo: %q", got.Status)
	}
}

func idForIndex(i int) string {
	switch i {
	case 0:
		return "alpha"
	case 1:
		return "bravo"
	default:
		return "charlie"
	}
}
