package app

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeAlertConfigRepo is an in-memory AlertConfigRepository for unit tests.
type fakeAlertConfigRepo struct {
	mu   sync.Mutex
	rows []AlertTemplateConfig
}

func (f *fakeAlertConfigRepo) GetByTypeID(_ context.Context, typeID string) ([]AlertTemplateConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []AlertTemplateConfig
	for _, r := range f.rows {
		if r.TypeID == typeID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeAlertConfigRepo) GetByTypeAndKind(_ context.Context, typeID, alertKind string) (*AlertTemplateConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.rows {
		if f.rows[i].TypeID == typeID && f.rows[i].AlertKind == alertKind {
			cp := f.rows[i]
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeAlertConfigRepo) Upsert(_ context.Context, in AlertTemplateConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.rows {
		if f.rows[i].TypeID == in.TypeID && f.rows[i].AlertKind == in.AlertKind {
			in.UpdatedAt = time.Now()
			f.rows[i] = in
			return nil
		}
	}
	in.ID = int64(len(f.rows) + 1)
	in.CreatedAt = time.Now()
	in.UpdatedAt = in.CreatedAt
	f.rows = append(f.rows, in)
	return nil
}

// Compile-time: fakeAlertConfigRepo satisfies AlertConfigRepository.
var _ AlertConfigRepository = (*fakeAlertConfigRepo)(nil)

func TestFakeAlertConfigRepo_UpsertAndGet(t *testing.T) {
	ctx := context.Background()
	repo := &fakeAlertConfigRepo{}

	cfg := AlertTemplateConfig{
		TypeID:      "dt-periodic-financial",
		AlertKind:   AlertKindDeadline,
		TemplateKey: "reminder.deadline_approaching",
		Enabled:     true,
		CreatedBy:   "admin",
	}
	if err := repo.Upsert(ctx, cfg); err != nil {
		t.Fatalf("Upsert error: %v", err)
	}

	got, err := repo.GetByTypeAndKind(ctx, "dt-periodic-financial", AlertKindDeadline)
	if err != nil {
		t.Fatalf("GetByTypeAndKind error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if got.TemplateKey != "reminder.deadline_approaching" {
		t.Errorf("TemplateKey = %q, want %q", got.TemplateKey, "reminder.deadline_approaching")
	}
	if !got.Enabled {
		t.Error("expected Enabled=true")
	}
}

func TestFakeAlertConfigRepo_UpsertUpdatesExisting(t *testing.T) {
	ctx := context.Background()
	repo := &fakeAlertConfigRepo{}

	base := AlertTemplateConfig{
		TypeID: "dt-event", AlertKind: AlertKindWorkflowStep,
		TemplateKey: "reminder.workflow_step_due", Enabled: true, CreatedBy: "admin",
	}
	_ = repo.Upsert(ctx, base)

	// Update: disable the same kind
	updated := base
	updated.Enabled = false
	updated.TemplateKey = "reminder.workflow_step_due_v2"
	_ = repo.Upsert(ctx, updated)

	rows, _ := repo.GetByTypeID(ctx, "dt-event")
	if len(rows) != 1 {
		t.Fatalf("expected 1 row after double upsert, got %d", len(rows))
	}
	if rows[0].Enabled {
		t.Error("expected Enabled=false after update")
	}
	if rows[0].TemplateKey != "reminder.workflow_step_due_v2" {
		t.Errorf("TemplateKey = %q after update", rows[0].TemplateKey)
	}
}

func TestFakeAlertConfigRepo_GetByTypeID_ReturnsBothKinds(t *testing.T) {
	ctx := context.Background()
	repo := &fakeAlertConfigRepo{}

	_ = repo.Upsert(ctx, AlertTemplateConfig{TypeID: "dt-x", AlertKind: AlertKindDeadline, TemplateKey: "reminder.deadline_approaching", Enabled: true, CreatedBy: "a"})
	_ = repo.Upsert(ctx, AlertTemplateConfig{TypeID: "dt-x", AlertKind: AlertKindWorkflowStep, TemplateKey: "reminder.workflow_step_due", Enabled: true, CreatedBy: "a"})
	_ = repo.Upsert(ctx, AlertTemplateConfig{TypeID: "dt-other", AlertKind: AlertKindDeadline, TemplateKey: "reminder.deadline_approaching", Enabled: true, CreatedBy: "a"})

	rows, err := repo.GetByTypeID(ctx, "dt-x")
	if err != nil {
		t.Fatalf("GetByTypeID error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows for dt-x, got %d", len(rows))
	}
}

func TestFakeAlertConfigRepo_GetByTypeAndKind_MissingReturnsNil(t *testing.T) {
	ctx := context.Background()
	repo := &fakeAlertConfigRepo{}

	got, err := repo.GetByTypeAndKind(ctx, "nonexistent", AlertKindDeadline)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for missing type, got %+v", got)
	}
}
