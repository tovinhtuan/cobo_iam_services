package app

import (
	"context"
	"strings"
	"testing"
	"time"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
)

type fakeMilestoneRepository struct {
	inserted []StepMilestoneRow
}

func (f *fakeMilestoneRepository) InsertStepMilestones(_ context.Context, rows []StepMilestoneRow) error {
	f.inserted = append(f.inserted, rows...)
	return nil
}

func (f *fakeMilestoneRepository) ListByInstance(context.Context, string, string) ([]InstanceReminderDTO, error) {
	return nil, nil
}

func TestCreateWorkflowInstanceInternalSeedsMilestonesWhenTimelineEnabled(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	milestones := &fakeMilestoneRepository{}
	svc := NewService(repo, nil, fakeWorkflowIDGen{},
		WithMilestoneRepository(milestones),
		WithFlags(Flags{TimelineEnabled: true}),
	)
	t0 := time.Date(2026, time.June, 12, 0, 0, 0, 0, time.UTC)

	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject: Subject{
			UserID:       "user-001",
			MembershipID: "member-001",
			CompanyID:    "company-001",
		},
		RecordID: "record-001",
		T0Date:   &t0,
		T0Policy: "user_defined",
		Snapshot: []StepSnapshot{{
			StepID:       "step-review",
			StepCode:     "step-review",
			DueRule:      "T+1",
			DisplayOrder: 1,
		}},
	})
	if err != nil {
		t.Fatalf("CreateWorkflowInstanceInternal() error = %v", err)
	}
	if len(milestones.inserted) == 0 {
		t.Fatal("expected milestone rows to be seeded when timeline is enabled")
	}
	if milestones.inserted[0].InstanceID != repo.createdInstance.WorkflowInstanceID {
		t.Fatalf("expected milestone instance_id %q, got %q", repo.createdInstance.WorkflowInstanceID, milestones.inserted[0].InstanceID)
	}
}

func TestBuildMilestonesFromSnapshot_RuleIsolationAndEndDate(t *testing.T) {
	t0 := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	snapshot := []StepSnapshot{
		{StepID: "step-one", StepCode: "step-one", DueRule: "T+10", DisplayOrder: 1, ProcessingDays: 10, EffectiveReminderDays: []int{7, 2}},
		{StepID: "step-two", StepCode: "step-two", DueRule: "T+5", DisplayOrder: 2, ProcessingDays: 5, EffectiveReminderDays: []int{3, 1}},
		{StepID: "step-thr", StepCode: "step-thr", DueRule: "T+5", DisplayOrder: 3, ProcessingDays: 5, EffectiveReminderDays: []int{5}},
	}
	rows, err := buildMilestonesFromSnapshot("instance1", "company1", snapshot, t0, func() string { return "abcdefgh" })
	if err != nil {
		t.Fatal(err)
	}
	byStep := map[string][]string{}
	var dueMinus []StepMilestoneRow
	for _, row := range rows {
		byStep[row.StepID] = append(byStep[row.StepID], row.MilestoneType)
		if strings.HasPrefix(row.MilestoneType, "due_minus_") {
			dueMinus = append(dueMinus, row)
		}
		if strings.HasPrefix(row.MilestoneType, "before_start_") {
			t.Fatalf("runtime must not seed before_start_*: %s", row.MilestoneType)
		}
	}
	assertContainsExactly(t, dueMinusTypes(byStep["step-one"]), []string{"due_minus_7d", "due_minus_2d"})
	assertContainsExactly(t, dueMinusTypes(byStep["step-two"]), []string{"due_minus_3d", "due_minus_1d"})
	assertContainsExactly(t, dueMinusTypes(byStep["step-thr"]), []string{"due_minus_5d"})

	// Step1 EndDate = June 1 + 10 - 1 = June 10; -7 = June 3
	var step1minus7 time.Time
	for _, row := range dueMinus {
		if row.StepID == "step-one" && row.MilestoneType == "due_minus_7d" {
			step1minus7 = row.ScheduledDate
		}
	}
	want := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.FixedZone("Asia/Ho_Chi_Minh", 7*3600))
	if step1minus7.Year() != want.Year() || step1minus7.Month() != want.Month() || step1minus7.Day() != want.Day() {
		t.Fatalf("step1 -7 scheduled=%v want day %v", step1minus7, want)
	}
}

func TestBuildMilestonesFromSnapshot_ProposalDefaultFallback(t *testing.T) {
	t0 := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	snap := MapProposalWorkflowToSnapshot(&adhocapp.ProposalWorkflowSnapshot{
		SchemaVersion: 2,
		Frozen:        true,
		Steps: []adhocapp.ProposalWorkflowStep{
			{ID: "ps-a", Order: 1, Name: "A", ProcessingDays: 10, DepartmentID: "d1"},
		},
	})
	rows, err := buildMilestonesFromSnapshot("instance1", "company1", snap, t0, func() string { return "abcdefgh" })
	if err != nil {
		t.Fatal(err)
	}
	found3, found1 := false, false
	for _, row := range rows {
		if strings.HasPrefix(row.MilestoneType, "before_start_") {
			t.Fatalf("ad-hoc runtime must not seed before_start_*: %s", row.MilestoneType)
		}
		if row.MilestoneType == "due_minus_3d" {
			found3 = true
		}
		if row.MilestoneType == "due_minus_1d" {
			found1 = true
		}
	}
	if !found3 || !found1 {
		t.Fatalf("ad-hoc DEFAULT fallback missing due-minus, rows=%v", rows)
	}
}

func dueMinusTypes(types []string) []string {
	var out []string
	for _, t := range types {
		if strings.HasPrefix(t, "due_minus_") {
			out = append(out, t)
		}
	}
	return out
}

func assertContainsExactly(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
	set := map[string]int{}
	for _, g := range got {
		set[g]++
	}
	for _, w := range want {
		if set[w] != 1 {
			t.Fatalf("got=%v want=%v missing %s", got, want, w)
		}
	}
}
