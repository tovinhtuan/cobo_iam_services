package app_test

import (
	"context"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func TestSlugifyTemplateDepartmentCode_PreservesUnicodeNameSeparately(t *testing.T) {
	code := disclosureapp.SlugifyTemplateDepartmentCode("Phòng QA Default Tên")
	if code != "tpl_dept_phong_qa_default_ten" {
		t.Fatalf("code=%q", code)
	}
}

func TestCmsCreateTemplateDepartment_NameOnlyAutoCode(t *testing.T) {
	svc := newCMSTemplateDeptService([]string{"platform.cms.view", "cms.template.write"})
	created, err := svc.CmsCreateTemplateDepartment(context.Background(), disclosureapp.CmsTemplateDepartmentCreateRequest{
		Subject: disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
		Name:    "Phòng QA Default Tên",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.DepartmentName != "Phòng QA Default Tên" {
		t.Fatalf("name=%q", created.DepartmentName)
	}
	if created.DepartmentCode != "tpl_dept_phong_qa_default_ten" {
		t.Fatalf("code=%q", created.DepartmentCode)
	}
}

func TestCmsCreateTemplateDepartment_DuplicateName(t *testing.T) {
	svc := newCMSTemplateDeptService([]string{"platform.cms.view", "cms.template.write"})
	_, err := svc.CmsCreateTemplateDepartment(context.Background(), disclosureapp.CmsTemplateDepartmentCreateRequest{
		Subject: disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c1"},
		Name:    "Phòng Pháp chế",
	})
	if err == nil {
		t.Fatal("expected duplicate name error for seeded department")
	}
}
