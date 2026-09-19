package workflowstepcomments

import "testing"

func TestRecordMutateAllowFreeze(t *testing.T) {
	allow := []string{"Draft", "draft", "PendingReview", "In Progress", "in progress"}
	for _, s := range allow {
		if !isRecordMutateAllowed(s) {
			t.Fatalf("expected allow %q", s)
		}
	}
	deny := []string{"published", "Completed", "done", "approved", "archived", "closed", "cancelled", ""}
	for _, s := range deny {
		if isRecordMutateAllowed(s) {
			t.Fatalf("expected deny %q", s)
		}
	}
	for _, s := range []string{"completed", "done", "Published"} {
		if !isRecordTerminalFrozen(s) {
			t.Fatalf("expected terminal %q", s)
		}
	}
}

func TestStepCreateAllow(t *testing.T) {
	for _, s := range []string{"current", "incomplete", "completed"} {
		if !isStepCreateAllowed(s) {
			t.Fatalf("allow %q", s)
		}
	}
	for _, s := range []string{"not_started", "overdue", "past_incomplete", "unknown", ""} {
		if isStepCreateAllowed(s) {
			t.Fatalf("deny %q", s)
		}
	}
}
