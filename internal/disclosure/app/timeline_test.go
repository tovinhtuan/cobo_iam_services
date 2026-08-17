package app

import (
	"strings"
	"testing"
	"time"
)

func TestParseDueRuleProcessingDays(t *testing.T) {
	if got := parseDueRuleProcessingDays("T+3"); got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
	if got := parseDueRuleProcessingDays(""); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
}

func TestComputeStepTimelinesUsesDueRule(t *testing.T) {
	t0 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	steps := []WorkflowStepDTO{
		{StepID: "s1", DueRule: "T+1", DisplayOrder: 1},
		{StepID: "s2", DueRule: "T+2", DisplayOrder: 2},
	}
	timelines, err := ComputeStepTimelines(t0, "UTC", steps, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(timelines) != 2 {
		t.Fatalf("expected 2 timelines, got %d", len(timelines))
	}
	if timelines[0].ProcessingDays != 1 || timelines[1].ProcessingDays != 2 {
		t.Fatalf("unexpected processing days: %+v", timelines)
	}
}

func TestGenerateDueMinusReminderCandidates_UsesEndDateNotStartDate(t *testing.T) {
	loc := time.UTC
	t0 := time.Date(2026, 6, 1, 0, 0, 0, 0, loc)
	tl := StepTimeline{
		StepID: "step-review", StepOrder: 1,
		StartDate:      time.Date(2026, 6, 10, 0, 0, 0, 0, loc),
		EndDate:        time.Date(2026, 6, 20, 0, 0, 0, 0, loc),
		ProcessingDays: 11,
	}
	rows := GenerateDueMinusReminderCandidates(tl, t0, "co", "instance1", []int{7, 2}, func() string { return "abcdefgh" })
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	if rows[0].MilestoneType != DueMinusMilestoneType(7) || rows[1].MilestoneType != DueMinusMilestoneType(2) {
		t.Fatalf("types=%s %s", rows[0].MilestoneType, rows[1].MilestoneType)
	}
	want7 := time.Date(2026, 6, 13, 0, 0, 0, 0, loc)
	want2 := time.Date(2026, 6, 18, 0, 0, 0, 0, loc)
	if !rows[0].ScheduledDate.Equal(want7) || !rows[1].ScheduledDate.Equal(want2) {
		t.Fatalf("scheduled=%v %v want %v %v", rows[0].ScheduledDate, rows[1].ScheduledDate, want7, want2)
	}
	for _, row := range rows {
		if row.ScheduledDate.Equal(tl.StartDate.AddDate(0, 0, -7)) {
			t.Fatal("must not schedule from StartDate")
		}
	}
}

func TestGenerateTimelineMilestoneCandidates_PreservesStartAndEnd(t *testing.T) {
	loc := time.UTC
	t0 := time.Date(2026, 6, 1, 0, 0, 0, 0, loc)
	tl := StepTimeline{
		StepID: "step-review", StepOrder: 1,
		StartDate: time.Date(2026, 6, 10, 0, 0, 0, 0, loc),
		EndDate:   time.Date(2026, 6, 12, 0, 0, 0, 0, loc),
	}
	rows := GenerateTimelineMilestoneCandidates(tl, t0, "co", "instance1", func() string { return "abcdefgh" })
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	if rows[0].MilestoneType != MilestoneStepStart || rows[1].MilestoneType != MilestoneStepEnd {
		t.Fatalf("types=%s %s", rows[0].MilestoneType, rows[1].MilestoneType)
	}
}

func TestGenerateDueMinusReminderCandidates_PersistsRequiredOffsetsIncluding90d(t *testing.T) {
	loc := time.UTC
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, loc)
	tl := StepTimeline{
		StepID: "step-review", StepOrder: 1,
		StartDate:      time.Date(2026, 6, 1, 0, 0, 0, 0, loc),
		EndDate:        time.Date(2026, 8, 1, 0, 0, 0, 0, loc),
		ProcessingDays: 62,
	}
	offsets := []int{1, 2, 3, 5, 7, 90}
	rows := GenerateDueMinusReminderCandidates(tl, t0, "co", "instance1xxxxxxxx", offsets, func() string { return "abcdefgh" })
	if len(rows) != len(offsets) {
		t.Fatalf("rows=%d want %d", len(rows), len(offsets))
	}
	for i, offset := range offsets {
		want := string(DueMinusMilestoneType(offset))
		if string(rows[i].MilestoneType) != want {
			t.Fatalf("offset=%d type=%s want %s", offset, rows[i].MilestoneType, want)
		}
		recovered, ok := RecoverDueMinusMilestoneTypeFromID(rows[i].MilestoneID)
		if !ok || recovered != want {
			t.Fatalf("milestone_id %q did not embed %s", rows[i].MilestoneID, want)
		}
	}
}

