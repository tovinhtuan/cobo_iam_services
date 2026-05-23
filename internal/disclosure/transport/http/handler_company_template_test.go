package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type fakeDisclosureHandlerService struct {
	disclosureapp.Service
	createResp     *disclosureapp.CompanyTemplateWriteResponse
	createErr      error
	updateResp     *disclosureapp.CompanyTemplateWriteResponse
	updateErr      error
	transitionResp *disclosureapp.CompanyTemplateWriteResponse
	transitionErr  error
}

func (f *fakeDisclosureHandlerService) CreateCompanyTemplate(_ context.Context, _ disclosureapp.CreateCompanyTemplateRequest) (*disclosureapp.CompanyTemplateWriteResponse, error) {
	return f.createResp, f.createErr
}

func (f *fakeDisclosureHandlerService) UpdateCompanyTemplate(_ context.Context, _ disclosureapp.UpdateCompanyTemplateRequest) (*disclosureapp.CompanyTemplateWriteResponse, error) {
	return f.updateResp, f.updateErr
}

func (f *fakeDisclosureHandlerService) TransitionCompanyTemplateLifecycle(_ context.Context, _ disclosureapp.TransitionCompanyTemplateLifecycleRequest) (*disclosureapp.CompanyTemplateWriteResponse, error) {
	return f.transitionResp, f.transitionErr
}

type fakeDisclosureInspector struct{}

func (fakeDisclosureInspector) InspectAccessToken(context.Context, string) (*iamapp.AccessTokenClaims, error) {
	return &iamapp.AccessTokenClaims{
		Sub:          "user-001",
		MembershipID: "member-001",
		CompanyID:    "company-001",
	}, nil
}

func (fakeDisclosureInspector) InspectPreCompanyToken(context.Context, string) (*iamapp.PreCompanyTokenClaims, error) {
	return nil, nil
}

type failingDisclosureInspector struct {
	err error
}

func (f failingDisclosureInspector) InspectAccessToken(context.Context, string) (*iamapp.AccessTokenClaims, error) {
	return nil, f.err
}

func (failingDisclosureInspector) InspectPreCompanyToken(context.Context, string) (*iamapp.PreCompanyTokenClaims, error) {
	return nil, nil
}

type fakeDisclosureAuditService struct {
	entries []auditapp.AppendAuditLogRequest
}

func (f *fakeDisclosureAuditService) AppendAuditLog(_ context.Context, req auditapp.AppendAuditLogRequest) error {
	f.entries = append(f.entries, req)
	return nil
}

func TestCreateCompanyTemplate_AppendsAuditLogOnSuccess(t *testing.T) {
	svc := &fakeDisclosureHandlerService{
		createResp: &disclosureapp.CompanyTemplateWriteResponse{
			TypeID:           "type-001",
			CompanyID:        "company-001",
			Name:             "Template A",
			TemplateCategory: "periodic",
			ReviewStatus:     "draft",
		},
	}
	auditSvc := &fakeDisclosureAuditService{}
	h := NewHandler(svc, fakeDisclosureInspector{}, nil, auditSvc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/disclosure-types", strings.NewReader(`{"name":"Template A","template_category":"periodic","change_note":"initial"}`))
	req.Header.Set("Authorization", "Bearer atk_test")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.createCompanyTemplate(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status=%d want 201; body=%s", rr.Code, rr.Body.String())
	}
	if len(auditSvc.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(auditSvc.entries))
	}
	if auditSvc.entries[0].Action != "disclosure.company_template.created" {
		t.Fatalf("action=%q want disclosure.company_template.created", auditSvc.entries[0].Action)
	}
	if auditSvc.entries[0].ResourceID != "type-001" {
		t.Fatalf("resource_id=%q want type-001", auditSvc.entries[0].ResourceID)
	}
}

