package app

import (
	"testing"
	"time"

	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

func hcmLoc(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	}
	return loc
}

func hcmDay(t *testing.T, y int, m time.Month, d int) time.Time {
	t.Helper()
	loc := hcmLoc(t)
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

func hcmAt(t *testing.T, y int, m time.Month, d, hour, min int) time.Time {
	t.Helper()
	loc := hcmLoc(t)
	return time.Date(y, m, d, hour, min, 0, 0, loc)
}

func mustStatus(t *testing.T, got *TimelinessStatus, want TimelinessStatus) {
	t.Helper()
	if got == nil {
		t.Fatalf("expected %s, got nil", want)
	}
	if *got != want {
		t.Fatalf("expected %s, got %s", want, *got)
	}
}

func TestResolveWorkflowTimeliness_WithinDeadlineInclusive(t *testing.T) {
	got := ResolveWorkflowTimeliness(WorkflowTimelinessInput{
		FinalStepExists:    true,
		FinalStepCompleted: false,
		FinalPlannedEnd:    "2026-09-11",
		TodayHCM:           hcmDay(t, 2026, 9, 11),
		Location:           hcmLoc(t),
	})
	mustStatus(t, got, TimelinessWithinDeadline)
}

func TestResolveWorkflowTimeliness_Overdue(t *testing.T) {
	got := ResolveWorkflowTimeliness(WorkflowTimelinessInput{
		FinalStepExists:    true,
		FinalStepCompleted: false,
		FinalPlannedEnd:    "2026-09-11",
		TodayHCM:           hcmDay(t, 2026, 9, 12),
		Location:           hcmLoc(t),
	})
	mustStatus(t, got, TimelinessOverdue)
}

func TestResolveWorkflowTimeliness_OnTimeInclusive(t *testing.T) {
	completed := hcmAt(t, 2026, 9, 11, 23, 30)
	got := ResolveWorkflowTimeliness(WorkflowTimelinessInput{
		FinalStepExists:    true,
		FinalStepCompleted: true,
		FinalPlannedEnd:    "2026-09-11",
		FinalCompletedAt:   &completed,
		TodayHCM:           hcmDay(t, 2026, 9, 15),
		Location:           hcmLoc(t),
	})
	mustStatus(t, got, TimelinessOnTime)
}

func TestResolveWorkflowTimeliness_CompletedLate(t *testing.T) {
	completed := hcmAt(t, 2026, 9, 12, 1, 0)
	got := ResolveWorkflowTimeliness(WorkflowTimelinessInput{
		FinalStepExists:    true,
		FinalStepCompleted: true,
		FinalPlannedEnd:    "2026-09-11",
		FinalCompletedAt:   &completed,
		TodayHCM:           hcmDay(t, 2026, 9, 15),
		Location:           hcmLoc(t),
	})
	mustStatus(t, got, TimelinessCompletedLate)
}

func TestResolveWorkflowTimeliness_MarkIncompleteDelayNotAutomaticLate(t *testing.T) {
	// delay_days=3 adjusted planned_end=14/09; completed 13/09 → ON_TIME
	// (is_delayed / delay_days are intentionally not inputs)
	completed := hcmDay(t, 2026, 9, 13)
	got := ResolveWorkflowTimeliness(WorkflowTimelinessInput{
		FinalStepExists:    true,
		FinalStepCompleted: true,
		FinalPlannedEnd:    "2026-09-14",
		FinalCompletedAt:   &completed,
		TodayHCM:           hcmDay(t, 2026, 9, 15),
		Location:           hcmLoc(t),
	})
	mustStatus(t, got, TimelinessOnTime)
}

func TestResolveWorkflowTimeliness_DelayAdjustedDeadline(t *testing.T) {
	// original 11/09 shifted to 14/09; completed 13/09 → ON_TIME
	completed := hcmDay(t, 2026, 9, 13)
	got := ResolveWorkflowTimeliness(WorkflowTimelinessInput{
		FinalStepExists:    true,
		FinalStepCompleted: true,
		FinalPlannedEnd:    "2026-09-14",
		FinalCompletedAt:   &completed,
		TodayHCM:           hcmDay(t, 2026, 9, 20),
		Location:           hcmLoc(t),
	})
	mustStatus(t, got, TimelinessOnTime)
}

func TestResolveWorkflowTimeliness_DueAtIrrelevant_OnTime(t *testing.T) {
	// disclosure DueAt=10/09 is NOT an input; workflow deadline 15/09, completed 14/09 → ON_TIME
	completed := hcmDay(t, 2026, 9, 14)
	got := ResolveWorkflowTimeliness(WorkflowTimelinessInput{
		FinalStepExists:    true,
		FinalStepCompleted: true,
		FinalPlannedEnd:    "2026-09-15",
		FinalCompletedAt:   &completed,
		TodayHCM:           hcmDay(t, 2026, 9, 20),
		Location:           hcmLoc(t),
	})
	mustStatus(t, got, TimelinessOnTime)
}

func TestResolveWorkflowTimeliness_CompletedLateBeforeDisclosureDue(t *testing.T) {
	// workflow planned end 10/09, completed 11/09, disclosure DueAt 20/09 (not used) → COMPLETED_LATE
	completed := hcmDay(t, 2026, 9, 11)
	got := ResolveWorkflowTimeliness(WorkflowTimelinessInput{
		FinalStepExists:    true,
		FinalStepCompleted: true,
		FinalPlannedEnd:    "2026-09-10",
		FinalCompletedAt:   &completed,
		TodayHCM:           hcmDay(t, 2026, 9, 12),
		Location:           hcmLoc(t),
	})
	mustStatus(t, got, TimelinessCompletedLate)
}

func TestResolveWorkflowTimeliness_MissingCompletedAt(t *testing.T) {
	got := ResolveWorkflowTimeliness(WorkflowTimelinessInput{
		FinalStepExists:    true,
		FinalStepCompleted: true,
		FinalPlannedEnd:    "2026-09-11",
		FinalCompletedAt:   nil,
		TodayHCM:           hcmDay(t, 2026, 9, 11),
		Location:           hcmLoc(t),
	})
	if got != nil {
		t.Fatalf("expected nil, got %s", *got)
	}
}

func TestResolveWorkflowTimeliness_MissingPlannedEnd(t *testing.T) {
	got := ResolveWorkflowTimeliness(WorkflowTimelinessInput{
		FinalStepExists:    true,
		FinalStepCompleted: false,
		FinalPlannedEnd:    "",
		TodayHCM:           hcmDay(t, 2026, 9, 11),
		Location:           hcmLoc(t),
	})
	if got != nil {
		t.Fatalf("expected nil, got %s", *got)
	}
}

func TestResolveWorkflowTimeliness_NoFinalStep(t *testing.T) {
	got := ResolveWorkflowTimeliness(WorkflowTimelinessInput{
		FinalStepExists: false,
		TodayHCM:        hcmDay(t, 2026, 9, 11),
		Location:        hcmLoc(t),
	})
	if got != nil {
		t.Fatalf("expected nil, got %s", *got)
	}
}

func TestResolveStepTimeliness_WithinDeadline(t *testing.T) {
	got := ResolveStepTimeliness(StepTimelinessInput{
		IsFuture:    false,
		IsCompleted: false,
		PlannedEnd:  "2026-09-11",
		TodayHCM:    hcmDay(t, 2026, 9, 11),
		Location:    hcmLoc(t),
	})
	mustStatus(t, got, TimelinessWithinDeadline)
}

func TestResolveStepTimeliness_Overdue(t *testing.T) {
	got := ResolveStepTimeliness(StepTimelinessInput{
		IsFuture:    false,
		IsCompleted: false,
		PlannedEnd:  "2026-09-11",
		TodayHCM:    hcmDay(t, 2026, 9, 12),
		Location:    hcmLoc(t),
	})
	mustStatus(t, got, TimelinessOverdue)
}

func TestResolveStepTimeliness_OnTime(t *testing.T) {
	completed := hcmDay(t, 2026, 9, 11)
	got := ResolveStepTimeliness(StepTimelinessInput{
		IsFuture:    false,
		IsCompleted: true,
		PlannedEnd:  "2026-09-11",
		CompletedAt: &completed,
		TodayHCM:    hcmDay(t, 2026, 9, 15),
		Location:    hcmLoc(t),
	})
	mustStatus(t, got, TimelinessOnTime)
}

func TestResolveStepTimeliness_CompletedLate(t *testing.T) {
	completed := hcmDay(t, 2026, 9, 12)
	got := ResolveStepTimeliness(StepTimelinessInput{
		IsFuture:    false,
		IsCompleted: true,
		PlannedEnd:  "2026-09-11",
		CompletedAt: &completed,
		TodayHCM:    hcmDay(t, 2026, 9, 15),
		Location:    hcmLoc(t),
	})
	mustStatus(t, got, TimelinessCompletedLate)
}

func TestResolveStepTimeliness_FutureNull(t *testing.T) {
	got := ResolveStepTimeliness(StepTimelinessInput{
		IsFuture:    true,
		IsCompleted: false,
		PlannedEnd:  "2026-09-20",
		TodayHCM:    hcmDay(t, 2026, 9, 11),
		Location:    hcmLoc(t),
	})
	if got != nil {
		t.Fatalf("expected nil for future step, got %s", *got)
	}
}

func TestResolveStepTimeliness_MissingCompletedAt(t *testing.T) {
	got := ResolveStepTimeliness(StepTimelinessInput{
		IsFuture:    false,
		IsCompleted: true,
		PlannedEnd:  "2026-09-11",
		CompletedAt: nil,
		TodayHCM:    hcmDay(t, 2026, 9, 11),
		Location:    hcmLoc(t),
	})
	if got != nil {
		t.Fatalf("expected nil, got %s", *got)
	}
}

func TestResolveStepTimeliness_MissingPlannedEnd(t *testing.T) {
	got := ResolveStepTimeliness(StepTimelinessInput{
		IsFuture:    false,
		IsCompleted: false,
		PlannedEnd:  "",
		TodayHCM:    hcmDay(t, 2026, 9, 11),
		Location:    hcmLoc(t),
	})
	if got != nil {
		t.Fatalf("expected nil, got %s", *got)
	}
}

func TestComputeDeadlineSteps_AttachesTimelinessFields(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	today := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	ctx := WorkflowInstanceContext{
		RecordID: "rec-timeliness",
		T0Date:   t0,
		Snapshot: []workflowapp.StepSnapshot{
			{StepID: "s1", Stage: "One", DisplayOrder: 1, ProcessingDays: 2},
			{StepID: "s2", Stage: "Two", DisplayOrder: 2, ProcessingDays: 2},
		},
		Timezone: "Asia/Ho_Chi_Minh",
	}
	resp, err := ComputeDeadlineSteps(ctx, nil, today, ctx.Timezone, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.TimelinessStatus == nil || *resp.TimelinessStatus != string(TimelinessWithinDeadline) {
		t.Fatalf("workflow timeliness=%v", resp.TimelinessStatus)
	}
	if resp.TimelinessDeadline == nil || *resp.TimelinessDeadline == "" {
		t.Fatal("expected timeliness_deadline from final planned_end")
	}
	if resp.Steps[0].TimelinessStatus == nil || *resp.Steps[0].TimelinessStatus != string(TimelinessWithinDeadline) {
		t.Fatalf("step1 timeliness=%v", resp.Steps[0].TimelinessStatus)
	}
	if resp.Steps[1].IsFuture && resp.Steps[1].TimelinessStatus != nil {
		t.Fatalf("future step must have null timeliness, got %v", resp.Steps[1].TimelinessStatus)
	}
}

func TestComputeDeadlineSteps_DelayAdjustedFinalDeadlineOnTime(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	today := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	completed := time.Date(2026, 9, 13, 10, 0, 0, 0, hcmLoc(t))
	ctx := WorkflowInstanceContext{
		RecordID: "rec-delay-ontime",
		T0Date:   t0,
		Snapshot: []workflowapp.StepSnapshot{
			{StepID: "s1", Stage: "One", DisplayOrder: 1, ProcessingDays: 5},
			{StepID: "s2", Stage: "Two", DisplayOrder: 2, ProcessingDays: 5},
		},
		Timezone: "Asia/Ho_Chi_Minh",
	}
	states := map[string]StepRuntimeState{
		"s1": {
			StepCode:           "s1",
			DelayDaysApplied:   3,
			MarkedIncompleteAt: &t0,
			CompletedAt:        &completed,
		},
		"s2": {
			StepCode:    "s2",
			CompletedAt: &completed,
		},
	}
	resp, err := ComputeDeadlineSteps(ctx, states, today, ctx.Timezone, false)
	if err != nil {
		t.Fatal(err)
	}
	if resp.TimelinessStatus == nil || *resp.TimelinessStatus != string(TimelinessOnTime) {
		t.Fatalf("expected ON_TIME with adjusted deadline, got %v deadline=%v completed=%v",
			resp.TimelinessStatus, resp.TimelinessDeadline, resp.TimelinessCompletedAt)
	}
	if resp.Steps[0].IsDelayed != true || resp.Steps[0].DelayDays != 3 {
		t.Fatal("delay flags must still be present but not force COMPLETED_LATE")
	}
}
