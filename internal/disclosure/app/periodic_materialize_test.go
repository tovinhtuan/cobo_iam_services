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
	retries         map[string]string
	failed          map[string]string
	claimAttempts   map[string]int
	mu              sync.Mutex
	tryClaimReturns map[string]bool
}

func newFakePeriodicRepo(cycles []PeriodicCycleRow) *fakePeriodicRepo {
	return &fakePeriodicRepo{
		cycles:          cycles,
		claimed:         map[string]bool{},
		completed:       map[string]string{},
		retries:         map[string]string{},
		failed:          map[string]string{},
		claimAttempts:   map[string]int{},
		tryClaimReturns: map[string]bool{},
	}
}

func (r *fakePeriodicRepo) ListActivePeriodicTypes(context.Context) ([]PeriodicTypeRow, error) {
	return nil, nil
}

func (r *fakePeriodicRepo) UpsertPeriodicCycle(context.Context, PeriodicCycleRow) error { return nil }

func (r *fakePeriodicRepo) ListPendingCycles(_ context.Context, asOf time.Time, bufferDays int) ([]PeriodicCycleRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := asOf.AddDate(0, 0, bufferDays)
	out := make([]PeriodicCycleRow, 0, len(r.cycles))
	for _, c := range r.cycles {
		if _, failed := r.failed[c.CycleID]; failed {
			continue
		}
		gate := c.OpenAt
		if gate.IsZero() {
			gate = c.CycleStart
		}
		if gate.IsZero() {
			continue
		}
		gateDay := time.Date(gate.Year(), gate.Month(), gate.Day(), 0, 0, 0, 0, time.UTC)
		cutDay := time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 0, 0, 0, 0, time.UTC)
		if gateDay.After(cutDay) {
			continue
		}
		out = append(out, c)
	}
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

func (r *fakePeriodicRepo) MarkPeriodicCycleRetry(_ context.Context, cycleID, attemptRecordID, _, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.retries[cycleID] = attemptRecordID
	delete(r.claimed, cycleID)
	return nil
}

func (r *fakePeriodicRepo) MarkPeriodicCycleFailed(_ context.Context, cycleID, attemptRecordID, _, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failed[cycleID] = attemptRecordID
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

func (r *fakePeriodicRepo) ListCompanyTypePreferencesByTypeIDs(context.Context, []string) ([]CompanyTypePreference, error) {
	return nil, nil
}

func (r *fakePeriodicRepo) DeactivateIncompatibleCompanyCycleOverrides(context.Context, string, string) (int64, error) {
	return 0, nil
}

type fakePeriodicCreator struct {
	mu                 sync.Mutex
	recordID           string
	workflowInstanceID string
	err                error
	lastT0             *time.Time
	lastPlannedDate    string
	calls              int
}

func (f *fakePeriodicCreator) CreateAndSubmitRecord(_ context.Context, _, _, _, _ string, t0Date *time.Time) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastT0 = t0Date
	f.lastPlannedDate = ""
	if f.err != nil {
		return "", "", f.err
	}
	return f.recordID, f.workflowInstanceID, nil
}

func (f *fakePeriodicCreator) CreateAndSubmitRecordWithPlannedDate(_ context.Context, _, _, _, _ string, t0Date *time.Time, plannedDate string) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastT0 = t0Date
	f.lastPlannedDate = plannedDate
	if f.err != nil {
		return "", "", f.err
	}
	return f.recordID, f.workflowInstanceID, nil
}

func (f *fakePeriodicCreator) CreateAndSubmitPeriodicRecord(_ context.Context, _ string, _, _, _, _ string, t0Date *time.Time, plannedDate string) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastT0 = t0Date
	f.lastPlannedDate = plannedDate
	if f.err != nil {
		return f.recordID, f.workflowInstanceID, f.err
	}
	return f.recordID, f.workflowInstanceID, nil
}

