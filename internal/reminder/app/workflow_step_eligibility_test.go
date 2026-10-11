package app

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type fakeRuntimeReader struct {
	byInstance map[string]*WorkflowStepRuntime
	calls      int
}

func (f *fakeRuntimeReader) WorkflowStepRuntime(_ context.Context, _, instanceID string, _ time.Time) (*WorkflowStepRuntime, error) {
	f.calls++
	return f.byInstance[instanceID], nil
}

func (f *orderedMilestoneScanner) MarkMilestoneObsolete(_ context.Context, milestoneID string) error {
	f.obsolete[milestoneID] = true
	return nil
}

func (f *orderedMilestoneScanner) ListDueMilestonesPage(_ context.Context, asOf time.Time, after *MilestoneCursor, limit int) ([]DueMilestone, error) {
	var out []DueMilestone
	for _, m := range f.open() {
		if m.ScheduledDate.After(asOf) {
			continue
		}
		if after != nil {
			if m.ScheduledDate.Before(after.ScheduledDate) {
				continue
			}
			if m.ScheduledDate.Equal(after.ScheduledDate) && m.MilestoneID <= after.MilestoneID {
				continue
			}
		}
		out = append(out, m)
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out, nil
}

func hcmDate(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func legacyRuntime(steps ...WorkflowStepRuntimeStep) *WorkflowStepRuntime {
	return &WorkflowStepRuntime{InstanceStatus: "in_progress", WorkflowSource: "global_template", Steps: steps}
}

// RMC-01: on a non-proposal (global_template) workflow only step 1 ever has a task. After
// step 1 is completed on the deadline screen, step 2 must be reminded and step 1 must not.
func TestSeedOccurrences_LegacyStep2EligibleAfterStep1CompletedOnScreen(t *testing.T) {
	now := hcmDate(2026, 10, 10)
	scanner := newOrderedMilestoneScanner([]DueMilestone{
		{MilestoneID: "m-s1", CompanyID: "c1", WorkflowInstanceID: "inst-1", StepID: "s1", MilestoneType: "due_minus_1d", ScheduledDate: now},
		{MilestoneID: "m-s2", CompanyID: "c1", WorkflowInstanceID: "inst-1", StepID: "s2", MilestoneType: "due_minus_1d", ScheduledDate: now},
	})
	repo := newMySQLLikeOccurrenceRepo()
	rt := &fakeRuntimeReader{byInstance: map[string]*WorkflowStepRuntime{
		"inst-1": legacyRuntime(
			WorkflowStepRuntimeStep{StepID: "s1", StepCode: "s1", Completed: true, EndDate: hcmDate(2026, 10, 9), HasEnd: true},
			WorkflowStepRuntimeStep{StepID: "s2", StepCode: "s2", Current: true, EndDate: hcmDate(2026, 10, 11), HasEnd: true},
		),
	}}
	svc := NewService(fakeConfigRepo{}, repo, &fakeAttemptRepo{},
		WithMilestoneScanner(scanner),
		WithWorkflowStepTaskStateReader(fakeStepTaskStateReader{states: map[string]WorkflowStepTaskState{
			"s1": WorkflowStepTaskPending, // the only task never advances on legacy workflows
			"s2": WorkflowStepTaskAbsent,
		}}),
		WithWorkflowStepRuntimeReader(rt),
	)
	n, err := svc.SeedOccurrencesFromDueMilestones(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("seeded=%d want 1 (step 2 only)", n)
	}
	if _, ok := repo.byKey[WorkflowStepReminderIdempotencyKey("inst-1", "s2", "due_minus_1d")]; !ok {
		t.Fatalf("step 2 not seeded: %v", repo.byKey)
	}
	if _, ok := repo.byKey[WorkflowStepReminderIdempotencyKey("inst-1", "s1", "due_minus_1d")]; ok {
		t.Fatal("completed step 1 must not be reminded")
	}
	if !scanner.obsolete["m-s1"] {
		t.Fatal("completed step milestone must be marked obsolete so it leaves the queue")
	}
}

func TestDecideWorkflowStepReminder(t *testing.T) {
	today := hcmDate(2026, 10, 10)
	cur := WorkflowStepRuntimeStep{StepID: "s1", StepCode: "S1", Current: true, EndDate: hcmDate(2026, 10, 12), HasEnd: true}
	cases := []struct {
		name   string
		rt     *WorkflowStepRuntime
		task   WorkflowStepTaskState
		stepID string
		offset int
		want   stepReminderDecision
		reason string
	}{
		{"instance missing", nil, WorkflowStepTaskAbsent, "s1", 1, decisionObsolete, reasonInstanceMissing},
		{"instance approved", &WorkflowStepRuntime{InstanceStatus: "approved", Steps: []WorkflowStepRuntimeStep{cur}}, WorkflowStepTaskAbsent, "s1", 1, decisionObsolete, reasonInstanceClosed},
		{"step not in snapshot", legacyRuntime(cur), WorkflowStepTaskAbsent, "zz", 1, decisionObsolete, reasonStepNotInSnapshot},
		{"legacy current by step code", legacyRuntime(cur), WorkflowStepTaskAbsent, "S1", 2, decisionEligible, ""},
		{"legacy not current yet", legacyRuntime(WorkflowStepRuntimeStep{StepID: "s1", EndDate: hcmDate(2026, 10, 20), HasEnd: true}), WorkflowStepTaskAbsent, "s1", 1, decisionWait, reasonStepNotCurrent},
		{"legacy completed", legacyRuntime(WorkflowStepRuntimeStep{StepID: "s1", Completed: true}), WorkflowStepTaskPending, "s1", 1, decisionObsolete, reasonStepCompleted},
		{"current step expired after grace", legacyRuntime(WorkflowStepRuntimeStep{StepID: "s1", Current: true, EndDate: hcmDate(2026, 10, 7), HasEnd: true}), WorkflowStepTaskAbsent, "s1", 1, decisionObsolete, reasonStepExpired},
		// PERF-48: step 1 is late; step 2's own end passed but the plan (step 3) is not over → wait.
		{"later step waits while earlier step is late", legacyRuntime(
			WorkflowStepRuntimeStep{StepID: "s1", Current: true, EndDate: hcmDate(2026, 10, 1), HasEnd: true},
			WorkflowStepRuntimeStep{StepID: "s2", EndDate: hcmDate(2026, 10, 5), HasEnd: true},
			WorkflowStepRuntimeStep{StepID: "s3", EndDate: hcmDate(2026, 10, 9), HasEnd: true},
		), WorkflowStepTaskAbsent, "s2", 1, decisionWait, reasonStepNotCurrent},
		{"non-current step obsolete once the whole plan is past", legacyRuntime(
			WorkflowStepRuntimeStep{StepID: "s1", Current: true, EndDate: hcmDate(2026, 10, 1), HasEnd: true},
			WorkflowStepRuntimeStep{StepID: "s2", EndDate: hcmDate(2026, 10, 7), HasEnd: true},
		), WorkflowStepTaskAbsent, "s2", 1, decisionObsolete, reasonWorkflowExpired},
		{"inside grace still eligible", legacyRuntime(WorkflowStepRuntimeStep{StepID: "s1", Current: true, EndDate: hcmDate(2026, 10, 8), HasEnd: true}), WorkflowStepTaskAbsent, "s1", 1, decisionEligible, ""},
		{"proposal pending task", &WorkflowStepRuntime{InstanceStatus: "in_progress", WorkflowSource: "proposal_snapshot_v3", Steps: []WorkflowStepRuntimeStep{{StepID: "s1"}}}, WorkflowStepTaskPending, "s1", 1, decisionEligible, ""},
		{"proposal absent task waits", &WorkflowStepRuntime{InstanceStatus: "in_progress", WorkflowSource: "proposal_snapshot_v2", Steps: []WorkflowStepRuntimeStep{{StepID: "s1"}}}, WorkflowStepTaskAbsent, "s1", 1, decisionWait, reasonStepNotCurrent},
		{"proposal completed task", &WorkflowStepRuntime{InstanceStatus: "in_progress", WorkflowSource: "proposal_snapshot_v3", Steps: []WorkflowStepRuntimeStep{{StepID: "s1"}}}, WorkflowStepTaskCompleted, "s1", 1, decisionObsolete, reasonStepCompleted},
		// RMC-04: predecessor delay shifted the end; due_minus_3d of a step ending 10-20 waits until 10-17.
		{"delay shifted waits", legacyRuntime(WorkflowStepRuntimeStep{StepID: "s1", Current: true, EndDate: hcmDate(2026, 10, 20), HasEnd: true}), WorkflowStepTaskAbsent, "s1", 3, decisionWait, reasonDelayShifted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := decideWorkflowStepReminder(tc.rt, tc.task, tc.stepID, tc.offset, today)
			if got != tc.want || reason != tc.reason {
				t.Fatalf("got (%v,%q) want (%v,%q)", got, reason, tc.want, tc.reason)
			}
		})
	}
}

// REM-06: dead milestones (closed instances) must leave the queue instead of filling every
// LIMIT 200 batch, so a newer eligible milestone is still seeded.
func TestSeedOccurrences_DeadMilestonesDoNotStarveBatch(t *testing.T) {
	now := hcmDate(2026, 10, 10)
	var due []DueMilestone
	for i := 0; i < 200; i++ {
		due = append(due, DueMilestone{
			MilestoneID: fmt.Sprintf("dead-%03d", i), CompanyID: "c1", WorkflowInstanceID: "closed",
			StepID: "s1", MilestoneType: "due_minus_1d", ScheduledDate: now.AddDate(0, 0, -10+i%10),
		})
	}
	due = append(due, DueMilestone{MilestoneID: "live", CompanyID: "c1", WorkflowInstanceID: "open", StepID: "s1", MilestoneType: "due_minus_1d", ScheduledDate: now})
	scanner := newOrderedMilestoneScanner(due)
	repo := newMySQLLikeOccurrenceRepo()
	rt := &fakeRuntimeReader{byInstance: map[string]*WorkflowStepRuntime{
		"closed": {InstanceStatus: "approved", Steps: []WorkflowStepRuntimeStep{{StepID: "s1"}}},
		"open":   legacyRuntime(WorkflowStepRuntimeStep{StepID: "s1", Current: true, EndDate: hcmDate(2026, 10, 11), HasEnd: true}),
	}}
	svc := NewService(fakeConfigRepo{}, repo, &fakeAttemptRepo{},
		WithMilestoneScanner(scanner),
		WithWorkflowStepTaskStateReader(fakeStepTaskStateReader{}),
		WithWorkflowStepRuntimeReader(rt),
	)
	for tick := 0; tick < 2; tick++ {
		if _, err := svc.SeedOccurrencesFromDueMilestones(context.Background(), now); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := repo.byKey[WorkflowStepReminderIdempotencyKey("open", "s1", "due_minus_1d")]; !ok {
		t.Fatal("eligible milestone starved behind dead rows")
	}
	if len(scanner.obsolete) != 200 {
		t.Fatalf("obsoleted=%d want 200", len(scanner.obsolete))
	}
	if rt.calls > 2*3 {
		t.Fatalf("runtime reader must be cached per instance within a tick, calls=%d", rt.calls)
	}
}

// REM-06 (paging): rows that must wait (step not current yet) stay in the table but must
// not hide an eligible row behind them.
func TestSeedOccurrences_WaitRowsDoNotStarve(t *testing.T) {
	now := hcmDate(2026, 10, 10)
	var due []DueMilestone
	for i := 0; i < 250; i++ {
		due = append(due, DueMilestone{
			MilestoneID: fmt.Sprintf("wait-%03d", i), CompanyID: "c1", WorkflowInstanceID: "waiting",
			StepID: "s2", MilestoneType: "due_minus_1d", ScheduledDate: now.AddDate(0, 0, -1),
		})
	}
	due = append(due, DueMilestone{MilestoneID: "zz-live", CompanyID: "c1", WorkflowInstanceID: "open", StepID: "s1", MilestoneType: "due_minus_1d", ScheduledDate: now})
	scanner := newOrderedMilestoneScanner(due)
	repo := newMySQLLikeOccurrenceRepo()
	rt := &fakeRuntimeReader{byInstance: map[string]*WorkflowStepRuntime{
		"waiting": legacyRuntime(
			WorkflowStepRuntimeStep{StepID: "s1", Current: true, EndDate: hcmDate(2026, 10, 15), HasEnd: true},
			WorkflowStepRuntimeStep{StepID: "s2", EndDate: hcmDate(2026, 10, 20), HasEnd: true},
		),
		"open": legacyRuntime(WorkflowStepRuntimeStep{StepID: "s1", Current: true, EndDate: hcmDate(2026, 10, 11), HasEnd: true}),
	}}
	svc := NewService(fakeConfigRepo{}, repo, &fakeAttemptRepo{},
		WithMilestoneScanner(scanner),
		WithWorkflowStepTaskStateReader(fakeStepTaskStateReader{}),
		WithWorkflowStepRuntimeReader(rt),
	)
	if _, err := svc.SeedOccurrencesFromDueMilestones(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.byKey[WorkflowStepReminderIdempotencyKey("open", "s1", "due_minus_1d")]; !ok {
		t.Fatal("eligible milestone hidden behind >200 waiting rows")
	}
	if len(scanner.obsolete) != 0 {
		t.Fatalf("waiting rows must not be obsoleted, got %d", len(scanner.obsolete))
	}
}

// PERF-54: more waiting rows than one tick can read (5 pages x 200) must not hide later rows
// forever; the scan resumes where the previous tick stopped and wraps around.
func TestSeedOccurrences_WaitRowsBeyondTickBudgetDoNotStarve(t *testing.T) {
	now := hcmDate(2026, 10, 10)
	waiting := milestoneSeedBatchLimit*milestoneSeedMaxPages + 300
	var due []DueMilestone
	for i := 0; i < waiting; i++ {
		due = append(due, DueMilestone{
			MilestoneID: fmt.Sprintf("wait-%05d", i), CompanyID: "c1", WorkflowInstanceID: "waiting",
			StepID: "s2", MilestoneType: "due_minus_1d", ScheduledDate: now.AddDate(0, 0, -1),
		})
	}
	due = append(due, DueMilestone{MilestoneID: "zz-live", CompanyID: "c1", WorkflowInstanceID: "open", StepID: "s1", MilestoneType: "due_minus_1d", ScheduledDate: now})
	scanner := newOrderedMilestoneScanner(due)
	repo := newMySQLLikeOccurrenceRepo()
	rt := &fakeRuntimeReader{byInstance: map[string]*WorkflowStepRuntime{
		"waiting": legacyRuntime(
			WorkflowStepRuntimeStep{StepID: "s1", Current: true, EndDate: hcmDate(2026, 10, 15), HasEnd: true},
			WorkflowStepRuntimeStep{StepID: "s2", EndDate: hcmDate(2026, 10, 20), HasEnd: true},
		),
		"open": legacyRuntime(WorkflowStepRuntimeStep{StepID: "s1", Current: true, EndDate: hcmDate(2026, 10, 11), HasEnd: true}),
	}}
	svc := NewService(fakeConfigRepo{}, repo, &fakeAttemptRepo{},
		WithMilestoneScanner(scanner),
		WithWorkflowStepTaskStateReader(fakeStepTaskStateReader{}),
		WithWorkflowStepRuntimeReader(rt),
	)
	key := WorkflowStepReminderIdempotencyKey("open", "s1", "due_minus_1d")
	for tick := 0; tick < 3; tick++ {
		if _, err := svc.SeedOccurrencesFromDueMilestones(context.Background(), now); err != nil {
			t.Fatal(err)
		}
		if _, ok := repo.byKey[key]; ok {
			return
		}
	}
	t.Fatalf("eligible milestone behind %d waiting rows never seeded in 3 ticks", waiting)
}

// PERF-54: after a partial tick the scan continues past the cursor and then wraps, so a row
// before the cursor that became eligible meanwhile is still reached.
func TestSeedOccurrences_SeedCursorWrapsAround(t *testing.T) {
	now := hcmDate(2026, 10, 10)
	due := []DueMilestone{{MilestoneID: "aa-early", CompanyID: "c1", WorkflowInstanceID: "early", StepID: "s2", MilestoneType: "due_minus_1d", ScheduledDate: now.AddDate(0, 0, -2)}}
	for i := 0; i < milestoneSeedBatchLimit*milestoneSeedMaxPages+50; i++ {
		due = append(due, DueMilestone{
			MilestoneID: fmt.Sprintf("wait-%05d", i), CompanyID: "c1", WorkflowInstanceID: "waiting",
			StepID: "s2", MilestoneType: "due_minus_1d", ScheduledDate: now.AddDate(0, 0, -1),
		})
	}
	scanner := newOrderedMilestoneScanner(due)
	repo := newMySQLLikeOccurrenceRepo()
	waiting := legacyRuntime(
		WorkflowStepRuntimeStep{StepID: "s1", Current: true, EndDate: hcmDate(2026, 10, 15), HasEnd: true},
		WorkflowStepRuntimeStep{StepID: "s2", EndDate: hcmDate(2026, 10, 20), HasEnd: true},
	)
	rt := &fakeRuntimeReader{byInstance: map[string]*WorkflowStepRuntime{"waiting": waiting, "early": waiting}}
	svc := NewService(fakeConfigRepo{}, repo, &fakeAttemptRepo{},
		WithMilestoneScanner(scanner),
		WithWorkflowStepTaskStateReader(fakeStepTaskStateReader{}),
		WithWorkflowStepRuntimeReader(rt),
	)
	key := WorkflowStepReminderIdempotencyKey("early", "s2", "due_minus_1d")
	if _, err := svc.SeedOccurrencesFromDueMilestones(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.byKey[key]; ok {
		t.Fatal("step 2 must wait while step 1 is current")
	}
	// Step 1 of the early instance completes; its step 2 becomes current.
	rt.byInstance["early"] = legacyRuntime(
		WorkflowStepRuntimeStep{StepID: "s1", Completed: true, EndDate: hcmDate(2026, 10, 9), HasEnd: true},
		WorkflowStepRuntimeStep{StepID: "s2", Current: true, EndDate: hcmDate(2026, 10, 11), HasEnd: true},
	)
	if _, err := svc.SeedOccurrencesFromDueMilestones(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.byKey[key]; !ok {
		t.Fatal("row before the cursor not reached after wrap-around")
	}
}
