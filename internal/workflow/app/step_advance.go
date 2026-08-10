package app

import (
	"sort"
	"strings"
)

// OrderedSnapshotSteps returns a copy of snapshot sorted by display_order then step_id.
func OrderedSnapshotSteps(snapshot []StepSnapshot) []StepSnapshot {
	out := append([]StepSnapshot(nil), snapshot...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].DisplayOrder == out[j].DisplayOrder {
			return out[i].StepID < out[j].StepID
		}
		return out[i].DisplayOrder < out[j].DisplayOrder
	})
	return out
}

// SnapshotStepIdentity returns the durable step code used on workflow_tasks.step_code.
func SnapshotStepIdentity(step StepSnapshot) string {
	code := strings.TrimSpace(step.StepCode)
	if code == "" {
		code = strings.TrimSpace(step.StepID)
	}
	return code
}

// FindSnapshotStepIndex locates a step by task step_code against step_code or step_id.
func FindSnapshotStepIndex(snapshot []StepSnapshot, stepCode string) int {
	want := strings.TrimSpace(stepCode)
	if want == "" {
		return -1
	}
	ordered := OrderedSnapshotSteps(snapshot)
	for i, step := range ordered {
		if SnapshotStepIdentity(step) == want || strings.TrimSpace(step.StepID) == want {
			return i
		}
	}
	return -1
}

// NextSnapshotStep returns the step after the current task's proposal step in frozen snapshot order.
// ok=false means current is final (or current not found).
func NextSnapshotStep(snapshot []StepSnapshot, currentStepCode string) (StepSnapshot, bool) {
	ordered := OrderedSnapshotSteps(snapshot)
	idx := FindSnapshotStepIndex(ordered, currentStepCode)
	if idx < 0 || idx+1 >= len(ordered) {
		return StepSnapshot{}, false
	}
	return ordered[idx+1], true
}

// IsProposalSnapshotV2 reports whether the instance was materialized from frozen proposal workflow v2.
func IsProposalSnapshotV2(workflowSource string) bool {
	return strings.TrimSpace(workflowSource) == WorkflowSourceProposalSnapshotV2
}

// IsProposalSnapshotV3 reports whether the instance was materialized from frozen proposal workflow v3.
func IsProposalSnapshotV3(workflowSource string) bool {
	return strings.TrimSpace(workflowSource) == WorkflowSourceProposalSnapshotV3
}

// IsProposalSnapshotFrozen reports v2 or v3 frozen proposal materialization (multi-step advance path).
func IsProposalSnapshotFrozen(workflowSource string) bool {
	return IsProposalSnapshotV2(workflowSource) || IsProposalSnapshotV3(workflowSource)
}
