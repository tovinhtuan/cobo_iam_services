package inmemory

import (
	"context"
	"sync"
	"time"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
)

// EmailNotificationRepository is the in-memory implementation of
// notificationapp.EmailNotificationRepository. It is used by tests and by the
// no-MySQL local bootstrap mode; the MySQL-backed variant in infra/mysql is the
// production path. Both must enforce the same idempotency contract.
type EmailNotificationRepository struct {
	mu      sync.RWMutex
	byID    map[string]*notificationapp.EmailNotification
	byIdem  map[string]string // idempotency_key -> notification_id
}

func NewEmailNotificationRepository() *EmailNotificationRepository {
	return &EmailNotificationRepository{
		byID:   map[string]*notificationapp.EmailNotification{},
		byIdem: map[string]string{},
	}
}

func (r *EmailNotificationRepository) InsertNotification(_ context.Context, n *notificationapp.EmailNotification) error {
	if n == nil {
		return notificationapp.ErrRepositoryPersist
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byIdem[n.IdempotencyKey]; exists {
		return notificationapp.ErrAlreadyDispatched
	}
	if _, exists := r.byID[n.EmailNotificationID]; exists {
		return notificationapp.ErrRepositoryPersist
	}
	cp := *n
	r.byID[n.EmailNotificationID] = &cp
	r.byIdem[n.IdempotencyKey] = n.EmailNotificationID
	return nil
}

func (r *EmailNotificationRepository) FindByIdempotencyKey(_ context.Context, idempotencyKey string) (*notificationapp.EmailNotification, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byIdem[idempotencyKey]
	if !ok {
		return nil, nil
	}
	row, ok := r.byID[id]
	if !ok {
		return nil, nil
	}
	cp := *row
	return &cp, nil
}

func (r *EmailNotificationRepository) GetByID(_ context.Context, notificationID string) (*notificationapp.EmailNotification, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	row, ok := r.byID[notificationID]
	if !ok {
		return nil, nil
	}
	cp := *row
	return &cp, nil
}

func (r *EmailNotificationRepository) MarkSending(_ context.Context, notificationID string, attemptedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.byID[notificationID]
	if !ok {
		return notificationapp.ErrRepositoryPersist
	}
	row.Status = notificationapp.EmailStatusSending
	row.UpdatedAt = attemptedAt
	return nil
}

func (r *EmailNotificationRepository) MarkSent(_ context.Context, notificationID string, sentAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.byID[notificationID]
	if !ok {
		return notificationapp.ErrRepositoryPersist
	}
	row.Status = notificationapp.EmailStatusSent
	row.SentAt = &sentAt
	row.UpdatedAt = sentAt
	return nil
}

func (r *EmailNotificationRepository) MarkRetry(_ context.Context, notificationID, errorCode, errorMessageRedacted string, nextRetryAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.byID[notificationID]
	if !ok {
		return notificationapp.ErrRepositoryPersist
	}
	row.Status = notificationapp.EmailStatusRetry
	row.LastErrorCode = errorCode
	row.LastErrorMessageRedacted = errorMessageRedacted
	row.UpdatedAt = nextRetryAt
	return nil
}

func (r *EmailNotificationRepository) MarkFailedPermanent(_ context.Context, notificationID, errorCode, errorMessageRedacted string, failedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.byID[notificationID]
	if !ok {
		return notificationapp.ErrRepositoryPersist
	}
	row.Status = notificationapp.EmailStatusFailedPermanent
	row.LastErrorCode = errorCode
	row.LastErrorMessageRedacted = errorMessageRedacted
	row.UpdatedAt = failedAt
	return nil
}

// Snapshot returns a copy of every stored row, sorted by created_at. Test-only
// helper for assertions that don't need to know about internal map order.
func (r *EmailNotificationRepository) Snapshot() []notificationapp.EmailNotification {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]notificationapp.EmailNotification, 0, len(r.byID))
	for _, row := range r.byID {
		out = append(out, *row)
	}
	return out
}
