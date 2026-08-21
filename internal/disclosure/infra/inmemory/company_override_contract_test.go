package inmemory

import (
	"context"
	"strings"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func validOverrideStep(id string) disclosureapp.WorkflowStepDTO {
	return disclosureapp.WorkflowStepDTO{
		StepID:          id,
		Stage:           "Stage " + id,
		DepartmentID:    "dept-finance",
		AssigneeRoleIds: []string{"role-reviewer"},
		ProcessingDays:  2,
		DisplayOrder:    1,
	}
}

func cmsStep(id string, days int) disclosureapp.WorkflowStepDTO {
	return disclosureapp.WorkflowStepDTO{
		StepID: id, Stage: strings.ToUpper(id), DepartmentID: "d1",
		AssigneeRoleIds: []string{"r1"}, ProcessingDays: days, DisplayOrder: days,
	}
}

func seedPinnedCMS(r *Repository, typeID string, steps []disclosureapp.WorkflowStepDTO) {
	manifest, _, hash, err := disclosureapp.CanonicalWorkflowPublication(steps)
	if err != nil {
		panic(err)
	}
	r.catalog[typeID] = disclosureapp.DisclosureTypeDTO{
		TypeID:                    typeID,
		VersionNo:                 1,
		WorkflowAuthorityMode:     disclosureapp.WorkflowAuthorityTemplatePinned,
		WorkflowManifest:          &manifest,
		PublicationCandidateHash:  hash,
	}
}

// Contract matrix T1–T8 / T12 for company override vs CMS default.
func TestCompanyOverrideContract_DraftDoesNotAffectEffective(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	const typeID = "dt-co-draft"
	const companyID = "company-a"

	seedPinnedCMS(r, typeID, []disclosureapp.WorkflowStepDTO{
		cmsStep("g1", 1), cmsStep("g2", 2), cmsStep("g3", 3),
	})
	active := 1
	r.globalWorkflows = map[string]*disclosureapp.GlobalWorkflowDTO{
		typeID: {
			WorkflowID: "wf-1", TypeID: typeID, Status: "active", ActiveVersionNo: &active,
			Steps: []disclosureapp.GlobalWorkflowStepInput{
				{StepID: "legacy-1", Stage: "LEGACY", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 9, DisplayOrder: 1},
			},
		},
	}

	// T1: no override → pinned CMS default (FE wire source remains global_template)
	eff, err := r.GetEffectiveWorkflow(ctx, companyID, typeID)
	if err != nil {
		t.Fatalf("T1 GetEffectiveWorkflow: %v", err)
	}
	if eff.Source != "global_template" || len(eff.Workflow) != 3 {
		t.Fatalf("T1 source=%s steps=%d want global_template/3", eff.Source, len(eff.Workflow))
	}

	// T2/T3: draft only — effective remains CMS
	_, err = r.UpsertCompanyWorkflowOverrideDraft(ctx, disclosureapp.UpsertCompanyWorkflowOverrideDraftRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID, UserID: "u1"},
		TypeID:  typeID,
		Workflow: []disclosureapp.WorkflowStepDTO{
			validOverrideStep("d1"),
			validOverrideStep("d2"),
			validOverrideStep("d3"),
			validOverrideStep("d4"),
		},
	})
	if err != nil {
		t.Fatalf("T2 upsert draft: %v", err)
	}
	view, err := r.GetCompanyWorkflowOverride(ctx, companyID, typeID)
	if err != nil {
		t.Fatalf("GetCompanyWorkflowOverride: %v", err)
	}
	if view.DraftVersion == nil || len(view.DraftVersion.Workflow) != 4 {
		t.Fatalf("T2 expected draft 4 steps")
	}
	if view.ActiveVersion != nil {
		t.Fatal("T2 draft must not create active version")
	}
	eff, err = r.GetEffectiveWorkflow(ctx, companyID, typeID)
	if err != nil {
		t.Fatalf("T3 GetEffectiveWorkflow: %v", err)
	}
	if eff.Source != "global_template" || len(eff.Workflow) != 3 {
		t.Fatalf("T3 FAIL_COMPANY_OVERRIDE_DRAFT_BECOMES_ACTIVE source=%s steps=%d", eff.Source, len(eff.Workflow))
	}

	// T4: activate → company override wins
	if _, err := r.ApproveCompanyWorkflowOverride(ctx, disclosureapp.ApproveCompanyWorkflowOverrideRequest{
		Subject:   disclosureapp.Subject{CompanyID: companyID, UserID: "u1"},
		TypeID:    typeID,
		VersionNo: view.DraftVersion.VersionNo,
		Reason:    "activate",
	}); err != nil {
		t.Fatalf("T4 approve: %v", err)
	}
	eff, err = r.GetEffectiveWorkflow(ctx, companyID, typeID)
	if err != nil {
		t.Fatalf("T4 GetEffectiveWorkflow: %v", err)
	}
	if eff.Source != "company_override" || len(eff.Workflow) != 4 {
		t.Fatalf("T4 source=%s steps=%d want company_override/4", eff.Source, len(eff.Workflow))
	}

	// T5: draft v2 while active v1 → effective stays v1 (4)
	_, err = r.UpsertCompanyWorkflowOverrideDraft(ctx, disclosureapp.UpsertCompanyWorkflowOverrideDraftRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID, UserID: "u1"},
		TypeID:  typeID,
		Workflow: []disclosureapp.WorkflowStepDTO{
			validOverrideStep("n1"), validOverrideStep("n2"), validOverrideStep("n3"),
			validOverrideStep("n4"), validOverrideStep("n5"),
		},
	})
	if err != nil {
		t.Fatalf("T5 upsert draft v2: %v", err)
	}
	eff, err = r.GetEffectiveWorkflow(ctx, companyID, typeID)
	if err != nil {
		t.Fatalf("T5 GetEffectiveWorkflow: %v", err)
	}
	if eff.Source != "company_override" || len(eff.Workflow) != 4 {
		t.Fatalf("T5 FAIL_COMPANY_OVERRIDE_ACTIVE_VERSION_ISOLATION source=%s steps=%d", eff.Source, len(eff.Workflow))
	}

	view, _ = r.GetCompanyWorkflowOverride(ctx, companyID, typeID)
	if view.DraftVersion == nil || len(view.DraftVersion.Workflow) != 5 {
		t.Fatalf("T5 expected draft 5 steps")
	}

	// T6: activate v2
	if _, err := r.ApproveCompanyWorkflowOverride(ctx, disclosureapp.ApproveCompanyWorkflowOverrideRequest{
		Subject:   disclosureapp.Subject{CompanyID: companyID, UserID: "u1"},
		TypeID:    typeID,
		VersionNo: view.DraftVersion.VersionNo,
		Reason:    "activate-v2",
	}); err != nil {
		t.Fatalf("T6 approve: %v", err)
	}
	eff, err = r.GetEffectiveWorkflow(ctx, companyID, typeID)
	if err != nil {
		t.Fatalf("T6: %v", err)
	}
	if len(eff.Workflow) != 5 {
		t.Fatalf("T6 steps=%d want 5", len(eff.Workflow))
	}

	// T7: mutating legacy Global Workflow must not change pinned CMS or override
	r.globalWorkflows[typeID].Steps = []disclosureapp.GlobalWorkflowStepInput{
		{StepID: "g1", Stage: "G1", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 1, DisplayOrder: 1},
		{StepID: "g2", Stage: "G2", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 2, DisplayOrder: 2},
		{StepID: "g3", Stage: "G3", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 3, DisplayOrder: 3},
		{StepID: "g4", Stage: "G4", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 4, DisplayOrder: 4},
		{StepID: "g5", Stage: "G5", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 5, DisplayOrder: 5},
		{StepID: "g6", Stage: "G6", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 6, DisplayOrder: 6},
	}
	effA, _ := r.GetEffectiveWorkflow(ctx, companyID, typeID)
	if len(effA.Workflow) != 5 || effA.Source != "company_override" {
		t.Fatalf("T7 company A must stay override 5, got source=%s steps=%d", effA.Source, len(effA.Workflow))
	}
	effB, _ := r.GetEffectiveWorkflow(ctx, "company-b", typeID)
	if len(effB.Workflow) != 3 || effB.Source != "global_template" {
		t.Fatalf("T7 company B must see pinned CMS 3, got source=%s steps=%d", effB.Source, len(effB.Workflow))
	}

	// T8: reset → fallback current pinned CMS (still 3, not mutated global)
	reset, err := r.ResetCompanyWorkflowOverrideActive(ctx, disclosureapp.ResetCompanyWorkflowOverrideActiveRequest{
		Subject: disclosureapp.Subject{CompanyID: companyID, UserID: "u1"},
		TypeID:  typeID,
	})
	if err != nil {
		t.Fatalf("T8 reset: %v", err)
	}
	if reset.ActiveVersionNo != 0 {
		t.Fatalf("T8 active_version_no=%d want 0", reset.ActiveVersionNo)
	}
	if reset.EffectiveSource != "global_template" {
		t.Fatalf("T8 EffectiveSource=%s want global_template", reset.EffectiveSource)
	}
	eff, err = r.GetEffectiveWorkflow(ctx, companyID, typeID)
	if err != nil {
		t.Fatalf("T8 GetEffectiveWorkflow: %v", err)
	}
	if eff.Source != "global_template" || len(eff.Workflow) != 3 {
		t.Fatalf("T8 FAIL_COMPANY_OVERRIDE_RESET_TO_DEFAULT source=%s steps=%d", eff.Source, len(eff.Workflow))
	}
}

