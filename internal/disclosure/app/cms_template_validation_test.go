package app

import (
	"context"
	"net/http"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type companyTemplateValidationRepo struct {
	Repository
}

func (r *companyTemplateValidationRepo) ListActiveDeadlineRuleCatalog(_ context.Context) ([]DeadlineRuleCatalogDTO, error) {
	return defaultDeadlineRuleCatalog(), nil
}

func (r *companyTemplateValidationRepo) CreateCompanyTemplate(_ context.Context, req CreateCompanyTemplateRequest) (*CompanyTemplateWriteResponse, error) {
	return &CompanyTemplateWriteResponse{
		TypeID:           "type-001",
		CompanyID:        req.Subject.CompanyID,
		Name:             req.Name,
		TemplateCategory: req.TemplateCategory,
		DeadlineRule:     req.DeadlineRule,
		Periodicity:      req.Periodicity,
		ReviewStatus:     "draft",
	}, nil
}

func newCompanyTemplateValidationService() Service {
	return NewService(&companyTemplateValidationRepo{}, &fakeAuthService{
		permissions: []string{permissionLegacyTemplateManage},
	}, idgen.UUIDv7Generator{})
}

func TestCreateCompanyTemplate_RejectsMissingPeriodicPeriodicity(t *testing.T) {
	svc := newCompanyTemplateValidationService()
	_, err := svc.CreateCompanyTemplate(context.Background(), CreateCompanyTemplateRequest{
		Subject:          Subject{CompanyID: "company-001", MembershipID: "member-001"},
		Name:             "Template A",
		TemplateCategory: TemplateCategoryPeriodic,
		DeadlineRule:     "T+5",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	herr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T", err)
	}
	if herr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("status=%d want %d", herr.HTTPStatus, http.StatusBadRequest)
	}
}

func TestCreateCompanyTemplate_AcceptsIrregularWithoutPeriodicity(t *testing.T) {
	svc := newCompanyTemplateValidationService()
	resp, err := svc.CreateCompanyTemplate(context.Background(), CreateCompanyTemplateRequest{
		Subject:          Subject{CompanyID: "company-001", MembershipID: "member-001"},
		Name:             "Template B",
		TemplateCategory: TemplateCategoryIrregular,
		DeadlineRule:     "T+1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TemplateCategory != TemplateCategoryIrregular {
		t.Fatalf("template_category=%q want %q", resp.TemplateCategory, TemplateCategoryIrregular)
	}
}

func TestCreateCompanyTemplate_RejectsDeadlineRuleOutsideCatalog(t *testing.T) {
	svc := newCompanyTemplateValidationService()
	_, err := svc.CreateCompanyTemplate(context.Background(), CreateCompanyTemplateRequest{
		Subject:          Subject{CompanyID: "company-001", MembershipID: "member-001"},
		Name:             "Template C",
		TemplateCategory: TemplateCategoryIrregular,
		DeadlineRule:     "Trong vòng 5 ngày kể từ ngày sự kiện",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	herr, ok := err.(*perr.HTTPError)
	if !ok {
		t.Fatalf("expected *perr.HTTPError, got %T", err)
	}
	if herr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("status=%d want %d", herr.HTTPStatus, http.StatusBadRequest)
	}
}

func TestCreateCompanyTemplate_AcceptsCatalogDatePattern(t *testing.T) {
	svc := newCompanyTemplateValidationService()
	resp, err := svc.CreateCompanyTemplate(context.Background(), CreateCompanyTemplateRequest{
		Subject:          Subject{CompanyID: "company-001", MembershipID: "member-001"},
		Name:             "Template D",
		TemplateCategory: TemplateCategoryIrregular,
		DeadlineRule:     "15/03",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.DeadlineRule != "15/03" {
		t.Fatalf("deadline_rule=%q want 15/03", resp.DeadlineRule)
	}
}
