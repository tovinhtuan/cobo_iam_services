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

func TestComputeDeadlineSteps_Step1IncompletePastEnd_StaysCurrent(t *testing.T) {
	// Sequential policy: incomplete step1 remains current even when today falls in step2 window.
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
	if resp.CurrentStepCode != "s1" {
		t.Fatalf("expected current s1 (incomplete overdue), got %q", resp.CurrentStepCode)
	}
	s1 := resp.Steps[0]
	if s1.IsFuture {
		t.Fatal("step1 must not be future")
	}
	if s1.Status != "current" {
		t.Fatalf("step1 status=%s want current", s1.Status)
	}
	if len(s1.AvailableActions) == 0 || s1.AvailableActions[0] != "complete" {
		t.Fatalf("step1 actions=%v", s1.AvailableActions)
	}
	s2 := resp.Steps[1]
	if !s2.IsFuture || s2.Status != "not_started" || !s2.IsLocked {
		t.Fatalf("step2 must stay locked future, status=%s isFuture=%v locked=%v", s2.Status, s2.IsFuture, s2.IsLocked)
	}
	if len(s2.AvailableActions) != 0 {
		t.Fatal("step2 must not have actions while s1 incomplete")
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
	// Step1 still incomplete → current; step3 future.
	if resp.CurrentStepCode != "s1" {
		t.Fatalf("expected current s1, got %q", resp.CurrentStepCode)
	}
	if resp.Steps[2].Status != "not_started" || !resp.Steps[2].IsFuture {
		t.Fatalf("step3 should be future, status=%s isFuture=%v", resp.Steps[2].Status, resp.Steps[2].IsFuture)
	}
}

func hasAction(actions []string, want string) bool {
	for _, a := range actions {
		if a == want {
			return true
		}
	}
	return false
}

func TestComputeDeadlineSteps_Step1BeforePlannedStart_Locked(t *testing.T) {
	t0 := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	today := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC) // before T0/start
	ctx := WorkflowInstanceContext{
		RecordID: "rec-early",
		T0Date:   t0,
		Snapshot: []workflowapp.StepSnapshot{
			{StepID: "s1", Stage: "One", DisplayOrder: 1, ProcessingDays: 3},
			{StepID: "s2", Stage: "Two", DisplayOrder: 2, ProcessingDays: 5},
		},
		Timezone: "Asia/Ho_Chi_Minh",
	}
	resp, err := ComputeDeadlineSteps(ctx, nil, today, ctx.Timezone, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.CurrentStepCode != "" {
		t.Fatalf("expected empty current before start, got %q", resp.CurrentStepCode)
	}
	if resp.Steps[0].Status != "not_started" || !resp.Steps[0].IsFuture || !resp.Steps[0].IsLocked {
		t.Fatalf("step1 want locked not_started, got status=%s future=%v locked=%v",
			resp.Steps[0].Status, resp.Steps[0].IsFuture, resp.Steps[0].IsLocked)
	}
	if len(resp.Steps[0].AvailableActions) != 0 {
		t.Fatalf("step1 actions=%v", resp.Steps[0].AvailableActions)
	}
	if !resp.Steps[1].IsLocked || !resp.Steps[1].IsFuture {
		t.Fatal("step2 must stay locked")
	}
}

func TestComputeDeadlineSteps_Step1CompletedBeforeStep2Start_UnlocksStep2(t *testing.T) {
	// Core product case: complete s1 early → s2 current before its planned_start.
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	today := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	completed := time.Date(2026, 7, 2, 8, 0, 0, 0, time.UTC)
	ctx := WorkflowInstanceContext{
		RecordID: "rec-seq",
		T0Date:   t0,
		Snapshot: []workflowapp.StepSnapshot{
			{StepID: "s1", Stage: "One", DisplayOrder: 1, ProcessingDays: 3},
			{StepID: "s2", Stage: "Two", DisplayOrder: 2, ProcessingDays: 5},
			{StepID: "s3", Stage: "Three", DisplayOrder: 3, ProcessingDays: 3},
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
	s1Start := resp.Steps[0].PlannedStartDate
	s2Start := resp.Steps[1].PlannedStartDate
	s2End := resp.Steps[1].PlannedEndDate
	if resp.CurrentStepCode != "s2" {
		t.Fatalf("current_step_code=%q want s2", resp.CurrentStepCode)
	}
	s1 := resp.Steps[0]
	if !s1.IsCompleted || s1.Status != "completed" || !s1.IsLocked {
		t.Fatalf("s1=%+v", s1)
	}
	s2 := resp.Steps[1]
	if today.Format("2006-01-02") >= s2Start {
		t.Fatalf("fixture invalid: today should be before s2 start (%s)", s2Start)
	}
	if s2.Status != "current" {
		t.Fatalf("s2 status=%s want current", s2.Status)
	}
	if s2.IsFuture || s2.IsLocked {
		t.Fatalf("s2 must be actionable: future=%v locked=%v", s2.IsFuture, s2.IsLocked)
	}
	if !hasAction(s2.AvailableActions, "complete") {
		t.Fatalf("s2 actions=%v want complete", s2.AvailableActions)
	}
	// Early unlock: not yet in calendar window → is_current_by_time false; status still current.
	if s2.IsCurrentByTime {
		t.Fatal("early-unlocked s2 must not claim is_current_by_time")
	}
	if s2.PlannedStartDate != s2Start || s2.PlannedEndDate != s2End {
		t.Fatal("planned dates mutated")
	}
	if resp.Steps[0].PlannedStartDate != s1Start {
		t.Fatal("s1 planned dates mutated")
	}
	s3 := resp.Steps[2]
	if !s3.IsFuture || !s3.IsLocked || s3.Status != "not_started" {
		t.Fatalf("s3 must stay locked, status=%s", s3.Status)
	}
	if s2.TimelinessStatus != nil && *s2.TimelinessStatus == "OVERDUE" {
		t.Fatalf("early current must not be OVERDUE, got %v", s2.TimelinessStatus)
	}
}

func TestComputeDeadlineSteps_Step1NotCompleted_Step2Locked(t *testing.T) {
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	today := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	ctx := WorkflowInstanceContext{
		T0Date: t0,
		Snapshot: []workflowapp.StepSnapshot{
			{StepID: "s1", DisplayOrder: 1, ProcessingDays: 3},
			{StepID: "s2", DisplayOrder: 2, ProcessingDays: 5},
		},
		Timezone: "Asia/Ho_Chi_Minh",
	}
	resp, err := ComputeDeadlineSteps(ctx, nil, today, ctx.Timezone, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.CurrentStepCode != "s1" {
		t.Fatalf("current=%q", resp.CurrentStepCode)
	}
	if !hasAction(resp.Steps[0].AvailableActions, "complete") {
		t.Fatalf("s1 actions=%v", resp.Steps[0].AvailableActions)
	}
	if !resp.Steps[1].IsLocked || !resp.Steps[1].IsFuture {
		t.Fatal("s2 must be locked")
	}
}

func TestComputeDeadlineSteps_Step2Completed_UnlocksStep3Early(t *testing.T) {
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	today := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)
	c1 := time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)
	c2 := time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC)
	ctx := WorkflowInstanceContext{
		T0Date: t0,
		Snapshot: []workflowapp.StepSnapshot{
			{StepID: "s1", DisplayOrder: 1, ProcessingDays: 2},
			{StepID: "s2", DisplayOrder: 2, ProcessingDays: 5},
			{StepID: "s3", DisplayOrder: 3, ProcessingDays: 4},
		},
		Timezone: "Asia/Ho_Chi_Minh",
	}
	states := map[string]StepRuntimeState{
		"s1": {StepCode: "s1", CompletedAt: &c1},
		"s2": {StepCode: "s2", CompletedAt: &c2},
	}
	resp, err := ComputeDeadlineSteps(ctx, states, today, ctx.Timezone, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.CurrentStepCode != "s3" {
		t.Fatalf("current=%q want s3", resp.CurrentStepCode)
	}
	s3 := resp.Steps[2]
	if s3.Status != "current" || s3.IsFuture || s3.IsLocked {
		t.Fatalf("s3=%+v", s3)
	}
	if !hasAction(s3.AvailableActions, "complete") {
		t.Fatalf("s3 actions=%v", s3.AvailableActions)
	}
}

func TestComputeDeadlineSteps_MarkedIncomplete_BlocksSuccessor(t *testing.T) {
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	today := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	marked := today
	ctx := WorkflowInstanceContext{
		T0Date: t0,
		Snapshot: []workflowapp.StepSnapshot{
			{StepID: "s1", DisplayOrder: 1, ProcessingDays: 3},
			{StepID: "s2", DisplayOrder: 2, ProcessingDays: 5},
		},
		Timezone: "Asia/Ho_Chi_Minh",
	}
	states := map[string]StepRuntimeState{
		"s1": {StepCode: "s1", MarkedIncompleteAt: &marked, IncompleteReason: "need more"},
	}
	resp, err := ComputeDeadlineSteps(ctx, states, today, ctx.Timezone, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.CurrentStepCode != "s1" {
		t.Fatalf("current=%q", resp.CurrentStepCode)
	}
	if resp.Steps[0].Status != "incomplete" {
		t.Fatalf("s1 status=%s", resp.Steps[0].Status)
	}
	if !resp.Steps[1].IsLocked {
		t.Fatal("s2 must stay locked while s1 incomplete")
	}
}

func TestComputeDeadlineSteps_NoPermission_NoActions(t *testing.T) {
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	today := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	ctx := WorkflowInstanceContext{
		T0Date: t0,
		Snapshot: []workflowapp.StepSnapshot{
			{StepID: "s1", DisplayOrder: 1, ProcessingDays: 3},
			{StepID: "s2", DisplayOrder: 2, ProcessingDays: 5},
		},
		Timezone: "Asia/Ho_Chi_Minh",
	}
	resp, err := ComputeDeadlineSteps(ctx, nil, today, ctx.Timezone, false)
	if err != nil {
		t.Fatal(err)
	}
	if resp.CurrentStepCode != "s1" {
		t.Fatalf("current=%q", resp.CurrentStepCode)
	}
	if len(resp.Steps[0].AvailableActions) != 0 {
		t.Fatalf("no permission must yield empty actions, got %v", resp.Steps[0].AvailableActions)
	}
}
