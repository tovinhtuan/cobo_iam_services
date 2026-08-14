package companyplan

import (
	"context"
	"strings"
	"sync"
	"time"
)

// MemoryRepository is an in-memory Repository for unit tests (no SQL).
type MemoryRepository struct {
	mu   sync.Mutex
	rows map[string]CompanyPlan
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{rows: map[string]CompanyPlan{}}
}

func (m *MemoryRepository) GetEffectivePlan(_ context.Context, companyID string, at time.Time) (*CompanyPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var candidates []CompanyPlan
	for _, p := range m.rows {
		if p.CompanyID == companyID {
			candidates = append(candidates, p)
		}
	}
	return SelectEffectivePlan(candidates, at), nil
}

func (m *MemoryRepository) GetEffectivePlans(ctx context.Context, companyIDs []string, at time.Time) (map[string]*CompanyPlan, error) {
	out := map[string]*CompanyPlan{}
	for _, id := range uniqueNonEmpty(companyIDs) {
		p, err := m.GetEffectivePlan(ctx, id, at)
		if err != nil {
			return nil, err
		}
		if p != nil {
			out[id] = p
		}
	}
	return out, nil
}

func (m *MemoryRepository) ListOccupyingByCompany(_ context.Context, companyID string) ([]CompanyPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []CompanyPlan
	for _, p := range m.rows {
		if p.CompanyID == companyID && IsOccupyingStatus(p.Status) {
			out = append(out, p)
		}
	}
	return out, nil
}

func (m *MemoryRepository) Create(_ context.Context, plan CompanyPlan) error {
	plan = NormalizeUTC(plan)
	if err := ValidateCreate(plan); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var existing []CompanyPlan
	for _, p := range m.rows {
		existing = append(existing, p)
	}
	if OccupyingOverlap(existing, plan, "") {
		return ErrOverlap
	}
	if _, ok := m.rows[plan.ID]; ok {
		return ErrInvalidPlan
	}
	now := NowUTC()
	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = now
	}
	if plan.UpdatedAt.IsZero() {
		plan.UpdatedAt = now
	}
	m.rows[plan.ID] = plan
	return nil
}

func (m *MemoryRepository) ActivateImmediate(_ context.Context, companyID string, code PlanCode, origin RecordOrigin, newID string) (*ActivateOutcome, error) {
	companyID = strings.TrimSpace(companyID)
	newID = strings.TrimSpace(newID)
	if strings.TrimSpace(string(origin)) == "" {
		origin = RecordOriginPlatformAdminManual
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var existing []CompanyPlan
	for _, p := range m.rows {
		if p.CompanyID == companyID {
			existing = append(existing, p)
		}
	}
	now := NowUTC()
	closes, create, already, previous, err := prepareImmediateActivation(existing, companyID, code, origin, newID, now)
	if err != nil {
		return nil, err
	}
	if already != nil {
		return &ActivateOutcome{Plan: *already, AlreadyActive: true, PreviousCode: previous}, nil
	}
	var closedIDs []string
	for _, cl := range closes {
		row, ok := m.rows[cl.ID]
		if !ok {
			continue
		}
		row.Status = cl.Status
		row.ExpiresAt = cl.ExpiresAt
		row.UpdatedAt = now
		m.rows[cl.ID] = row
		closedIDs = append(closedIDs, cl.ID)
	}
	if _, ok := m.rows[create.ID]; ok {
		return nil, ErrInvalidPlan
	}
	m.rows[create.ID] = create
	return &ActivateOutcome{Plan: create, PreviousCode: previous, ClosedIDs: closedIDs}, nil
}

func (m *MemoryRepository) DeleteByIDs(_ context.Context, ids []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range ids {
		delete(m.rows, id)
	}
	return nil
}

func (m *MemoryRepository) DeleteByOrigin(_ context.Context, origin RecordOrigin) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, p := range m.rows {
		if p.Origin == origin {
			delete(m.rows, id)
		}
	}
	return nil
}

var _ Repository = (*MemoryRepository)(nil)
