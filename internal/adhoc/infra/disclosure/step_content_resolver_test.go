package disclosure

import (
	"context"
	"errors"
	"testing"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

type stubEffectiveWorkflowReader struct {
	req  disclosureapp.GetEffectiveWorkflowRequest
	resp *disclosureapp.GetEffectiveWorkflowResponse
	err  error
}

func (s *stubEffectiveWorkflowReader) GetEffectiveWorkflow(_ context.Context, req disclosureapp.GetEffectiveWorkflowRequest) (*disclosureapp.GetEffectiveWorkflowResponse, error) {
	s.req = req
	return s.resp, s.err
}

func TestStepContentResolverAdapter_MapsEffectiveWorkflowOfOwnCompany(t *testing.T) {
	reader := &stubEffectiveWorkflowReader{resp: &disclosureapp.GetEffectiveWorkflowResponse{Data: disclosureapp.EffectiveWorkflowDTO{
		Source: "company_override", VersionNo: 7,
		Workflow: []disclosureapp.WorkflowStepDTO{
			{
				StepID: " tpl-1 ", Description: " <p>Mô tả</p> ", DescriptionFormat: "safe_html", Instructions: "",
				Documents: []disclosureapp.WorkflowDocumentDTO{
					{DocID: "doc-1", Name: "Nghị quyết", Required: true, TemplateFileID: "wdt-1", TemplateFileName: "mau.docx"},
					{DocID: "doc-2", Name: "Phụ lục", TemplateFileName: "no-id.docx"},
					{DocID: " ", Name: " "},
				},
				ReminderConfig: &disclosureapp.WorkflowStepReminderConfig{Enabled: true, Mode: "days_before", DaysBefore: []int{5, 2}},
			},
			{StepID: "tpl-1", Description: "duplicate id ignored"},
			{StepID: "tpl-2", DescriptionFormat: "safe_html"},
			{StepID: ""},
		},
	}}}
	a := &StepContentResolverAdapter{svc: reader}
	got, err := a.ResolveStepContent(context.Background(), adhocapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "co-1"}, "type-1")
	if err != nil {
		t.Fatal(err)
	}
	if reader.req.Subject.CompanyID != "co-1" || reader.req.Subject.MembershipID != "m1" || reader.req.Subject.UserID != "u1" || reader.req.TypeID != "type-1" || !reader.req.RuntimeOnly {
		t.Fatalf("request=%#v", reader.req)
	}
	if got.WorkflowSource != "company_override" || got.VersionNo != 7 || len(got.BySourceStepID) != 2 {
		t.Fatalf("catalog=%#v", got)
	}
	c1 := got.BySourceStepID["tpl-1"]
	if c1.Description != "<p>Mô tả</p>" || c1.DescriptionFormat != "safe_html" || len(c1.Documents) != 2 ||
		c1.Documents[1].TemplateFileName != "" || c1.ReminderConfig == nil || len(c1.ReminderConfig.DaysBefore) != 2 {
		t.Fatalf("tpl-1=%#v", c1)
	}
	reader.resp.Data.Workflow[0].ReminderConfig.DaysBefore[0] = 9
	if c1.ReminderConfig.DaysBefore[0] != 5 {
		t.Fatal("reminder days must be copied")
	}
	if c2 := got.BySourceStepID["tpl-2"]; c2.DescriptionFormat != "" || c2.ReminderConfig != nil || c2.Documents != nil {
		t.Fatalf("tpl-2=%#v", c2)
	}
}

func TestStepContentResolverAdapter_PassesErrorsThrough(t *testing.T) {
	want := errors.New("db down")
	_, err := (&StepContentResolverAdapter{svc: &stubEffectiveWorkflowReader{err: want}}).ResolveStepContent(context.Background(), adhocapp.Subject{CompanyID: "co"}, "t")
	if !errors.Is(err, want) {
		t.Fatalf("got %v", err)
	}
}

func TestResolveWorkflowSnapshotForMaterialize_ProposalProjectsCapturedRequirements(t *testing.T) {
	for _, schema := range []int{adhocapp.ProposalWorkflowSchemaV2, adhocapp.ProposalWorkflowSchemaV3} {
		step := adhocapp.ProposalWorkflowStep{
			ID: "ps-1", SourceStepID: "tpl-1", Order: 1, Name: "A", ProcessingDays: 1, DepartmentID: "d1",
			Inherited: &adhocapp.ProposalStepInheritedContent{Documents: []adhocapp.ProposalStepDocument{{DocID: "doc-1", Name: "Nghị quyết", Required: true}}},
		}
		if schema == adhocapp.ProposalWorkflowSchemaV3 {
			step.AssigneeMembershipIDs = []string{"m1"}
		} else {
			step.AssigneeMembershipID = "m1"
		}
		snap := &adhocapp.ProposalWorkflowSnapshot{SchemaVersion: schema, Frozen: true, Steps: []adhocapp.ProposalWorkflowStep{step}}
		got, err := resolveWorkflowSnapshotForMaterialize(context.Background(), nil, disclosureapp.Subject{CompanyID: "co"}, "t", adhocapp.CreateRecordOpts{ProposalWorkflow: snap})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.documentRequirements) != 1 || got.documentRequirements[0].StepCode != "ps-1" || !got.documentRequirements[0].Required {
			t.Fatalf("schema %d requirements=%#v", schema, got.documentRequirements)
		}
		if got.effectiveWorkflowN != 0 {
			t.Fatal("proposal path must not read the live template")
		}
	}
}
