package app

import (
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