func TestMaterializePeriodicUsesCycleStartAsT0(t *testing.T) {
	cycleStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	dueDate := cycleStart.AddDate(0, 0, 7)
	repo := newFakePeriodicRepo([]PeriodicCycleRow{{
		CycleID:    "cycle-1",
		TypeID:     "type-1",
		CompanyID:  "co-1",
		CycleLabel: "2026-05",
		CycleStart: cycleStart,
		DueDate:    dueDate,
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
	wantPlannedDate := dueDate.Format("2006-01-02")
	if creator.lastPlannedDate != wantPlannedDate {
		t.Fatalf("planned_date=%q want %q", creator.lastPlannedDate, wantPlannedDate)
	}
	if repo.completed["cycle-1"] != "rec-1" {
		t.Fatalf("completed record=%q", repo.completed["cycle-1"])
	}
}

func TestMaterializePeriodicSetsPlannedDateFromDueDate(t *testing.T) {
	cycleStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC)
	repo := newFakePeriodicRepo([]PeriodicCycleRow{{
		CycleID:    "cycle-q2",
		TypeID:     "type-qreport",
		CompanyID:  "co-1",
		CycleLabel: "2026-Q2",
		CycleStart: cycleStart,
		DueDate:    dueDate,
	}})
	creator := &fakePeriodicCreator{recordID: "rec-q2", workflowInstanceID: "wf-q2"}

	n, err := materializePeriodicDisclosures(context.Background(), time.Now(), repo, creator)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if n != 1 {
		t.Fatalf("materialized=%d want 1", n)
	}
	if creator.lastPlannedDate != "2026-04-29" {
		t.Fatalf("planned_date=%q want %q", creator.lastPlannedDate, "2026-04-29")
	}
}

func TestMaterializePeriodicZeroDueDateEmptyPlannedDate(t *testing.T) {
	cycleStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	repo := newFakePeriodicRepo([]PeriodicCycleRow{{
		CycleID:    "cycle-zero",
		TypeID:     "type-1",
		CompanyID:  "co-1",
		CycleLabel: "2026-Q2",
		CycleStart: cycleStart,
		DueDate:    time.Time{}, // zero: no due_date calculated
	}})
	creator := &fakePeriodicCreator{recordID: "rec-1", workflowInstanceID: "wf-1"}

	_, err := materializePeriodicDisclosures(context.Background(), time.Now(), repo, creator)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if creator.lastPlannedDate != "" {
		t.Fatalf("expected empty planned_date for zero DueDate, got %q", creator.lastPlannedDate)
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
	if _, ok := repo.failed["cycle-1"]; !ok {
		t.Fatal("empty effective workflow must mark the cycle FAILED")
	}
}

func TestMaterializePeriodic_RetriesPartialRecordWithoutDuplicateCycleCompletion(t *testing.T) {
	cycleStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	repo := newFakePeriodicRepo([]PeriodicCycleRow{{
		CycleID: "cycle-retry", TypeID: "type-1", CompanyID: "co-1", CycleStart: cycleStart,
		DueDate: cycleStart.AddDate(0, 0, 7),
	}})
	creator := &fakePeriodicCreator{recordID: "rec-deterministic", err: context.DeadlineExceeded}

	n, err := materializePeriodicDisclosures(context.Background(), time.Now(), repo, creator)
	if err != nil || n != 0 {
		t.Fatalf("first materialize n=%d err=%v", n, err)
	}
	if got := repo.retries["cycle-retry"]; got != "rec-deterministic" {
		t.Fatalf("retry attempt record=%q want deterministic record", got)
	}
	if repo.claimed["cycle-retry"] {
		t.Fatal("retry must release the claim")
	}

	creator.err = nil
	creator.workflowInstanceID = "wf-1"
	n, err = materializePeriodicDisclosures(context.Background(), time.Now(), repo, creator)
	if err != nil || n != 1 {
		t.Fatalf("retry materialize n=%d err=%v", n, err)
	}
	if got := repo.completed["cycle-retry"]; got != "rec-deterministic" {
		t.Fatalf("completed record=%q want deterministic record", got)
	}
	if creator.calls != 2 {
		t.Fatalf("creator calls=%d want 2", creator.calls)
	}
}

func TestMaterializePeriodic_ConcurrentTicksHaveSingleWinner(t *testing.T) {
	cycleStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	repo := newFakePeriodicRepo([]PeriodicCycleRow{{
		CycleID: "cycle-race", TypeID: "type-1", CompanyID: "co-1", CycleStart: cycleStart,
		DueDate: cycleStart.AddDate(0, 0, 7),
	}})
	creator := &fakePeriodicCreator{recordID: "rec-race", workflowInstanceID: "wf-race"}

	start := make(chan struct{})
	results := make(chan int, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			n, err := materializePeriodicDisclosures(context.Background(), time.Now(), repo, creator)
			results <- n
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)

	total := 0
	for n := range results {
		total += n
	}
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent materialize: %v", err)
		}
	}
	creator.mu.Lock()
	calls := creator.calls
	creator.mu.Unlock()
	if total != 1 || calls != 1 {
		t.Fatalf("materialized=%d creator_calls=%d; want a single winner", total, calls)
	}
	if got := repo.completed["cycle-race"]; got != "rec-race" {
		t.Fatalf("completed record=%q", got)
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

func TestMaterializePeriodic_PreopenExcludedWithBufferZero(t *testing.T) {
	loc := asiaHoChiMinh()
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, loc)
	openAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	cycleStart := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	repo := newFakePeriodicRepo([]PeriodicCycleRow{{
		CycleID: "c-pre", TypeID: "t1", CompanyID: "co-1", CycleLabel: "2026-10",
		CycleStart: cycleStart, OpenAt: openAt, DueDate: cycleStart.AddDate(0, 0, 7),
	}})
	creator := &fakePeriodicCreator{recordID: "r1", workflowInstanceID: "w1"}

	pending, err := repo.ListPendingCycles(context.Background(), stripTime(now), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("PREOPEN buffer=0 must exclude; got %d", len(pending))
	}
	n, err := materializePeriodicDisclosures(context.Background(), now, repo, creator)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 || creator.calls != 0 {
		t.Fatalf("materialize want 0; n=%d calls=%d", n, creator.calls)
	}
}

func TestMaterializePeriodic_AtAndAfterOpenAt(t *testing.T) {
	loc := asiaHoChiMinh()
	openAt := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	cycleStart := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	cycle := PeriodicCycleRow{
		CycleID: "c-open", TypeID: "t1", CompanyID: "co-1", CycleLabel: "2026-09",
		CycleStart: cycleStart, OpenAt: openAt, DueDate: cycleStart.AddDate(0, 0, 7),
	}

	t.Run("at", func(t *testing.T) {
		repo := newFakePeriodicRepo([]PeriodicCycleRow{cycle})
		creator := &fakePeriodicCreator{recordID: "r1", workflowInstanceID: "w1"}
		now := time.Date(2026, 9, 10, 8, 0, 0, 0, loc)
		pending, _ := repo.ListPendingCycles(context.Background(), stripTime(now), 0)
		if len(pending) != 1 {
			t.Fatalf("at OpenAt want 1 pending got %d", len(pending))
		}
		n, err := materializePeriodicDisclosures(context.Background(), now, repo, creator)
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("materialized=%d want 1", n)
		}
	})

	t.Run("after_once", func(t *testing.T) {
		repo := newFakePeriodicRepo([]PeriodicCycleRow{cycle})
		creator := &fakePeriodicCreator{recordID: "r1", workflowInstanceID: "w1"}
		now := time.Date(2026, 9, 12, 8, 0, 0, 0, loc)
		n, err := materializePeriodicDisclosures(context.Background(), now, repo, creator)
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("first materialize=%d want 1", n)
		}
		repo.tryClaimReturns["c-open"] = false
		n, err = materializePeriodicDisclosures(context.Background(), now, repo, creator)
		if err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("second materialize=%d want 0", n)
		}
	})
}
