package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type zeroDBWriteSpyRepo struct {
	disclosureapp.Repository
	writeCount                   int
	listDisplayGroupsCount       int
	listTemplateDepartmentsCount int
	typeExistsCount              int
}

func (s *zeroDBWriteSpyRepo) ListDisplayGroups(ctx context.Context) ([]disclosureapp.DisplayGroupDTO, error) {
	s.listDisplayGroupsCount++
	return s.Repository.ListDisplayGroups(ctx)
}

func (s *zeroDBWriteSpyRepo) ListTemplateDepartments(ctx context.Context) ([]disclosureapp.TemplateDepartmentDTO, error) {
	s.listTemplateDepartmentsCount++
	return s.Repository.ListTemplateDepartments(ctx)
}

func (s *zeroDBWriteSpyRepo) TypeExists(ctx context.Context, typeID string) (bool, error) {
	s.typeExistsCount++
	return s.Repository.TypeExists(ctx, typeID)
}

func (s *zeroDBWriteSpyRepo) Create(ctx context.Context, rec disclosureapp.RecordDTO) (*disclosureapp.RecordDTO, error) {
	s.writeCount++
	return s.Repository.Create(ctx, rec)
}

func (s *zeroDBWriteSpyRepo) Update(ctx context.Context, rec disclosureapp.RecordDTO) (*disclosureapp.RecordDTO, error) {
	s.writeCount++
	return s.Repository.Update(ctx, rec)
}

func (s *zeroDBWriteSpyRepo) UpsertTypeVersion(ctx context.Context, req disclosureapp.UpsertTypeVersionRequest) (*disclosureapp.UpsertTypeVersionResponse, error) {
	s.writeCount++
	return s.Repository.UpsertTypeVersion(ctx, req)
}

func (s *zeroDBWriteSpyRepo) ArchiveGlobalTemplate(ctx context.Context, typeID, userID string) error {
	s.writeCount++
	return s.Repository.ArchiveGlobalTemplate(ctx, typeID, userID)
}

func (s *zeroDBWriteSpyRepo) CreateDisplayGroup(ctx context.Context, req disclosureapp.CmsDisplayGroupCreateRequest) (*disclosureapp.DisplayGroupDTO, error) {
	s.writeCount++
	return s.Repository.CreateDisplayGroup(ctx, req)
}

func (s *zeroDBWriteSpyRepo) CreateTemplateDepartment(ctx context.Context, req disclosureapp.CmsTemplateDepartmentCreateRequest) (*disclosureapp.TemplateDepartmentDTO, error) {
	s.writeCount++
	return s.Repository.CreateTemplateDepartment(ctx, req)
}

type importAuthMock struct {
	permissions []string
}

func (a *importAuthMock) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{Permissions: a.permissions}, nil
}

func (a *importAuthMock) Authorize(_ context.Context, _ authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return nil, nil
}

func (a *importAuthMock) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return nil, nil
}

func newImportService(repo disclosureapp.Repository, perms []string) disclosureapp.Service {
	return disclosureapp.NewService(repo, &importAuthMock{permissions: perms}, idgen.UUIDv7Generator{})
}

func TestTemplateImportService_ZeroDBWriteAssertion(t *testing.T) {
	rawRepo := inmemory.NewRepository()
	spy := &zeroDBWriteSpyRepo{Repository: rawRepo}
	svc := newImportService(spy, []string{"platform.cms.view", "cms.template.write"})
	sub := disclosureapp.Subject{UserID: "admin-1", CompanyID: "cobo-platform"}

	examplePath := filepath.Join("..", "..", "..", "docs", "schema", "template-import-v1.example.json")
	fileBytes, err := os.ReadFile(examplePath)
	if err != nil {
		t.Fatalf("failed reading example file: %v", err)
	}

	// 1. Execute Validate on valid example file
	resp, err := svc.ValidateTemplateImport(context.Background(), disclosureapp.ValidateTemplateImportRequest{
		Subject:   sub,
		Filename:  "template-import-v1.example.json",
		FileBytes: fileBytes,
	})
	if err != nil {
		t.Fatalf("ValidateTemplateImport failed: %v", err)
	}

	if !resp.ParseValid {
		t.Errorf("expected ParseValid=true, got false: %v", resp.Errors)
	}
	if !resp.DomainValid {
		t.Errorf("expected DomainValid=true, got false: %v", resp.Errors)
	}

	// 2. HARD PROOF: Zero DB writes executed
	if spy.writeCount != 0 {
		t.Fatalf("VALIDATE_DB_WRITE_COUNT = %d, want 0", spy.writeCount)
	}
}

