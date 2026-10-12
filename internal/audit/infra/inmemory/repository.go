package inmemory

import (
	"context"
	"sort"
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

func (r *Repository) ListByCompany(ctx context.Context, companyID, action, resourceType, resourceID, fromOccurredAt, toOccurredAt, cursor string, limit int) ([]auditapp.Entry, error) {
	return r.ListFiltered(ctx, auditapp.ListFilter{
		CompanyID:      companyID,
		Action:         action,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		FromOccurredAt: fromOccurredAt,
		ToOccurredAt:   toOccurredAt,
		Cursor:         cursor,
		Limit:          limit,
	})
}

func (r *Repository) ListFiltered(_ context.Context, filter auditapp.ListFilter) ([]auditapp.Entry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	out := make([]auditapp.Entry, 0, limit)
	fromTime, hasFrom := parseRFC3339(filter.FromOccurredAt)
	toTime, hasTo := parseRFC3339(filter.ToOccurredAt)
	cursorTime, hasCursor := parseRFC3339(filter.Cursor)
	companyID := strings.TrimSpace(filter.CompanyID)
	actorUserID := strings.TrimSpace(filter.ActorUserID)
	action := strings.TrimSpace(filter.Action)
	resourceType := strings.TrimSpace(filter.ResourceType)
	resourceID := strings.TrimSpace(filter.ResourceID)
	cursorEventID := strings.TrimSpace(filter.CursorEventID)
	entries := r.entries
	if filter.KeysetByEventID {
		// Same order as MySQL (occurred_at DESC, event_id DESC); the loop below walks the slice backwards.
		entries = append([]auditapp.Entry(nil), r.entries...)
		sort.SliceStable(entries, func(a, b int) bool {
			ta, _ := parseRFC3339(entries[a].OccurredAt)
			tb, _ := parseRFC3339(entries[b].OccurredAt)
			if !ta.Equal(tb) {
				return ta.Before(tb)
			}
			return entries[a].EventID < entries[b].EventID
		})
	}
	for i := len(entries) - 1; i >= 0; i-- {
		item := entries[i]
		occurredAt, hasOccurred := parseRFC3339(item.OccurredAt)
		if hasFrom && hasOccurred && occurredAt.Before(fromTime) {
			continue
		}
		if hasTo && hasOccurred && occurredAt.After(toTime) {
			continue
		}
		if hasCursor && hasOccurred {
			keysetTie := filter.KeysetByEventID && cursorEventID != "" && occurredAt.Equal(cursorTime) && item.EventID < cursorEventID
			if !occurredAt.Before(cursorTime) && !keysetTie {
				continue
			}
		}
		if companyID != "" && !strings.EqualFold(strings.TrimSpace(item.CompanyID), companyID) {
			continue
		}
		if actorUserID != "" && !strings.EqualFold(strings.TrimSpace(item.ActorUserID), actorUserID) {
			continue
		}
		if hasAnyPrefix(item.Action, filter.ExcludedPrefixes()) {
			continue
		}
		if !matchActionFilter(item.Action, action, filter.ActionPrefix, filter.RequireAdminPrefix) {
			continue
		}
		if resourceType != "" && !strings.EqualFold(strings.TrimSpace(item.ResourceType), resourceType) {
			continue
		}
		if resourceID != "" && !strings.EqualFold(strings.TrimSpace(item.ResourceID), resourceID) {
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func matchActionFilter(itemAction, filterAction string, actionPrefix, requireAdminPrefix bool) bool {
	itemAction = strings.TrimSpace(itemAction)
	filterAction = strings.TrimSpace(filterAction)
	switch {
	case actionPrefix && filterAction != "":
		return strings.HasPrefix(strings.ToLower(itemAction), strings.ToLower(filterAction))
	case filterAction != "":
		return strings.EqualFold(itemAction, filterAction)
	case requireAdminPrefix:
		return strings.HasPrefix(strings.ToLower(itemAction), "admin.")
	default:
		return true
	}
}

func (r *Repository) Snapshot() []auditapp.Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]auditapp.Entry, len(r.entries))
	copy(out, r.entries)
	return out
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

func hasAnyPrefix(action string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(action, p) {
			return true
		}
	}
	return false
}
