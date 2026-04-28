package inmemory

import (
	"context"
	"strings"
	"sync"
	"time"

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

func (r *Repository) ListByCompany(_ context.Context, companyID, action, resourceType, resourceID, fromOccurredAt, toOccurredAt, cursor string, limit int) ([]auditapp.Entry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 {
		limit = 50
	}
	out := make([]auditapp.Entry, 0, limit)
	fromTime, hasFrom := parseRFC3339(fromOccurredAt)
	toTime, hasTo := parseRFC3339(toOccurredAt)
	cursorTime, hasCursor := parseRFC3339(cursor)
	for i := len(r.entries) - 1; i >= 0; i-- {
		item := r.entries[i]
		occurredAt, hasOccurred := parseRFC3339(item.OccurredAt)
		if hasFrom && hasOccurred && occurredAt.Before(fromTime) {
			continue
		}
		if hasTo && hasOccurred && occurredAt.After(toTime) {
			continue
		}
		if hasCursor && hasOccurred && !occurredAt.Before(cursorTime) {
			continue
		}
		if strings.TrimSpace(companyID) != "" && !strings.EqualFold(strings.TrimSpace(item.CompanyID), strings.TrimSpace(companyID)) {
			continue
		}
		if strings.TrimSpace(action) != "" && !strings.EqualFold(strings.TrimSpace(item.Action), strings.TrimSpace(action)) {
			continue
		}
		if strings.TrimSpace(resourceType) != "" && !strings.EqualFold(strings.TrimSpace(item.ResourceType), strings.TrimSpace(resourceType)) {
			continue
		}
		if strings.TrimSpace(resourceID) != "" && !strings.EqualFold(strings.TrimSpace(item.ResourceID), strings.TrimSpace(resourceID)) {
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func parseRFC3339(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func (r *Repository) Snapshot() []auditapp.Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]auditapp.Entry, len(r.entries))
	copy(out, r.entries)
	return out
}
