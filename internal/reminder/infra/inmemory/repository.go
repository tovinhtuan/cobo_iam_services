package inmemory

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
)

type Repository struct {
	mu          sync.RWMutex
	configs     map[string]reminderapp.ReminderConfigDTO
	occurrences map[string]reminderapp.ReminderOccurrenceDTO
	attempts    map[string][]reminderapp.ReminderDeliveryAttemptDTO
	// updatedAt mirrors the persistent `updated_at` column so the reaper predicate
	// (DISPATCHING older than X) is testable in-memory. Keyed by occurrence ID.
	updatedAt map[string]time.Time
}

func NewRepository() *Repository {
	return &Repository{
		configs:     make(map[string]reminderapp.ReminderConfigDTO),
		occurrences: make(map[string]reminderapp.ReminderOccurrenceDTO),
		attempts:    make(map[string][]reminderapp.ReminderDeliveryAttemptDTO),
		updatedAt:   make(map[string]time.Time),
	}
}

func (r *Repository) UpsertByScope(_ context.Context, in reminderapp.ReminderConfigDTO) (*reminderapp.ReminderConfigDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := configKey(in.ScopeType, in.ScopeID)
	existing, ok := r.configs[key]
	if ok {
		in.Version = existing.Version + 1
	} else {
		in.Version = 1
	}
	if in.UpdatedAt.IsZero() {
		in.UpdatedAt = time.Now().UTC()
	}
	r.configs[key] = in
	out := in
	return &out, nil
}

func (r *Repository) GetByScope(_ context.Context, scopeType reminderapp.ScopeType, scopeID string) (*reminderapp.ReminderConfigDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := configKey(scopeType, scopeID)
	cfg, ok := r.configs[key]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "reminder config not found", nil)
	}
	out := cfg
	return &out, nil
}

func (r *Repository) ListHistoryByDisclosure(_ context.Context, disclosureID string, q reminderapp.HistoryQuery) (*reminderapp.ReminderHistoryPage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]reminderapp.ReminderOccurrenceDTO, 0)
	for _, occ := range r.occurrences {
		if occ.DisclosureID != disclosureID {
			continue
		}
		if q.Scope != "" && q.Scope != "all" {
			if q.Scope == "disclosure" && occ.ScopeType != reminderapp.ScopeTypeDisclosure {
				continue
			}
			if q.Scope == "step" && occ.ScopeType != reminderapp.ScopeTypeWorkflowStep {
				continue
			}
		}
		if q.Status != "" && occ.Status != q.Status {
			continue
		}
		if q.From != nil && occ.ScheduledAt.Before(*q.From) {
			continue
		}
		if q.To != nil && occ.ScheduledAt.After(*q.To) {
			continue
		}
		items = append(items, occ)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].ScheduledAt.After(items[j].ScheduledAt)
	})

	page := q.Page
	if page <= 0 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}

	return &reminderapp.ReminderHistoryPage{
		Items:    items[start:end],
		Page:     page,
		PageSize: pageSize,
		Total:    len(items),
	}, nil
}

func (r *Repository) ClaimForDispatch(_ context.Context, occurrenceID string) (*reminderapp.ReminderOccurrenceDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	occ, ok := r.occurrences[occurrenceID]
	if !ok {
		occ = reminderapp.ReminderOccurrenceDTO{
			OccurrenceID:   occurrenceID,
			ScopeType:      reminderapp.ScopeTypeDisclosure,
			ScopeID:        occurrenceID,
			DisclosureID:   occurrenceID,
			ScheduledAt:    time.Now().UTC(),
			Status:         reminderapp.ReminderStatusPending,
			AttemptCount:   0,
			IdempotencyKey: occurrenceID,
		}
	}
	occ.Status = reminderapp.ReminderStatusDispatching
	r.occurrences[occurrenceID] = occ
	r.updatedAt[occurrenceID] = time.Now().UTC()
	out := occ
	return &out, nil
}

