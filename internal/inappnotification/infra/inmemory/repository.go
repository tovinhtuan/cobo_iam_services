package inmemory

import (
	"context"
	"sort"
	"strings"
	"sync"

	inappapp "github.com/cobo/cobo_iam_services/internal/inappnotification/app"
)

// Repository is an in-memory implementation for tests.
type Repository struct {
	mu    sync.Mutex
	items map[string]inappapp.InAppNotification
}

func NewRepository() *Repository {
	return &Repository{items: map[string]inappapp.InAppNotification{}}
}

func (r *Repository) Create(_ context.Context, n inappapp.InAppNotification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[n.ID] = n
	return nil
}

func (r *Repository) ListByUser(_ context.Context, userID, companyID string, limit int) ([]inappapp.InAppNotification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []inappapp.InAppNotification
	for _, n := range r.items {
		if n.UserID == userID && n.CompanyID == companyID {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *Repository) UnreadCount(_ context.Context, userID, companyID string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, n := range r.items {
		if n.UserID == userID && n.CompanyID == companyID && !n.IsRead {
			count++
		}
	}
	return count, nil
}

func (r *Repository) MarkRead(_ context.Context, userID, notifID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	n, ok := r.items[notifID]
	if ok && n.UserID == userID {
		n.IsRead = true
		r.items[notifID] = n
	}
	return nil
}

func (r *Repository) MarkAllRead(_ context.Context, userID, companyID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, n := range r.items {
		if n.UserID == userID && n.CompanyID == companyID {
			n.IsRead = true
			r.items[id] = n
		}
	}
	return nil
}

// UserIDsByEmails is an in-memory test helper: maps email → fake user_id by prepending "u_".
type UserIDQuerier struct {
	mu    sync.Mutex
	email map[string]string // email → userID
}

func NewUserIDQuerier() *UserIDQuerier {
	return &UserIDQuerier{email: map[string]string{}}
}

func (q *UserIDQuerier) Register(email, userID string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.email[strings.ToLower(strings.TrimSpace(email))] = userID
}

func (q *UserIDQuerier) UserIDsByEmails(_ context.Context, _ string, emails []string) ([]string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []string
	for _, e := range emails {
		if uid, ok := q.email[strings.ToLower(strings.TrimSpace(e))]; ok {
			out = append(out, uid)
		}
	}
	return out, nil
}
