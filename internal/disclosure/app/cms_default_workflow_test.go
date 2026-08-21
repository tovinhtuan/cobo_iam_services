package app

import (
	"context"
	"strings"
	"testing"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

func validStep(id, stage string) WorkflowStepDTO {
	return WorkflowStepDTO{
		StepID:          id,
		Stage:           stage,
		DepartmentID:    "dept-finance",
		AssigneeRoleIds: []string{"role-reviewer"},
		ProcessingDays:  2,
		DisplayOrder:    1,
	}
}

func enterpriseBlocks(steps ...WorkflowStepDTO) []TemplateBlockDTO {
	raw := make([]any, 0, len(steps))
	for i, s := range steps {
		role := ""
		if len(s.AssigneeRoleIds) > 0 {
			role = s.AssigneeRoleIds[0]
		}
		raw = append(raw, map[string]any{
			"step_id":           s.StepID,
			"stage":             s.Stage,
			"department_id":     s.DepartmentID,
			"assignee_role_ids": []any{role},
			"processing_days":   float64(s.ProcessingDays),
			"display_order":     float64(i + 1),
			"documents":         []any{},
		})
	}
	return []TemplateBlockDTO{{
		BlockKey: "enterprise_workflow",
		Config:   map[string]any{"steps": raw},
	}}
}

func TestResolveCMSDefaultWorkflow_CaseMatrix(t *testing.T) {
	global3 := []WorkflowStepDTO{
		validStep("g1", "G1"),
		validStep("g2", "G2"),
		validStep("g3", "G3"),
	}
	for i := range global3 {
		global3[i].DisplayOrder = i + 1
	}
	enterprise3 := []WorkflowStepDTO{
		validStep("e1", "E1"),
		validStep("e2", "E2"),
		validStep("e3", "E3"),
	}
	for i := range enterprise3 {
		enterprise3[i].DisplayOrder = i + 1
	}
	enterprise6 := make([]WorkflowStepDTO, 6)
	for i := 0; i < 6; i++ {
		enterprise6[i] = validStep("ex"+strings.Repeat("x", i+1), "E"+strings.Repeat("x", i+1))
		enterprise6[i].DisplayOrder = i + 1
	}
	invalidGlobal := []WorkflowStepDTO{{
		StepID: "bad", Stage: "Bad", DepartmentID: "", AssigneeRoleIds: nil, ProcessingDays: 0,
	}}

	tests := []struct {
		name       string
		in         CMSDefaultWorkflowInput
		wantSource string
		wantSteps  int
		wantHas    bool
		wantValid  bool
	}{
		{
			name: "CASE_A_active_global_enterprise_empty",
			in: CMSDefaultWorkflowInput{
				ActiveGlobalOK: true, ActiveGlobalVersionNo: 1, ActiveGlobalSteps: global3,
				EnterpriseBlocks: enterpriseBlocks(),
			},
			wantSource: CMSDefaultSourceGlobalWorkflow, wantSteps: 3, wantHas: true, wantValid: true,
		},
		{
			name: "CASE_B_global_draft_only_ignored",
			in: CMSDefaultWorkflowInput{
				ActiveGlobalOK:    false,
				ActiveGlobalSteps: global3,
				EnterpriseBlocks:  enterpriseBlocks(),
			},
			wantSource: CMSDefaultSourceNone, wantSteps: 0, wantHas: false, wantValid: false,
		},
		{
			name: "CASE_C_enterprise_fallback",
			in: CMSDefaultWorkflowInput{
				ActiveGlobalOK:      false,
				EnterpriseBlocks:    enterpriseBlocks(enterprise3...),
				EnterpriseVersionNo: 2,
			},
			wantSource: CMSDefaultSourceTemplateEnterprise, wantSteps: 3, wantHas: true, wantValid: true,
		},
		{
			name: "CASE_D_dual_store_global_wins",
			in: CMSDefaultWorkflowInput{
				ActiveGlobalOK: true, ActiveGlobalVersionNo: 1, ActiveGlobalSteps: global3,
				EnterpriseBlocks: enterpriseBlocks(enterprise6...),
			},
			wantSource: CMSDefaultSourceGlobalWorkflow, wantSteps: 3, wantHas: true, wantValid: true,
		},
		{
			name: "CASE_E_active_vs_draft_isolation_uses_active_only",
			in: CMSDefaultWorkflowInput{
				ActiveGlobalOK: true, ActiveGlobalVersionNo: 1, ActiveGlobalSteps: global3,
			},
			wantSource: CMSDefaultSourceGlobalWorkflow, wantSteps: 3, wantHas: true, wantValid: true,
		},
		{
			name: "CASE_F_active_but_invalid",
			in: CMSDefaultWorkflowInput{
				ActiveGlobalOK: true, ActiveGlobalVersionNo: 1, ActiveGlobalSteps: invalidGlobal,
			},
			wantSource: CMSDefaultSourceGlobalWorkflow, wantSteps: 1, wantHas: true, wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveCMSDefaultWorkflow(tt.in)
			if got.Source != tt.wantSource {
				t.Fatalf("source=%s want %s", got.Source, tt.wantSource)
			}
			if len(got.Steps) != tt.wantSteps {
				t.Fatalf("steps=%d want %d", len(got.Steps), tt.wantSteps)
			}
			if got.HasWorkflow != tt.wantHas {
				t.Fatalf("has_workflow=%v want %v", got.HasWorkflow, tt.wantHas)
			}
			if got.IsValid != tt.wantValid {
				t.Fatalf("is_valid=%v want %v errors=%v", got.IsValid, tt.wantValid, got.ValidationErrors)
			}
		})
	}
}

