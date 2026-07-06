package app

import (
	"encoding/json"
	"testing"

	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

func TestCurrentStepNameFromRow_prefersExtractedName(t *testing.T) {
	got := CurrentStepNameFromRow("focal_confirm", "Phòng ban lập hồ sơ", nil)
	if got != "Phòng ban lập hồ sơ" {
		t.Fatalf("got %q", got)
	}
}

func TestCurrentStepNameFromRow_fallsBackToSnapshot(t *testing.T) {
	snapshot, err := json.Marshal([]workflowapp.StepSnapshot{
		{StepCode: "focal_confirm", Stage: "Trưởng bộ phận phê duyệt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := CurrentStepNameFromRow("focal_confirm", "", snapshot)
	if got != "Trưởng bộ phận phê duyệt" {
		t.Fatalf("got %q", got)
	}
}

func TestCurrentStepNameFromSnapshot_emptyWhenNoMatch(t *testing.T) {
	snapshot, _ := json.Marshal([]workflowapp.StepSnapshot{
		{StepCode: "legal_review", Stage: "Phòng Pháp chế"},
	})
	got := CurrentStepNameFromSnapshot("unknown", snapshot)
	if got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestCurrentStepNameFromSnapshot_doesNotReturnStepCode(t *testing.T) {
	snapshot, _ := json.Marshal([]workflowapp.StepSnapshot{
		{StepCode: "focal_confirm", Stage: ""},
	})
	got := CurrentStepNameFromSnapshot("focal_confirm", snapshot)
	if got != "" {
		t.Fatalf("got %q, want empty (no raw code)", got)
	}
}