func TestCompanyOverrideContract_InvalidPublishBlocked(t *testing.T) {
	// T10: invalid/empty override blocked at validation gate (service uses this before activate).
	if err := disclosureapp.ValidateCompanyWorkflowOverrideSteps(nil); err == nil {
		t.Fatal("T10 empty steps must fail ValidateCompanyWorkflowOverrideSteps")
	}
	if err := disclosureapp.ValidateCompanyWorkflowOverrideSteps([]disclosureapp.WorkflowStepDTO{}); err == nil {
		t.Fatal("T10 empty slice must fail ValidateCompanyWorkflowOverrideSteps")
	}
}

func TestCompanyOverrideContract_CompanyLayerUsesPinnedTemplatePublication(t *testing.T) {
	r := NewRepository()
	ctx := context.Background()
	const typeID = "dt-co-t12"
	seedPinnedCMS(r, typeID, []disclosureapp.WorkflowStepDTO{cmsStep("g1", 1), cmsStep("g2", 2)})
	active := 1
	r.globalWorkflows = map[string]*disclosureapp.GlobalWorkflowDTO{
		typeID: {
			WorkflowID: "wf-1", TypeID: typeID, Status: "active", ActiveVersionNo: &active,
			Steps: []disclosureapp.GlobalWorkflowStepInput{
				{StepID: "legacy-a", Stage: "LA", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 1, DisplayOrder: 1},
				{StepID: "legacy-b", Stage: "LB", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 2, DisplayOrder: 2},
				{StepID: "legacy-c", Stage: "LC", DepartmentID: "d1", AssigneeRoleIds: []string{"r1"}, ProcessingDays: 3, DisplayOrder: 3},
			},
		},
	}
	eff, err := r.GetEffectiveWorkflow(ctx, "c-x", typeID)
	if err != nil {
		t.Fatal(err)
	}
	if eff.Source != "global_template" || len(eff.Workflow) != 2 {
		t.Fatalf("T12 want global_template/2 got %s/%d", eff.Source, len(eff.Workflow))
	}
}
