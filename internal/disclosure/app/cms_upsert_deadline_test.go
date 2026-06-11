package app

import (
	"context"
	"net/http"
	"testing"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// upsertDeadlineRepo stubs the minimal Repository methods needed for UpsertTypeVersion.
type upsertDeadlineRepo struct {
	Repository
	upsertCalled bool
}

func (r *upsertDeadlineRepo) ListActiveDeadlineRuleCatalog(_ context.Context) ([]DeadlineRuleCatalogDTO, error) {
	return defaultDeadlineRuleCatalog(), nil
}

func (r *upsertDeadlineRepo) ListDisplayGroups(_ context.Context) ([]DisplayGroupDTO, error) {
	return []DisplayGroupDTO{{DisplayGroupCode: "display_groups_001", NameVI: "Test Group", IsActive: true}}, nil
}

func (r *upsertDeadlineRepo) UpsertTypeVersion(_ context.Context, req UpsertTypeVersionRequest) (*UpsertTypeVersionResponse, error) {
	r.upsertCalled = true
	return &UpsertTypeVersionResponse{TypeID: req.TypeID, VersionNo: 1, IsActive: true, UpdatedBy: req.Subject.UserID}, nil
}

func newCMSUpsertDeadlineService(repo Repository) Service {
	return NewService(repo, &fakeAuthService{
		permissions: []string{permissionPlatformCMSView, permissionCMSTemplateWrite},
	}, idgen.UUIDv7Generator{})
}

func baseUpsertRequest() UpsertTypeVersionRequest {
	return UpsertTypeVersionRequest{
		Subject:          Subject{UserID: "user-001", MembershipID: "m-001", CompanyID: "c-001"},
		TypeID:           "test-template-001",
		Scope:            "global",
		GroupID:          "group-001",
		Name:             "Test Template",
		Category:         "Định kỳ",
		TemplateCategory: TemplateCategoryPeriodic,
		DeadlineStrategy: DeadlineStrategyFixedCycleDays,
		Periodicity:      PeriodicityQuarterly,
		DisplayGroupCodes:  []string{"display_groups_001"},
		ApplicabilityRules: applicability.DefaultGlobalRules(true),
		Blocks: []TemplateBlockDTO{
			{BlockID: "b1", BlockKey: "legal_basis", BlockType: "rich_text", Title: "Cơ sở", Description: "test", DisplayOrder: 1, Config: map[string]any{"max_length": 8000, "allow_html": false}},
			{BlockID: "b2", BlockKey: "disclosure_content", BlockType: "rich_text", Title: "Nội dung", Description: "test", DisplayOrder: 2, Config: map[string]any{"max_length": 50000, "allow_html": true}},
			{BlockID: "b3", BlockKey: "deadline", BlockType: "text", Title: "Kỳ hạn", Description: "test", DisplayOrder: 3, Config: map[string]any{"max_length": 4000}},
			{BlockID: "b4", BlockKey: "channels_and_format", BlockType: "rich_text", Title: "Kênh", Description: "test", DisplayOrder: 4, Config: map[string]any{
				"max_length": 12000, "allow_html": false,
				"channels":   []any{map[string]any{"name": "Web", "disclosure_method": "ELECTRONIC", "file_types": []any{"PDF"}, "attachment_requirement": "REQUIRED"}},
				"file_types": []any{"PDF"},
			}},
			{BlockID: "b5", BlockKey: "legal_risks", BlockType: "rich_text", Title: "Rủi ro", Description: "test", DisplayOrder: 5, Config: map[string]any{"max_length": 8000, "allow_html": false}},
			{BlockID: "b6", BlockKey: "enterprise_workflow", BlockType: "rich_text", Title: "Workflow", Description: "test", DisplayOrder: 6, Config: map[string]any{
				"max_length": 12000, "allow_html": true,
				"steps": []any{map[string]any{"step_id": "s1", "stage": "Bước 1", "department_id": "d1", "assignee_role_ids": []any{"r1"}, "processing_days": float64(3), "display_order": float64(1), "documents": []any{}}},
			}},
		},
	}
}

// TestUpsertTypeVersion_RejectsInvalidDeadlineRuleFormat verifies the CMS global
// template write path validates deadline_rule against the catalog.
// RED: This test should fail before the fix (deadline_rule not validated).
func TestUpsertTypeVersion_RejectsInvalidDeadlineRuleFormat(t *testing.T) {
	repo := &upsertDeadlineRepo{}
	svc := newCMSUpsertDeadlineService(repo)

	req := baseUpsertRequest()
	req.DeadlineRule = "5 ngay khong hop le"

	_, err := svc.UpsertTypeVersion(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error for invalid deadline_rule; CMS path must validate deadline_rule format")
	}
	herr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T", err)
	}
	if herr.HTTPStatus != http.StatusBadRequest {
		t.Errorf("status=%d want 400", herr.HTTPStatus)
	}
	if repo.upsertCalled {
		t.Error("UpsertTypeVersion should not be called when deadline_rule is invalid")
	}
}

