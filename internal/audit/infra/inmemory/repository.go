package inmemory

import (
	"context"
	"sync"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
)

type Repository struct {
	mu      sync.RWMutex
	entries []auditapp.Entry
}

func NewRepository() *Repository { return &Repository{entries: []auditapp.Entry{}} }

func (r *Repository) Append(_ context.Context, entry auditapp.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, entry)
	return nil
}

func (r *Repository) Snapshot() []auditapp.Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]auditapp.Entry, len(r.entries))
	copy(out, r.entries)
	return out
}
