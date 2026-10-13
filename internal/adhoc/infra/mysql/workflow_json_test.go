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

func TestDecodeProposalWorkflowPayload_schemaV3InheritedContentRoundTrip(t *testing.T) {
	raw := `{"schema_version":3,"disclosure_type_id":"t1","frozen":true,` +
		`"content_capture":{"captured_at":"2026-10-20T03:00:00Z","workflow_source":"company_override","version_no":4,"missing_source_step_ids":["gone"]},` +
		`"steps":[{"id":"ps1","source_step_id":"tpl-1","order":1,"name":"A","processing_days":2,"department_id":"d1","assignee_membership_ids":["m1"],` +
		`"inherited":{"description":"<p>x</p>","description_format":"safe_html","documents":[{"doc_id":"doc-1","name":"N","required":true,"template_file_id":"wdt-1","template_file_name":"m.docx"}],` +
		`"reminder_config":{"enabled":true,"mode":"days_before","days_before":[5,2]}}}]}`
	_, _, snap, err := decodeProposalWorkflowPayload(raw)
	if err != nil || snap == nil || snap.ContentCapture == nil || snap.ContentCapture.VersionNo != 4 {
		t.Fatalf("snap=%#v err=%v", snap, err)
	}
	inh := snap.Steps[0].Inherited
	if inh == nil || inh.Documents[0].TemplateFileID != "wdt-1" || inh.ReminderConfig.DaysBefore[1] != 2 {
		t.Fatalf("inherited=%#v", inh)
	}
	out, err := marshalProposalWorkflowPayload(adhocapp.ProposalDTO{Workflow: snap}, nil)
	if err != nil || !strings.Contains(out, `"description_format":"safe_html","documents":[{"doc_id":"doc-1"`) || !strings.Contains(out, `"missing_source_step_ids":["gone"]`) {
		t.Fatalf("out=%s err=%v", out, err)
	}
}
