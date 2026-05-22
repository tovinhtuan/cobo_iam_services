package inmemory

import (
	"context"
	"fmt"
	"sync"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
)

// EmailDeliveryAttemptRepository is the in-memory implementation used by tests
// and no-MySQL local mode. The unique (notification_id, attempt_no) invariant
// is enforced so test code mirrors production semantics.
type EmailDeliveryAttemptRepository struct {
	mu       sync.RWMutex
	byID     map[string]*notificationapp.DeliveryAttempt
	byNotif  map[string][]string // notification_id -> ordered attempt_ids
	keyIndex map[string]struct{} // notification_id|attempt_no
}

func NewEmailDeliveryAttemptRepository() *EmailDeliveryAttemptRepository {
	return &EmailDeliveryAttemptRepository{
		byID:     map[string]*notificationapp.DeliveryAttempt{},
		byNotif:  map[string][]string{},
		keyIndex: map[string]struct{}{},
	}
}

func (r *EmailDeliveryAttemptRepository) InsertAttempt(_ context.Context, a *notificationapp.DeliveryAttempt) error {
	if a == nil {
		return notificationapp.ErrRepositoryPersist
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := r.attemptKey(a.NotificationID, a.AttemptNo)
	if _, exists := r.keyIndex[key]; exists {
		return fmt.Errorf("%w: duplicate (notification_id, attempt_no)", notificationapp.ErrRepositoryPersist)
	}
	if _, exists := r.byID[a.DeliveryAttemptID]; exists {
		return notificationapp.ErrRepositoryPersist
	}
	cp := *a
	r.byID[a.DeliveryAttemptID] = &cp
	r.byNotif[a.NotificationID] = append(r.byNotif[a.NotificationID], a.DeliveryAttemptID)
	r.keyIndex[key] = struct{}{}
	return nil
}

func (r *EmailDeliveryAttemptRepository) CountByNotificationID(_ context.Context, notificationID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byNotif[notificationID]), nil
}

// Snapshot returns a copy of every attempt stored for notification_id, in
// insertion order. Test-only helper.
func (r *EmailDeliveryAttemptRepository) Snapshot(notificationID string) []notificationapp.DeliveryAttempt {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := r.byNotif[notificationID]
	out := make([]notificationapp.DeliveryAttempt, 0, len(ids))
	for _, id := range ids {
		out = append(out, *r.byID[id])
	}
	return out
}

func (r *EmailDeliveryAttemptRepository) attemptKey(notificationID string, attemptNo int) string {
	return fmt.Sprintf("%s|%d", notificationID, attemptNo)
}
