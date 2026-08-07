package mysql

import (
	"strings"
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

func TestDecodeProposalWorkflowPayload_schemaV2(t *testing.T) {
	raw := `{"schema_version":2,"disclosure_type_id":"t1","frozen":false,"steps":[{"id":"ps1","order":1,"name":"A","processing_days":2,"department_id":"d1"}]}`
	steps, days, snap, err := decodeProposalWorkflowPayload(raw)
	if err != nil || days != nil || snap == nil || snap.SchemaVersion != 2 {
		t.Fatalf("snap=%#v days=%v err=%v", snap, days, err)
	}
	if len(steps) != 1 || steps[0].StepID != "ps1" {
		t.Fatalf("derived overrides %#v", steps)
	}
	p := adhocapp.ProposalDTO{Workflow: snap}
	out, err := marshalProposalWorkflowPayload(p, nil)
	if err != nil || !strings.Contains(out, `"schema_version":2`) {
		t.Fatalf("out=%s err=%v", out, err)
	}
}
