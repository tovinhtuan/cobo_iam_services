package app

import (
	"testing"
	"time"

	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

func TestComputeDeadlineSteps_CurrentByTimeWindow(t *testing.T) {
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	today := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	ctx := WorkflowInstanceContext{
		RecordID: "rec-1",
		T0Date:   t0,
		Snapshot: []workflowapp.StepSnapshot{
			{StepID: "s1", Stage: "Bước một", DisplayOrder: 1, ProcessingDays: 3},
			{StepID: "s2", Stage: "Bước hai", DisplayOrder: 2, ProcessingDays: 5},
		},
		Timezone: "Asia/Ho_Chi_Minh",
	}
	resp, err := ComputeDeadlineSteps(ctx, nil, today, ctx.Timezone, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(resp.Steps))
	}
	if resp.CurrentStepCode != "s1" {
		t.Fatalf("expected current s1, got %q", resp.CurrentStepCode)
	}
	if resp.Steps[0].Status != "current" {
		t.Fatalf("step1 status=%s", resp.Steps[0].Status)
	}
	if !resp.Steps[0].IsCurrentByTime {
		t.Fatal("step1 should be current by time")
	}
	if resp.Steps[1].IsFuture != true {
		t.Fatalf("step2 should be future start=%s today=%s isFuture=%v status=%s",
			resp.Steps[1].PlannedStartDate, today.Format("2006-01-02"), resp.Steps[1].IsFuture, resp.Steps[1].Status)
	}
	if len(resp.Steps[1].AvailableActions) != 0 {
		t.Fatal("future step must not have actions")
	}
}

func TestComputeDeadlineSteps_CompletedLocked(t *testing.T) {
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	today := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	completed := today.Add(-24 * time.Hour)
	ctx := WorkflowInstanceContext{
		RecordID: "rec-1",
		T0Date:   t0,
		Snapshot: []workflowapp.StepSnapshot{
			{StepID: "s1", Stage: "Bước một", DisplayOrder: 1, ProcessingDays: 3},
			{StepID: "s2", Stage: "Bước hai", DisplayOrder: 2, ProcessingDays: 5},
		},
		Timezone: "Asia/Ho_Chi_Minh",
	}
	states := map[string]StepRuntimeState{
		"s1": {StepCode: "s1", CompletedAt: &completed},
	}
	resp, err := ComputeDeadlineSteps(ctx, states, today, ctx.Timezone, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Steps[0].IsCompleted != true || !resp.Steps[0].IsLocked {
		t.Fatal("completed step must be locked")
	}
	if resp.CurrentStepCode != "s2" {
		t.Fatalf("expected current s2, got %q", resp.CurrentStepCode)
	}
}

func TestApplyDelayShiftsSubsequentSteps(t *testing.T) {
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	ctx := WorkflowInstanceContext{
		T0Date: t0,
		Snapshot: []workflowapp.StepSnapshot{
			{StepID: "s1", DisplayOrder: 1, ProcessingDays: 2},
			{StepID: "s2", DisplayOrder: 2, ProcessingDays: 2},
		},
		Timezone: "Asia/Ho_Chi_Minh",
	}
	states := map[string]StepRuntimeState{
		"s1": {StepCode: "s1", DelayDaysApplied: 3, MarkedIncompleteAt: &t0},
	}
	resp, err := ComputeDeadlineSteps(ctx, states, t0, ctx.Timezone, false)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Steps[1].PlannedStartDate != "2026-07-06" {
		t.Fatalf("step2 start shifted, got %s", resp.Steps[1].PlannedStartDate)
	}
}

func TestComputeDeadlineSteps_Step2Current_Step1NotFuture(t *testing.T) {
	// Mirrors DEV case: step1 Jul 1-3, step2 Jul 4-Aug 14, today in step2 window.
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	today := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	ctx := WorkflowInstanceContext{
		RecordID: "rec-dev",
		T0Date:   t0,
		Snapshot: []workflowapp.StepSnapshot{
			{StepID: "s1", Stage: "REBASE-TEST CMS V3 Stage", DisplayOrder: 1, ProcessingDays: 3},
			{StepID: "s2", Stage: "SmokeQA Stage", DisplayOrder: 2, ProcessingDays: 42},
		},
		Timezone: "Asia/Ho_Chi_Minh",
	}
	resp, err := ComputeDeadlineSteps(ctx, nil, today, ctx.Timezone, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.CurrentStepCode != "s2" {
		t.Fatalf("expected current s2, got %q", resp.CurrentStepCode)
	}
	s1 := resp.Steps[0]
	if s1.IsFuture {
		t.Fatal("step1 must not be future when step2 is current")
	}
	if s1.Status == "not_started" {
		t.Fatalf("step1 status must not be not_started, got %q", s1.Status)
	}
	if s1.Status != "overdue" && s1.Status != "past_incomplete" && s1.Status != "completed" {
		t.Fatalf("step1 expected overdue/past_incomplete/completed, got %q", s1.Status)
	}
	if resp.Steps[1].Status != "current" {
		t.Fatalf("step2 status=%s", resp.Steps[1].Status)
	}
}

func TestComputeDeadlineSteps_Step2Current_Step1Completed(t *testing.T) {
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	today := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	completed := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	ctx := WorkflowInstanceContext{
		T0Date: t0,
		Snapshot: []workflowapp.StepSnapshot{
			{StepID: "s1", DisplayOrder: 1, ProcessingDays: 3},
			{StepID: "s2", DisplayOrder: 2, ProcessingDays: 42},
		},
		Timezone: "Asia/Ho_Chi_Minh",
	}
	states := map[string]StepRuntimeState{
		"s1": {StepCode: "s1", CompletedAt: &completed},
	}
	resp, err := ComputeDeadlineSteps(ctx, states, today, ctx.Timezone, false)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Steps[0].Status != "completed" {
		t.Fatalf("step1 status=%s", resp.Steps[0].Status)
	}
	if resp.Steps[0].IsFuture {
		t.Fatal("completed step1 must not be future")
	}
}

func TestComputeDeadlineSteps_Step3FutureWhenStep2Current(t *testing.T) {
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	today := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	ctx := WorkflowInstanceContext{
		T0Date: t0,
		Snapshot: []workflowapp.StepSnapshot{
			{StepID: "s1", DisplayOrder: 1, ProcessingDays: 3},
			{StepID: "s2", DisplayOrder: 2, ProcessingDays: 5},
			{StepID: "s3", DisplayOrder: 3, ProcessingDays: 3},
		},
		Timezone: "Asia/Ho_Chi_Minh",
	}
	resp, err := ComputeDeadlineSteps(ctx, nil, today, ctx.Timezone, false)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Steps[2].Status != "not_started" || !resp.Steps[2].IsFuture {
		t.Fatalf("step3 should be future, status=%s isFuture=%v", resp.Steps[2].Status, resp.Steps[2].IsFuture)
	}
}