func TestTemplateImportService_PermissionGate(t *testing.T) {
	rawRepo := inmemory.NewRepository()
	// Subject with only portal read permission (no CMS permission)
	svc := newImportService(rawRepo, []string{"disclosure.record.view"})
	sub := disclosureapp.Subject{UserID: "user-regular", CompanyID: "company-1"}

	_, err := svc.ValidateTemplateImport(context.Background(), disclosureapp.ValidateTemplateImportRequest{
		Subject:   sub,
		Filename:  "test.json",
		FileBytes: []byte(`{"schema_version":"1.0","template":{"name":"T1","template_category":"periodic","deadline_rule":"T+5"}}`),
	})

	if err == nil {
		t.Fatal("expected 403 Forbidden for missing platform.cms.view, got nil")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusForbidden {
		t.Errorf("expected StatusForbidden (403), got %v", err)
	}
}

func TestTemplateImportService_StrictConcatenatedJSONRejection(t *testing.T) {
	rawRepo := inmemory.NewRepository()
	svc := newImportService(rawRepo, []string{"platform.cms.view", "cms.template.write"})
	sub := disclosureapp.Subject{UserID: "admin-1", CompanyID: "cobo-platform"}

	concatenated := []byte(`
		{"schema_version":"1.0","template":{"name":"T1","template_category":"periodic","deadline_rule":"T+5"}}
		{"schema_version":"1.0","template":{"name":"T2","template_category":"periodic","deadline_rule":"T+5"}}
	`)

	resp, err := svc.ValidateTemplateImport(context.Background(), disclosureapp.ValidateTemplateImportRequest{
		Subject:   sub,
		Filename:  "concat.json",
		FileBytes: concatenated,
	})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}

	if resp.ParseValid {
		t.Error("expected ParseValid=false for concatenated JSON, got true")
	}
	found := false
	for _, e := range resp.Errors {
		if e.Code == "MULTIPLE_JSON_VALUES_REJECTED" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected MULTIPLE_JSON_VALUES_REJECTED error, got %v", resp.Errors)
	}
}

func TestTemplateImportService_UnsupportedSchemaVersionReturns422(t *testing.T) {
	rawRepo := inmemory.NewRepository()
	svc := newImportService(rawRepo, []string{"platform.cms.view", "cms.template.write"})
	sub := disclosureapp.Subject{UserID: "admin-1", CompanyID: "cobo-platform"}

	badVersion := []byte(`{"schema_version":"2.0","template":{"name":"T1","template_category":"periodic","deadline_rule":"T+5"}}`)

	_, err := svc.ValidateTemplateImport(context.Background(), disclosureapp.ValidateTemplateImportRequest{
		Subject:   sub,
		Filename:  "v2.json",
		FileBytes: badVersion,
	})

	if err == nil {
		t.Fatal("expected error for schema_version 2.0, got nil")
	}
	he, ok := perr.AsHTTPError(err)
	if !ok || he.HTTPStatus != http.StatusUnprocessableEntity {
		t.Errorf("expected StatusUnprocessableEntity (422), got %v", err)
	}
}

func TestTemplateImportService_TokenIssuanceLifecycle(t *testing.T) {
	t.Setenv("CMS_TEMPLATE_IMPORT_SIGNING_SECRET", "test-signing-secret-for-suite")
	rawRepo := inmemory.NewRepository()
	svc := newImportService(rawRepo, []string{"platform.cms.view", "cms.template.write"})
	sub := disclosureapp.Subject{UserID: "admin-creator-123", CompanyID: "cobo-platform"}

	// 1. Example file with unseeded departments requires mapping -> can_confirm=false
	examplePath := filepath.Join("..", "..", "..", "docs", "schema", "template-import-v1.example.json")
	fileBytes, _ := os.ReadFile(examplePath)

	resp, err := svc.ValidateTemplateImport(context.Background(), disclosureapp.ValidateTemplateImportRequest{
		Subject:   sub,
		Filename:  "template-import-v1.example.json",
		FileBytes: fileBytes,
	})
	if err != nil {
		t.Fatalf("ValidateTemplateImport failed: %v", err)
	}

	if !resp.DomainValid {
		t.Fatalf("expected DomainValid=true, got false: %v", resp.Errors)
	}
	if !resp.MappingRequired {
		t.Error("expected MappingRequired=true for example file with foreign department codes")
	}
	if resp.CanConfirm {
		t.Error("expected CanConfirm=false while department mappings are required")
	}

	// 2. Matching target department catalog produces CanConfirm=true and stateless HMAC token
	matchingJSON := []byte(`{
		"schema_version": "1.0",
		"template": {
			"name": "BCTC Quý Hoàn Thiện",
			"template_category": "periodic",
			"periodicity": "quarterly",
			"deadline_rule": "Trong vòng 20 ngày kể từ khi kết thúc quý.",
			"display_group_codes": ["display_groups_003"],
			"deadline_config": {
				"frequency_unit": "QUARTERLY",
				"cycle_anchor_day": 31,
				"month_in_quarter": 3,
				"duration_type": "CALENDAR_DAYS"
			},
			"workflow": {
				"steps": [
					{
						"stage": "Lập báo cáo",
						"department_id": "dept-003",
						"assignee_role_ids": ["reviewer"],
						"processing_days": 5
					},
					{
						"stage": "Phê duyệt",
						"department_id": "dept-001",
						"assignee_role_ids": ["approver"],
						"processing_days": 2
					}
				]
			}
		}
	}`)

	resp2, err := svc.ValidateTemplateImport(context.Background(), disclosureapp.ValidateTemplateImportRequest{
		Subject:   sub,
		Filename:  "matching.json",
		FileBytes: matchingJSON,
	})
	if err != nil {
		t.Fatalf("ValidateTemplateImport matching failed: %v", err)
	}

	if !resp2.CanConfirm {
		t.Fatalf("expected CanConfirm=true for matching departments, got false: errs=%v, mappings=%v", resp2.Errors, resp2.RequiredMappings)
	}
	if resp2.ValidationToken == "" {
		t.Fatal("expected non-empty ValidationToken when CanConfirm=true")
	}
	if resp2.TokenExpiresAt == "" {
		t.Fatal("expected non-empty TokenExpiresAt")
	}

	// Verify token claims and signature
	signer := disclosureapp.NewTemplateImportSigner("", 0)
	claims, err := signer.VerifyToken(resp2.ValidationToken, sub.UserID, time.Now())
	if err != nil {
		t.Fatalf("VerifyToken failed on issued validation token: %v", err)
	}

	if claims.ActorID != sub.UserID {
		t.Errorf("claims.ActorID = %q, want %q", claims.ActorID, sub.UserID)
	}
	if claims.SchemaVersion != "1.0" {
		t.Errorf("claims.SchemaVersion = %q, want 1.0", claims.SchemaVersion)
	}
	if claims.PayloadHash == "" {
		t.Error("claims.PayloadHash is empty")
	}
}