func TestUpdateCompanyTemplate_AppendsAuditLogOnSuccess(t *testing.T) {
	svc := &fakeDisclosureHandlerService{
		updateResp: &disclosureapp.CompanyTemplateWriteResponse{
			TypeID:           "type-002",
			CompanyID:        "company-001",
			Name:             "Template B",
			TemplateCategory: "irregular",
			ReviewStatus:     "rejected",
		},
	}
	auditSvc := &fakeDisclosureAuditService{}
	h := NewHandler(svc, fakeDisclosureInspector{}, nil, auditSvc)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/company/disclosure-types/type-002", strings.NewReader(`{"name":"Template B","change_note":"revise"}`))
	req.Header.Set("Authorization", "Bearer atk_test")
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("type_id", "type-002")
	rr := httptest.NewRecorder()

	h.updateCompanyTemplate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
	if len(auditSvc.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(auditSvc.entries))
	}
	if auditSvc.entries[0].Action != "disclosure.company_template.updated" {
		t.Fatalf("action=%q want disclosure.company_template.updated", auditSvc.entries[0].Action)
	}
}

func TestCreateCompanyTemplate_DoesNotAppendAuditLogOnForbidden(t *testing.T) {
	svc := &fakeDisclosureHandlerService{
		createErr: perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "forbidden", nil),
	}
	auditSvc := &fakeDisclosureAuditService{}
	h := NewHandler(svc, fakeDisclosureInspector{}, nil, auditSvc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/disclosure-types", strings.NewReader(`{"name":"Template A","template_category":"periodic","change_note":"initial"}`))
	req.Header.Set("Authorization", "Bearer atk_test")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.createCompanyTemplate(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403; body=%s", rr.Code, rr.Body.String())
	}
	if len(auditSvc.entries) != 0 {
		t.Fatalf("expected 0 audit entries on forbidden create, got %d", len(auditSvc.entries))
	}
}

func TestUpdateCompanyTemplate_DoesNotAppendAuditLogOnForbidden(t *testing.T) {
	svc := &fakeDisclosureHandlerService{
		updateErr: perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "forbidden", nil),
	}
	auditSvc := &fakeDisclosureAuditService{}
	h := NewHandler(svc, fakeDisclosureInspector{}, nil, auditSvc)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/company/disclosure-types/type-002", strings.NewReader(`{"name":"Template B","change_note":"revise"}`))
	req.Header.Set("Authorization", "Bearer atk_test")
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("type_id", "type-002")
	rr := httptest.NewRecorder()

	h.updateCompanyTemplate(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403; body=%s", rr.Code, rr.Body.String())
	}
	if len(auditSvc.entries) != 0 {
		t.Fatalf("expected 0 audit entries on forbidden update, got %d", len(auditSvc.entries))
	}
}

func TestTransitionCompanyTemplateLifecycle_AppendsAuditLogOnSuccess(t *testing.T) {
	svc := &fakeDisclosureHandlerService{
		transitionResp: &disclosureapp.CompanyTemplateWriteResponse{
			TypeID:           "type-003",
			CompanyID:        "company-001",
			Name:             "Template C",
			TemplateCategory: "periodic",
			ReviewStatus:     "published",
		},
	}
	auditSvc := &fakeDisclosureAuditService{}
	h := NewHandler(svc, fakeDisclosureInspector{}, nil, auditSvc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/disclosure-types/type-003/lifecycle/publish", strings.NewReader(`{"reason":"approved"}`))
	req.Header.Set("Authorization", "Bearer atk_test")
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("type_id", "type-003")
	req.SetPathValue("action", "publish")
	rr := httptest.NewRecorder()

	h.transitionCompanyTemplateLifecycle(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want 200; body=%s", rr.Code, rr.Body.String())
	}
	if len(auditSvc.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(auditSvc.entries))
	}
	if auditSvc.entries[0].Action != "disclosure.company_template.lifecycle_transition" {
		t.Fatalf("action=%q want disclosure.company_template.lifecycle_transition", auditSvc.entries[0].Action)
	}
	if got := auditSvc.entries[0].Metadata["action"]; got != "publish" {
		t.Fatalf("metadata.action=%v want publish", got)
	}
}

func TestTransitionCompanyTemplateLifecycle_DoesNotAppendAuditLogOnFailure(t *testing.T) {
	svc := &fakeDisclosureHandlerService{
		transitionErr: perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "invalid transition", nil),
	}
	auditSvc := &fakeDisclosureAuditService{}
	h := NewHandler(svc, fakeDisclosureInspector{}, nil, auditSvc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/disclosure-types/type-004/lifecycle/publish", strings.NewReader(`{"reason":"approved"}`))
	req.Header.Set("Authorization", "Bearer atk_test")
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("type_id", "type-004")
	req.SetPathValue("action", "publish")
	rr := httptest.NewRecorder()

	h.transitionCompanyTemplateLifecycle(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status=%d want 409; body=%s", rr.Code, rr.Body.String())
	}
	if len(auditSvc.entries) != 0 {
		t.Fatalf("expected 0 audit entries on failure, got %d", len(auditSvc.entries))
	}
}