func TestGenerateTimelineAndLegacyMilestoneTypesRemainNamed(t *testing.T) {
	loc := time.UTC
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, loc)
	tl := StepTimeline{
		StepID: "step-review", StepOrder: 1,
		StartDate: time.Date(2026, 6, 10, 0, 0, 0, 0, loc),
		EndDate:   time.Date(2026, 6, 20, 0, 0, 0, 0, loc),
	}
	legacy := GenerateMilestoneCandidates(tl, t0, "co", "instance1xxxxxxxx", func() string { return "abcdefgh" })
	wantLegacy := map[MilestoneType]bool{
		MilestoneBefore5d: false, MilestoneBefore3d: false, MilestoneBefore1d: false,
		MilestoneStepStart: false, MilestoneStepEnd: false,
	}
	for _, row := range legacy {
		if _, ok := wantLegacy[row.MilestoneType]; !ok {
			t.Fatalf("unexpected legacy type %s", row.MilestoneType)
		}
		wantLegacy[row.MilestoneType] = true
	}
	for mtype, seen := range wantLegacy {
		if !seen {
			t.Fatalf("missing legacy type %s", mtype)
		}
	}
}

func TestGenerateConfiguredReminderPreviewCandidates_UsesDueMinusNotBeforeStart(t *testing.T) {
	loc := time.UTC
	t0 := time.Date(2026, 6, 1, 0, 0, 0, 0, loc)
	tl := StepTimeline{
		StepID: "step-review", StepOrder: 1,
		StartDate: time.Date(2026, 6, 10, 0, 0, 0, 0, loc),
		EndDate:   time.Date(2026, 6, 20, 0, 0, 0, 0, loc),
	}
	custom := &WorkflowStepReminderConfig{Enabled: true, Mode: WorkflowStepReminderModeDaysBefore, DaysBefore: []int{7, 2}}
	rows := GenerateConfiguredReminderPreviewCandidates(tl, t0, "co", "preview-1", custom, func() string { return "abcdefgh" })
	foundDueMinus := false
	for _, row := range rows {
		if strings.HasPrefix(string(row.MilestoneType), "before_start_") {
			t.Fatalf("preview must not emit before_start_*: %s", row.MilestoneType)
		}
		if row.MilestoneType == DueMinusMilestoneType(7) {
			foundDueMinus = true
		}
	}
	if !foundDueMinus {
		t.Fatal("custom preview must include due_minus_7d")
	}

	absent := GenerateConfiguredReminderPreviewCandidates(tl, t0, "co", "preview-1", nil, func() string { return "abcdefgh" })
	found3, found1 := false, false
	for _, row := range absent {
		if row.MilestoneType == DueMinusMilestoneType(3) {
			found3 = true
		}
		if row.MilestoneType == DueMinusMilestoneType(1) {
			found1 = true
		}
	}
	if !found3 || !found1 {
		t.Fatal("absent reminder_config preview must use DEFAULT [3,1]")
	}
}

func TestGenerateMilestoneCandidates_LegacyHelperStillHasBeforeStart(t *testing.T) {
	loc := time.UTC
	t0 := time.Date(2026, 6, 1, 0, 0, 0, 0, loc)
	tl := StepTimeline{
		StepID: "step-review", StepOrder: 1,
		StartDate: time.Date(2026, 6, 10, 0, 0, 0, 0, loc),
		EndDate:   time.Date(2026, 6, 12, 0, 0, 0, 0, loc),
	}
	rows := GenerateMilestoneCandidates(tl, t0, "co", "instance1", func() string { return "abcdefgh" })
	foundBefore := false
	for _, row := range rows {
		if row.MilestoneType == MilestoneBefore3d {
			foundBefore = true
		}
	}
	if !foundBefore {
		t.Fatal("legacy helper must still produce before_start_*")
	}
}
