package workflowstepcomments

import "strings"

// Record mutate allow/freeze sets — Option B (case-insensitive trim).
func normalizeRecordStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func isRecordTerminalFrozen(status string) bool {
	switch normalizeRecordStatus(status) {
	case "completed", "done", "published":
		return true
	default:
		return false
	}
}

func isRecordMutateAllowed(status string) bool {
	switch normalizeRecordStatus(status) {
	case "draft", "pendingreview", "in progress":
		return true
	default:
		return false
	}
}

func isStepCreateAllowed(stepStatus string) bool {
	switch strings.ToLower(strings.TrimSpace(stepStatus)) {
	case "current", "incomplete", "completed":
		return true
	default:
		return false
	}
}