func TestCreateCompanyTemplate_DoesNotAppendAuditLogWhenTokenInvalid(t *testing.T) {
	svc := &fakeDisclosureHandlerService{}
	auditSvc := &fakeDisclosureAuditService{}
	h := NewHandler(
		svc,
		failingDisclosureInspector{err: perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid token", nil)},
		nil,
		auditSvc,
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/disclosure-types", strings.NewReader(`{"name":"Template A","template_category":"periodic"}`))
	req.Header.Set("Authorization", "Bearer bad_token")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.createCompanyTemplate(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401; body=%s", rr.Code, rr.Body.String())
	}
	if len(auditSvc.entries) != 0 {
		t.Fatalf("expected 0 audit entries when token inspection fails, got %d", len(auditSvc.entries))
	}
}

// Cross-tenant audit semantics: service returns not-found (tenant mismatch) → no audit.

func TestTransitionCompanyTemplateLifecycle_DoesNotAppendAuditLogOnCrossTenantNotFound(t *testing.T) {
	svc := &fakeDisclosureHandlerService{
		// Service returns NotFound because the type belongs to a different company.
		transitionErr: perr.NewHTTPError(http.StatusNotFound, perr.CodeDataScopeDenied, "type not found in company scope", nil),
	}
	auditSvc := &fakeDisclosureAuditService{}
	h := NewHandler(svc, fakeDisclosureInspector{}, nil, auditSvc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/disclosure-types/type-other-company/lifecycle/publish", strings.NewReader(`{"reason":"cross-tenant attempt"}`))
	req.Header.Set("Authorization", "Bearer atk_test")
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("type_id", "type-other-company")
	req.SetPathValue("action", "publish")
	rr := httptest.NewRecorder()

	h.transitionCompanyTemplateLifecycle(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404; body=%s", rr.Code, rr.Body.String())
	}
	if len(auditSvc.entries) != 0 {
		t.Fatalf("expected 0 audit entries on cross-tenant lifecycle attempt, got %d", len(auditSvc.entries))
	}
}

func TestUpdateCompanyTemplate_DoesNotAppendAuditLogOnCrossTenantNotFound(t *testing.T) {
	svc := &fakeDisclosureHandlerService{
		// Service returns NotFound because the type is scoped to a different company.
		updateErr: perr.NewHTTPError(http.StatusNotFound, perr.CodeDataScopeDenied, "type not found in company scope", nil),
	}
	auditSvc := &fakeDisclosureAuditService{}
	h := NewHandler(svc, fakeDisclosureInspector{}, nil, auditSvc)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/company/disclosure-types/type-other-company", strings.NewReader(`{"name":"Hijack attempt"}`))
	req.Header.Set("Authorization", "Bearer atk_test")
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("type_id", "type-other-company")
	rr := httptest.NewRecorder()

	h.updateCompanyTemplate(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404; body=%s", rr.Code, rr.Body.String())
	}
	if len(auditSvc.entries) != 0 {
		t.Fatalf("expected 0 audit entries on cross-tenant update attempt, got %d", len(auditSvc.entries))
	}
}

func TestTransitionCompanyTemplateLifecycle_DoesNotAppendAuditLogOnForbidden(t *testing.T) {
	svc := &fakeDisclosureHandlerService{
		transitionErr: perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "forbidden", nil),
	}
	auditSvc := &fakeDisclosureAuditService{}
	h := NewHandler(svc, fakeDisclosureInspector{}, nil, auditSvc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/company/disclosure-types/type-005/lifecycle/publish", strings.NewReader(`{"reason":"no permission"}`))
	req.Header.Set("Authorization", "Bearer atk_test")
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("type_id", "type-005")
	req.SetPathValue("action", "publish")
	rr := httptest.NewRecorder()

	h.transitionCompanyTemplateLifecycle(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d want 403; body=%s", rr.Code, rr.Body.String())
	}
	if len(auditSvc.entries) != 0 {
		t.Fatalf("expected 0 audit entries on forbidden lifecycle transition, got %d", len(auditSvc.entries))
	}
}
