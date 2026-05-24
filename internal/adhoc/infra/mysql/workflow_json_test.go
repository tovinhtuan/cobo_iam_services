package mysql

import (
	"testing"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
)

func TestMarshalUnmarshalProposedWorkflow_legacyArray(t *testing.T) {
	raw, err := marshalProposedWorkflowJSON(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if raw != "[]" {
		t.Fatalf("got %q", raw)
	}
	steps, days, err := unmarshalProposedWorkflowJSON(`[{"step_id":"s1","processing_days":3}]`)
	if err != nil || days != nil || len(steps) != 1 {
		t.Fatalf("steps=%#v days=%#v err=%v", steps, days, err)
	}
}

func TestMarshalUnmarshalProposedWorkflow_embedDays(t *testing.T) {
	d := 20
	raw, err := marshalProposedWorkflowJSON([]adhocapp.WorkflowStepOverride{}, &d)
	if err != nil {
		t.Fatal(err)
	}
	steps, days, err := unmarshalProposedWorkflowJSON(raw)
	if err != nil || days == nil || *days != 20 || len(steps) != 0 {
		t.Fatalf("steps=%#v days=%#v err=%v", steps, days, err)
	}
}
