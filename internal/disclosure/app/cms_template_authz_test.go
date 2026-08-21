package app

import (
	"context"
	"net/http"
	"testing"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type cmsTemplateAuthzRepo struct {
	Repository
	updateConfigCalled bool
	versionDetail      *DisclosureTypeDTO
}

func (r *cmsTemplateAuthzRepo) GetActiveGlobalWorkflow(_ context.Context, _ string) ([]WorkflowStepDTO, int, bool, error) {
	return nil, 0, false, nil
}

func (r *cmsTemplateAuthzRepo) ActivateTypeVersion(_ context.Context, req ActivateTypeVersionRequest) (*ActivateTypeVersionResponse, error) {
	return &ActivateTypeVersionResponse{
		TypeID:    req.TypeID,
		VersionNo: req.VersionNo,
		IsActive:  true,
		UpdatedBy: req.Subject.UserID,
	}, nil
}

func pinDisclosureWorkflow(detail *DisclosureTypeDTO) *DisclosureTypeDTO {
	if detail == nil {
		return nil
	}
	steps := ExtractTemplateWorkflow(detail.Blocks)
	if len(steps) == 0 {
		return detail
	}
	manifest, _, hash, err := CanonicalWorkflowPublication(steps)
	if err != nil {
		return detail
	}
	out := *detail
	out.WorkflowAuthorityMode = WorkflowAuthorityTemplatePinned
	out.WorkflowManifest = &manifest
	out.PublicationCandidateHash = hash
	return &out
}

func (r *cmsTemplateAuthzRepo) GetTypeVersionDetail(_ context.Context, _, typeID string, versionNo int) (*DisclosureTypeDTO, error) {
	if r.versionDetail != nil {
		return pinDisclosureWorkflow(r.versionDetail), nil
	}
	return pinDisclosureWorkflow(&DisclosureTypeDTO{
		TypeID:             typeID,
		VersionNo:          versionNo,
		Scope:              "global",
		TemplateCategory:   TemplateCategoryIrregular,
		ApplicabilityRules: applicability.DefaultGlobalRules(false),
		Blocks: []TemplateBlockDTO{
			{
				BlockID:   "block-workflow",
				BlockKey:  "enterprise_workflow",
				BlockType: "rich_text",
				Config: map[string]any{
					"steps": []any{
						map[string]any{
							"step_id":           "review",
							"stage":             "Review",
							"department_id":     "dept-finance",
							"assignee_role_ids": []any{"role-reviewer"},
							"processing_days":   float64(2),
							"display_order":     float64(1),
							"documents":         []any{},
						},
					},
				},
			},
		},
	}), nil
}

func (r *cmsTemplateAuthzRepo) GetActiveVersionDeadlineConfig(_ context.Context, _ string) (int, *TemplateDeadlineConfig, error) {
	return 1, &TemplateDeadlineConfig{
		T0Policy:       "system_date",
		DeadlineDays:   5,
		ProcessingDays: 2,
	}, nil
}

func (r *cmsTemplateAuthzRepo) UpdateActiveVersionDeadlineConfig(_ context.Context, _ string, _ TemplateDeadlineConfig, _ string) error {
	r.updateConfigCalled = true
	return nil
}

func (r *cmsTemplateAuthzRepo) ListActiveDeadlineRuleCatalog(_ context.Context) ([]DeadlineRuleCatalogDTO, error) {
	return defaultDeadlineRuleCatalog(), nil
}

func newCMSTemplateAuthzService(perms []string, repo Repository) Service {
	return NewService(repo, &fakeAuthService{permissions: perms}, idgen.UUIDv7Generator{})
}

func TestGetTemplateReferenceData_RequiresCMSReadCapability(t *testing.T) {
	svc := newCMSTemplateAuthzService(
		[]string{permissionPlatformCMSView, permissionCMSTemplateRead},
		&cmsTemplateAuthzRepo{},
	)
	resp, err := svc.GetTemplateReferenceData(context.Background(), GetTemplateReferenceDataRequest{
		Subject: Subject{MembershipID: "member-001", CompanyID: "company-001"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Data.DeadlineRuleCatalog) != 2 {
		t.Fatalf("deadline rule catalog length=%d want 2", len(resp.Data.DeadlineRuleCatalog))
	}
}

func TestGetTemplateReferenceData_DeniesPlatformOnlyActor(t *testing.T) {
	svc := newCMSTemplateAuthzService(
		[]string{permissionPlatformCMSView},
		&cmsTemplateAuthzRepo{},
	)
	_, err := svc.GetTemplateReferenceData(context.Background(), GetTemplateReferenceDataRequest{
		Subject: Subject{MembershipID: "member-001", CompanyID: "company-001"},
	})
	if err == nil {
		t.Fatal("expected permission error")
	}
	herr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T", err)
	}
	if herr.HTTPStatus != http.StatusForbidden {
		t.Fatalf("status=%d want %d", herr.HTTPStatus, http.StatusForbidden)
	}
}

func TestUpdateTemplateDeadlineConfig_AllowsCMSTemplateWriteCompatibility(t *testing.T) {
	repo := &cmsTemplateAuthzRepo{}
	svc := newCMSTemplateAuthzService(
		[]string{permissionPlatformCMSView, permissionCMSTemplateWrite},
		repo,
	)
	_, err := svc.UpdateTemplateDeadlineConfig(context.Background(), UpdateTemplateDeadlineConfigRequest{
		Subject: Subject{UserID: "user-001", MembershipID: "member-001", CompanyID: "company-001"},
		TypeID:  "dt-001",
		DeadlineConfig: TemplateDeadlineConfig{
			T0Policy:       "system_date",
			DeadlineDays:   5,
			ProcessingDays: 2,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.updateConfigCalled {
		t.Fatal("expected UpdateActiveVersionDeadlineConfig to be called")
	}
}

func TestActivateTypeVersion_AllowsCanonicalActivatePermission(t *testing.T) {
	svc := newCMSTemplateAuthzService(
		[]string{permissionPlatformCMSView, permissionCMSTemplateActivate},
		&cmsTemplateAuthzRepo{},
	)
	resp, err := svc.ActivateTypeVersion(context.Background(), ActivateTypeVersionRequest{
		Subject:   Subject{UserID: "user-001", MembershipID: "member-001", CompanyID: "company-001"},
		TypeID:    "dt-001",
		VersionNo: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.VersionNo != 2 {
		t.Fatalf("version_no=%d want 2", resp.VersionNo)
	}
}

func TestActivateTypeVersion_RejectsVersionWithoutWorkflow(t *testing.T) {
	svc := newCMSTemplateAuthzService(
		[]string{permissionPlatformCMSView, permissionCMSTemplateActivate},
		&cmsTemplateAuthzRepo{
			versionDetail: &DisclosureTypeDTO{
				TypeID:    "dt-001",
				VersionNo: 2,
				Blocks:    []TemplateBlockDTO{},
			},
		},
	)
	_, err := svc.ActivateTypeVersion(context.Background(), ActivateTypeVersionRequest{
		Subject:   Subject{UserID: "user-001", MembershipID: "member-001", CompanyID: "company-001"},
		TypeID:    "dt-001",
		VersionNo: 2,
	})
	if err == nil {
		t.Fatal("expected activation error")
	}
	herr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T", err)
	}
	if herr.HTTPStatus != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want %d", herr.HTTPStatus, http.StatusUnprocessableEntity)
	}
	if herr.Code != "TEMPLATE_NO_WORKFLOW" && herr.Code != "TEMPLATE_WORKFLOW_NOT_PINNED" {
		t.Fatalf("code=%s want TEMPLATE_NO_WORKFLOW or TEMPLATE_WORKFLOW_NOT_PINNED", herr.Code)
	}
}
