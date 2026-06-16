package app

import (
	"encoding/json"
	"testing"

	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

func TestActiveDepartmentsFromRow_prefersExtractedDepartment(t *testing.T) {
	got := ActiveDepartmentsFromRow("focal_confirm", "Phòng CBTT", nil)
	if len(got) != 1 || got[0] != "Phòng CBTT" {
		t.Fatalf("got %v, want [Phòng CBTT]", got)
	}
}

func TestActiveDepartmentsFromRow_fallsBackToSnapshot(t *testing.T) {
	snapshot, err := json.Marshal([]workflowapp.StepSnapshot{
		{StepCode: "focal_confirm", Department: "Phòng CBTT"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := ActiveDepartmentsFromRow("focal_confirm", "", snapshot)
	if len(got) != 1 || got[0] != "Phòng CBTT" {
		t.Fatalf("got %v, want [Phòng CBTT]", got)
	}
}

func TestActiveDepartmentsFromSnapshot_optionC(t *testing.T) {
	snapshot, err := json.Marshal([]workflowapp.StepSnapshot{
		{StepCode: "legal_review", Department: "Phòng Pháp chế"},
		{StepCode: "focal_confirm", Department: "Phòng CBTT"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := ActiveDepartmentsFromSnapshot("focal_confirm", snapshot)
	if len(got) != 1 || got[0] != "Phòng CBTT" {
		t.Fatalf("got %v, want [Phòng CBTT]", got)
	}
}

func TestActiveDepartmentsFromSnapshot_emptyWhenNoMatch(t *testing.T) {
	snapshot, _ := json.Marshal([]workflowapp.StepSnapshot{
		{StepCode: "legal_review", Department: "Phòng Pháp chế"},
	})
	got := ActiveDepartmentsFromSnapshot("unknown", snapshot)
	if got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}
