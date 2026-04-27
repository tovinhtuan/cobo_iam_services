package inmemory

import (
	"context"
	"strings"
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

func (r *Repository) ListByCompany(_ context.Context, companyID, action string, limit int) ([]auditapp.Entry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 {
		limit = 50
	}
	out := make([]auditapp.Entry, 0, limit)
	for i := len(r.entries) - 1; i >= 0; i-- {
		item := r.entries[i]
		if strings.TrimSpace(companyID) != "" && !strings.EqualFold(strings.TrimSpace(item.CompanyID), strings.TrimSpace(companyID)) {
			continue
		}
		if strings.TrimSpace(action) != "" && !strings.EqualFold(strings.TrimSpace(item.Action), strings.TrimSpace(action)) {
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (r *Repository) Snapshot() []auditapp.Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]auditapp.Entry, len(r.entries))
	copy(out, r.entries)
	return out
}