// TestUpsertTypeVersion_AcceptsValidTNDeadlineRule verifies T+N format passes.
func TestUpsertTypeVersion_AcceptsValidTNDeadlineRule(t *testing.T) {
	repo := &upsertDeadlineRepo{}
	svc := newCMSUpsertDeadlineService(repo)

	req := baseUpsertRequest()
	req.DeadlineRule = "T+20"

	resp, err := svc.UpsertTypeVersion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error for valid T+N deadline_rule: %v", err)
	}
	if resp.TypeID != req.TypeID {
		t.Errorf("type_id=%s want %s", resp.TypeID, req.TypeID)
	}
	if !repo.upsertCalled {
		t.Error("UpsertTypeVersion should be called for valid request")
	}
}

// TestUpsertTypeVersion_AcceptsDatePatternDeadlineRule verifies dd/mm format passes.
func TestUpsertTypeVersion_AcceptsDatePatternDeadlineRule(t *testing.T) {
	repo := &upsertDeadlineRepo{}
	svc := newCMSUpsertDeadlineService(repo)

	req := baseUpsertRequest()
	req.DeadlineRule = "31/12"
	req.TemplateCategory = TemplateCategoryIrregular
	req.Periodicity = PeriodicityEventBased
	req.DeadlineStrategy = DeadlineStrategyEventHours
	req.ApplicabilityRules = applicability.DefaultGlobalRules(false)

	resp, err := svc.UpsertTypeVersion(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error for dd/mm deadline_rule: %v", err)
	}
	if resp.TypeID != req.TypeID {
		t.Errorf("type_id=%s want %s", resp.TypeID, req.TypeID)
	}
}

// activateDeadlineRepo provides a version detail with configurable deadline_rule.
type activateDeadlineRepo struct {
	Repository
	deadlineRule string
}

func (r *activateDeadlineRepo) ListActiveDeadlineRuleCatalog(_ context.Context) ([]DeadlineRuleCatalogDTO, error) {
	return defaultDeadlineRuleCatalog(), nil
}

func (r *activateDeadlineRepo) GetTypeVersionDetail(_ context.Context, _, typeID string, versionNo int) (*DisclosureTypeDTO, error) {
	return &DisclosureTypeDTO{
		TypeID:             typeID,
		VersionNo:          versionNo,
		Scope:              "global",
		TemplateCategory:   TemplateCategoryPeriodic,
		DeadlineRule:       r.deadlineRule,
		ApplicabilityRules: applicability.DefaultGlobalRules(true),
		Blocks: []TemplateBlockDTO{
			{BlockID: "bwf", BlockKey: "enterprise_workflow", BlockType: "rich_text", Config: map[string]any{
				"steps": []any{map[string]any{
					"step_id": "s1", "stage": "Review", "department_id": "dept-finance",
					"assignee_role_ids": []any{"role-reviewer"}, "processing_days": float64(2),
					"display_order": float64(1), "documents": []any{},
				}},
			}},
		},
	}, nil
}

func (r *activateDeadlineRepo) ActivateTypeVersion(_ context.Context, req ActivateTypeVersionRequest) (*ActivateTypeVersionResponse, error) {
	return &ActivateTypeVersionResponse{TypeID: req.TypeID, VersionNo: req.VersionNo, IsActive: true}, nil
}

func newCMSActivateDeadlineService(deadlineRule string) Service {
	return NewService(&activateDeadlineRepo{deadlineRule: deadlineRule}, &fakeAuthService{
		permissions: []string{permissionPlatformCMSView, permissionCMSTemplateActivate},
	}, idgen.UUIDv7Generator{})
}

// TestActivateTypeVersion_RejectsInvalidDeadlineRule verifies the activate path
// re-validates deadline_rule before publishing to Portal.
// RED: This test should fail before the fix.
func TestActivateTypeVersion_RejectsInvalidDeadlineRule(t *testing.T) {
	svc := newCMSActivateDeadlineService("5 ngay khong hop le")
	_, err := svc.ActivateTypeVersion(context.Background(), ActivateTypeVersionRequest{
		Subject:   Subject{UserID: "u-001", MembershipID: "m-001", CompanyID: "c-001"},
		TypeID:    "dt-001",
		VersionNo: 2,
	})
	if err == nil {
		t.Fatal("expected validation error: activate should reject invalid deadline_rule")
	}
	herr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T", err)
	}
	if herr.HTTPStatus != http.StatusBadRequest {
		t.Errorf("status=%d want 400", herr.HTTPStatus)
	}
}

// TestActivateTypeVersion_AcceptsValidDeadlineRule verifies valid T+N passes activation.
func TestActivateTypeVersion_AcceptsValidDeadlineRule(t *testing.T) {
	svc := newCMSActivateDeadlineService("T+20")
	resp, err := svc.ActivateTypeVersion(context.Background(), ActivateTypeVersionRequest{
		Subject:   Subject{UserID: "u-001", MembershipID: "m-001", CompanyID: "c-001"},
		TypeID:    "dt-001",
		VersionNo: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error for valid T+20: %v", err)
	}
	if resp.VersionNo != 2 {
		t.Errorf("version_no=%d want 2", resp.VersionNo)
	}
}
