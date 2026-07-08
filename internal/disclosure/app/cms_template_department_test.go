package app_test

import (
	"context"
	"net/http"
	"testing"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type cmsTemplateDeptAuth struct {
	permissions []string
}

func (a *cmsTemplateDeptAuth) GetEffectiveAccess(_ context.Context, _, _ string) (*authapp.EffectiveAccessSummary, error) {
	return &authapp.EffectiveAccessSummary{Permissions: a.permissions}, nil
}

func (a *cmsTemplateDeptAuth) Authorize(_ context.Context, _ authapp.AuthorizeRequest) (*authapp.AuthorizeDecision, error) {
	return nil, nil
}

func (a *cmsTemplateDeptAuth) AuthorizeBatch(_ context.Context, _ authapp.AuthorizeBatchRequest) (*authapp.AuthorizeBatchResponse, error) {
	return nil, nil
}

func newCMSTemplateDeptService(perms []string) disclosureapp.Service {
	return disclosureapp.NewService(inmemory.NewRepository(), &cmsTemplateDeptAuth{permissions: perms}, idgen.UUIDv7Generator{})
}

func TestCmsListTemplateDepartmentsCatalog_Success(t *testing.T) {
	svc := newCMSTemplateDeptService([]string{"platform.cms.view", "cms.template.read"})
	resp, err := svc.CmsListTemplateDepartmentsCatalog(context.Background(), disclosureapp.ListDisplayGroupsRequest{
		Subject: disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Items) < 4 {
		t.Fatalf("expected seeded departments, got %d", len(resp.Items))
	}
}

func TestCmsCreateTemplateDepartment_Success(t *testing.T) {
	svc := newCMSTemplateDeptService([]string{"platform.cms.view", "cms.template.write"})
	created, err := svc.CmsCreateTemplateDepartment(context.Background(), disclosureapp.CmsTemplateDepartmentCreateRequest{
		Subject: disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
		Code:    "tpl_dept_qa_default",
		Name:    "Phòng QA Default Template",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.DepartmentCode != "tpl_dept_qa_default" {
		t.Errorf("code=%s", created.DepartmentCode)
	}
	if created.DepartmentName != "Phòng QA Default Template" {
		t.Errorf("name=%s", created.DepartmentName)
	}
}

func TestCmsCreateTemplateDepartment_DuplicateCode(t *testing.T) {
	svc := newCMSTemplateDeptService([]string{"platform.cms.view", "cms.template.write"})
	_, err := svc.CmsCreateTemplateDepartment(context.Background(), disclosureapp.CmsTemplateDepartmentCreateRequest{
		Subject: disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
		Code:    "dept-001",
		Name:    "Duplicate",
	})
	if err == nil {
		t.Fatal("expected conflict error")
	}
	httpErr, ok := err.(*perr.HTTPError)
	if !ok || httpErr.HTTPStatus != http.StatusConflict {
		t.Fatalf("expected 409 conflict, got %v", err)
	}
}

func TestCmsCreateTemplateDepartment_Validation(t *testing.T) {
	svc := newCMSTemplateDeptService([]string{"platform.cms.view", "cms.template.write"})
	_, err := svc.CmsCreateTemplateDepartment(context.Background(), disclosureapp.CmsTemplateDepartmentCreateRequest{
		Subject: disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
		Code:    "tpl_dept_x",
		Name:    "",
	})
	if err == nil {
		t.Fatal("expected validation error for empty name")
	}
}