func TestActivateTypeVersion_RequiresPinnedTemplatePublication(t *testing.T) {
	repo := &cmsActivateCanonicalRepo{
		cmsTemplateAuthzRepo: cmsTemplateAuthzRepo{
			versionDetail: &DisclosureTypeDTO{
				TypeID:             "dt-global-only",
				VersionNo:          2,
				Scope:              "global",
				TemplateCategory:   TemplateCategoryIrregular,
				ApplicabilityRules: applicability.DefaultGlobalRules(false),
				Blocks:             enterpriseBlocks(validStep("g1", "G1"), validStep("g2", "G2"), validStep("g3", "G3")),
			},
		},
		globalSteps: []WorkflowStepDTO{
			validStep("legacy-g1", "LegacyG1"),
		},
		globalVersion: 1,
	}
	svc := newCMSTemplateAuthzService(
		[]string{permissionPlatformCMSView, permissionCMSTemplateActivate},
		repo,
	)
	resp, err := svc.ActivateTypeVersion(context.Background(), ActivateTypeVersionRequest{
		Subject:   Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
		TypeID:    "dt-global-only",
		VersionNo: 2,
	})
	if err != nil {
		t.Fatalf("CASE_A activate should pass with pinned template publication: %v", err)
	}
	if resp.VersionNo != 2 {
		t.Fatalf("version_no=%d want 2", resp.VersionNo)
	}
}

func TestActivateTypeVersion_BlocksWhenOnlyGlobalDraft(t *testing.T) {
	repo := &cmsActivateCanonicalRepo{
		cmsTemplateAuthzRepo: cmsTemplateAuthzRepo{
			versionDetail: &DisclosureTypeDTO{
				TypeID:    "dt-draft-only",
				VersionNo: 1,
				Blocks:    []TemplateBlockDTO{},
			},
		},
	}
	svc := newCMSTemplateAuthzService(
		[]string{permissionPlatformCMSView, permissionCMSTemplateActivate},
		repo,
	)
	_, err := svc.ActivateTypeVersion(context.Background(), ActivateTypeVersionRequest{
		Subject:   Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
		TypeID:    "dt-draft-only",
		VersionNo: 1,
	})
	if err == nil {
		t.Fatal("CASE_B expected TEMPLATE_WORKFLOW_NOT_PINNED or TEMPLATE_NO_WORKFLOW")
	}
}

type cmsActivateCanonicalRepo struct {
	cmsTemplateAuthzRepo
	globalSteps   []WorkflowStepDTO
	globalVersion int
}

func (r *cmsActivateCanonicalRepo) GetActiveGlobalWorkflow(_ context.Context, _ string) ([]WorkflowStepDTO, int, bool, error) {
	if len(r.globalSteps) == 0 {
		return nil, 0, false, nil
	}
	return r.globalSteps, r.globalVersion, true, nil
}