func TestTemplateImportService_NoNPlusOneQueryCount(t *testing.T) {
	rawRepo := inmemory.NewRepository()
	spy := &zeroDBWriteSpyRepo{Repository: rawRepo}
	svc := newImportService(spy, []string{"platform.cms.view", "cms.template.write"})
	sub := disclosureapp.Subject{UserID: "admin-scale-test", CompanyID: "cobo-platform"}

	// Build a fixture with 30 workflow steps referencing various departments and display groups
	steps := make([]disclosureapp.TemplateImportWorkflowStepV1, 0, 30)
	for i := 1; i <= 30; i++ {
		deptID := fmt.Sprintf("dept-%03d", (i%4)+1)
		steps = append(steps, disclosureapp.TemplateImportWorkflowStepV1{
			Stage:           fmt.Sprintf("Giai đoạn %d", i),
			DepartmentID:    deptID,
			AssigneeRoleIDs: []string{"reviewer", "approver"},
			ProcessingDays:  2,
		})
	}

	payload := disclosureapp.TemplateImportEnvelopeV1{
		SchemaVersion: "1.0",
		Template: disclosureapp.TemplateImportDefinitionV1{
			Name:              "Báo cáo quy mô 30 bước",
			TemplateCategory:  "periodic",
			Periodicity:       "monthly",
			DeadlineRule:      "T+5",
			DisplayGroupCodes: []string{"display_groups_001", "display_groups_002", "display_groups_003"},
			Workflow: &disclosureapp.TemplateImportWorkflowV1{
				Steps: steps,
			},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal fixture failed: %v", err)
	}

	resp, err := svc.ValidateTemplateImport(context.Background(), disclosureapp.ValidateTemplateImportRequest{
		Subject:   sub,
		Filename:  "scale_30_steps.json",
		FileBytes: payloadBytes,
	})
	if err != nil {
		t.Fatalf("ValidateTemplateImport failed: %v", err)
	}

	if !resp.DomainValid {
		t.Fatalf("expected DomainValid=true, got errors: %v", resp.Errors)
	}

	// Assertions for query counts:
	// Total DB writes MUST remain strictly 0
	if spy.writeCount != 0 {
		t.Errorf("VALIDATE_DB_WRITE_COUNT = %d, want 0", spy.writeCount)
	}

	// Catalog queries MUST be bounded O(1), not O(30)!
	if spy.listDisplayGroupsCount > 1 {
		t.Errorf("listDisplayGroupsCount = %d, want <= 1 (N+1 violation!)", spy.listDisplayGroupsCount)
	}
	if spy.listTemplateDepartmentsCount > 1 {
		t.Errorf("listTemplateDepartmentsCount = %d, want <= 1 (N+1 violation!)", spy.listTemplateDepartmentsCount)
	}
	if spy.typeExistsCount > 1 {
		t.Errorf("typeExistsCount = %d, want <= 1 (N+1 violation!)", spy.typeExistsCount)
	}
}
