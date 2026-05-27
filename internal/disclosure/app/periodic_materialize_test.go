package app

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	workflowerrs "github.com/cobo/cobo_iam_services/internal/workflow/errs"
)

type fakePeriodicRepo struct {
	cycles          []PeriodicCycleRow
	claimed         map[string]bool
	completed       map[string]string
	claimAttempts   map[string]int
	mu              sync.Mutex
	tryClaimReturns map[string]bool
}

func newFakePeriodicRepo(cycles []PeriodicCycleRow) *fakePeriodicRepo {
	return &fakePeriodicRepo{
		cycles:          cycles,
		claimed:         map[string]bool{},
		completed:       map[string]string{},
		claimAttempts:   map[string]int{},
		tryClaimReturns: map[string]bool{},
	}
}

func (r *fakePeriodicRepo) ListActivePeriodicTypes(context.Context) ([]PeriodicTypeRow, error) {
	return nil, nil
}

func (r *fakePeriodicRepo) UpsertPeriodicCycle(context.Context, PeriodicCycleRow) error { return nil }

func (r *fakePeriodicRepo) ListPendingCycles(_ context.Context, _ time.Time, _ int) ([]PeriodicCycleRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]PeriodicCycleRow, len(r.cycles))
	copy(out, r.cycles)
	return out, nil
}

func (r *fakePeriodicRepo) TryClaimPeriodicCycle(_ context.Context, cycleID string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.claimAttempts[cycleID]++
	if v, ok := r.tryClaimReturns[cycleID]; ok {
		if v {
			r.claimed[cycleID] = true
		}
		return v, nil
	}
	if r.claimed[cycleID] || r.completed[cycleID] != "" {
		return false, nil
	}
	r.claimed[cycleID] = true
	return true, nil
}

func (r *fakePeriodicRepo) ReleasePeriodicCycleClaim(_ context.Context, cycleID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.claimed, cycleID)
	return nil
}

func (r *fakePeriodicRepo) UpdateCycleRecord(_ context.Context, cycleID, recordID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.claimed[cycleID] {
		return nil
	}
	r.completed[cycleID] = recordID
	delete(r.claimed, cycleID)
	return nil
}

func (r *fakePeriodicRepo) ListAllActiveCompanyIDs(context.Context) ([]string, error) {
	return nil, nil
}

func (r *fakePeriodicRepo) GetCompanyTypePreference(context.Context, string, string) (*CompanyTypePreference, error) {
	return nil, nil
}

func (r *fakePeriodicRepo) UpsertCompanyTypePreference(context.Context, CompanyTypePreference) error {
	return nil
}

type fakePeriodicCreator struct {
	recordID           string
	workflowInstanceID string
	err                error
	lastT0             *time.Time
	calls              int
}

func (f *fakePeriodicCreator) CreateAndSubmitRecord(_ context.Context, _, _, _, _ string, t0Date *time.Time) (string, string, error) {
	f.calls++
	f.lastT0 = t0Date
	if f.err != nil {
		return "", "", f.err
	}
	return f.recordID, f.workflowInstanceID, nil
}

func TestMaterializePeriodicUsesCycleStartAsT0(t *testing.T) {
	cycleStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	repo := newFakePeriodicRepo([]PeriodicCycleRow{{
		CycleID:    "cycle-1",
		TypeID:     "type-1",
		CompanyID:  "co-1",
		CycleLabel: "2026-05",
		CycleStart: cycleStart,
		DueDate:    cycleStart.AddDate(0, 0, 7),
	}})
	creator := &fakePeriodicCreator{recordID: "rec-1", workflowInstanceID: "wf-1"}

	n, err := materializePeriodicDisclosures(context.Background(), time.Now(), repo, creator)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if n != 1 {
		t.Fatalf("materialized=%d want 1", n)
	}
	if creator.lastT0 == nil || !creator.lastT0.Equal(cycleStart) {
		t.Fatalf("t0=%v want %v", creator.lastT0, cycleStart)
	}
	if repo.completed["cycle-1"] != "rec-1" {
		t.Fatalf("completed record=%q", repo.completed["cycle-1"])
	}
}

func TestMaterializePeriodicClaimPreventsDuplicateWork(t *testing.T) {
	cycleStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	repo := newFakePeriodicRepo([]PeriodicCycleRow{{
		CycleID:    "cycle-1",
		TypeID:     "type-1",
		CompanyID:  "co-1",
		CycleStart: cycleStart,
		DueDate:    cycleStart.AddDate(0, 0, 7),
	}})
	repo.tryClaimReturns["cycle-1"] = false
	creator := &fakePeriodicCreator{recordID: "rec-1", workflowInstanceID: "wf-1"}

	n, err := materializePeriodicDisclosures(context.Background(), time.Now(), repo, creator)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if n != 0 {
		t.Fatalf("materialized=%d want 0", n)
	}
	if creator.calls != 0 {
		t.Fatalf("creator calls=%d want 0", creator.calls)
	}
	if len(repo.completed) != 0 {
		t.Fatalf("unexpected completion: %#v", repo.completed)
	}
}

func TestMaterializePeriodicEmptyEffectiveLeavesNoRecordID(t *testing.T) {
	cycleStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	repo := newFakePeriodicRepo([]PeriodicCycleRow{{
		CycleID:    "cycle-1",
		TypeID:     "type-1",
		CompanyID:  "co-1",
		CycleStart: cycleStart,
		DueDate:    cycleStart.AddDate(0, 0, 7),
	}})
	creator := &fakePeriodicCreator{
		err: perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "template has no effective workflow steps",
			workflowerrs.ErrEmptyWorkflowSnapshot),
	}

	n, err := materializePeriodicDisclosures(context.Background(), time.Now(), repo, creator)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if n != 0 {
		t.Fatalf("materialized=%d want 0", n)
	}
	if len(repo.completed) != 0 {
		t.Fatalf("completed=%#v want none", repo.completed)
	}
	if _, ok := repo.completed["cycle-1"]; ok {
		t.Fatal("cycle should not be completed")
	}
	if repo.claimed["cycle-1"] {
		t.Fatal("claim should be released after empty effective workflow")
	}
}

func TestMaterializePeriodicDoesNotCompleteWithoutWorkflowInstance(t *testing.T) {
	cycleStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	repo := newFakePeriodicRepo([]PeriodicCycleRow{{
		CycleID:    "cycle-1",
		TypeID:     "type-1",
		CompanyID:  "co-1",
		CycleStart: cycleStart,
		DueDate:    cycleStart.AddDate(0, 0, 7),
	}})
	creator := &fakePeriodicCreator{recordID: "rec-1", workflowInstanceID: ""}

	n, err := materializePeriodicDisclosures(context.Background(), time.Now(), repo, creator)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if n != 0 {
		t.Fatalf("materialized=%d want 0", n)
	}
	if len(repo.completed) != 0 {
		t.Fatalf("completed=%#v want none", repo.completed)
	}
}

func TestIsEmptyEffectiveWorkflowDetectsHTTPWrappedSnapshotError(t *testing.T) {
	err := perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "template has no effective workflow steps",
		workflowerrs.ErrEmptyWorkflowSnapshot)
	if !workflowerrs.IsEmptyEffectiveWorkflow(err) {
		t.Fatalf("expected empty effective workflow, got %v", err)
	}
}