func (r *Repository) UpdateDispatchResult(_ context.Context, in reminderapp.DispatchResultInput) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	occ, ok := r.occurrences[in.OccurrenceID]
	if !ok {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "occurrence not found", nil)
	}
	occ.Status = in.Status
	if in.IncrementAttempt {
		occ.AttemptCount++
		now := time.Now().UTC()
		occ.LastAttemptAt = &now
	}
	occ.LastErrorCode = strings.TrimSpace(in.LastErrorCode)
	occ.ProviderMessageID = strings.TrimSpace(in.ProviderMessageID)
	r.occurrences[in.OccurrenceID] = occ
	r.updatedAt[in.OccurrenceID] = time.Now().UTC()
	return nil
}

// RequeueStaleDispatching requeues DISPATCHING occurrences last touched before olderThan
// back to PENDING, mirroring the MySQL reaper predicate. Returns count requeued.
func (r *Repository) RequeueStaleDispatching(_ context.Context, olderThan time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	n := 0
	for id, occ := range r.occurrences {
		if occ.Status != reminderapp.ReminderStatusDispatching {
			continue
		}
		ts, ok := r.updatedAt[id]
		if ok && !ts.Before(olderThan.UTC()) {
			continue
		}
		occ.Status = reminderapp.ReminderStatusPending
		r.occurrences[id] = occ
		r.updatedAt[id] = time.Now().UTC()
		n++
	}
	return n, nil
}

func (r *Repository) InsertAttempt(_ context.Context, in reminderapp.ReminderDeliveryAttemptDTO) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.attempts[in.OccurrenceID] = append(r.attempts[in.OccurrenceID], in)
	return nil
}

func (r *Repository) SeedOccurrence(_ context.Context, in reminderapp.ReminderOccurrenceDTO) (*reminderapp.ReminderOccurrenceDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	occ := in
	if occ.OccurrenceID == "" {
		occ.OccurrenceID = occ.IdempotencyKey
	}
	r.occurrences[occ.OccurrenceID] = occ
	r.updatedAt[occ.OccurrenceID] = time.Now().UTC()
	out := occ
	return &out, nil
}

func (r *Repository) MaterializeDueOccurrences(_ context.Context, _ time.Time) (int, error) {
	// In-memory scheduler materialization is intentionally no-op in this scaffold.
	return 0, nil
}

func (r *Repository) ListDispatchCandidates(_ context.Context, now time.Time, limit int) ([]reminderapp.DispatchCandidate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 {
		limit = 50
	}
	type pair struct {
		occ reminderapp.ReminderOccurrenceDTO
		cfg reminderapp.ReminderConfigDTO
	}
	pairs := make([]pair, 0)
	for _, occ := range r.occurrences {
		if occ.Status != reminderapp.ReminderStatusPending && occ.Status != reminderapp.ReminderStatusRetryScheduled {
			continue
		}
		if occ.Status == reminderapp.ReminderStatusPending && occ.ScheduledAt.After(now) {
			continue
		}
		cfg, ok := r.configs[configKey(occ.ScopeType, occ.ScopeID)]
		if !ok {
			continue
		}
		pairs = append(pairs, pair{occ: occ, cfg: cfg})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].occ.ScheduledAt.Before(pairs[j].occ.ScheduledAt) })
	if len(pairs) > limit {
		pairs = pairs[:limit]
	}
	out := make([]reminderapp.DispatchCandidate, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, reminderapp.DispatchCandidate{
			OccurrenceID:   p.occ.OccurrenceID,
			IdempotencyKey: p.occ.IdempotencyKey,
			TemplateCode:   "REMINDER_DISCLOSURE_DUE",
			TemplatePayload: map[string]any{
				"disclosure_id": p.occ.DisclosureID,
				"scope_type":    string(p.occ.ScopeType),
				"scope_id":      p.occ.ScopeID,
				"title":         p.occ.DisclosureID,
				"deadline_date": p.occ.ScheduledAt.UTC().Format("2006-01-02"),
				"scheduled_at":  p.occ.ScheduledAt.UTC().Format(time.RFC3339),
				"action_url":    "/app/disclosures/" + p.occ.DisclosureID,
			},
			RecipientEmails: append([]string{}, p.cfg.Config.Recipients...),
			CurrentAttempt:  p.occ.AttemptCount,
			ScheduledAt:     p.occ.ScheduledAt,
			DeadlineAt:      p.occ.ScheduledAt,
		})
	}
	return out, nil
}

func configKey(scopeType reminderapp.ScopeType, scopeID string) string {
	return string(scopeType) + ":" + scopeID
}
