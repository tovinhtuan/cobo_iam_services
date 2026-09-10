package app

import (
	"context"
	"testing"
)

func TestEnrichStepDepartmentResolution_Matched(t *testing.T) {
	repo := &stubRepo{
		departments: []DeadlineAlertFilterOptionDTO{
			{ID: "dept-hr", Code: "HR", Name: "Phòng Nhân sự (công ty)"},
		},
		templateDepts: []DeadlineAlertFilterOptionDTO{
			{ID: "tpl_dept_phong_nhan_su", Code: "tpl_dept_phong_nhan_su", Name: "Phòng Nhân sự"},
		},
		adminAvailable:    true,
		adminAvailableSet: true,
	}
	svc := &service{repo: repo}
	resp := &ListDeadlineStepsResponse{Steps: []DeadlineStepDTO{
		{StepCode: "s1", DepartmentID: "dept-hr", DepartmentName: "dept-hr"},
	}}
	if err := svc.enrichStepDepartmentNames(context.Background(), "c1", resp); err != nil {
		t.Fatal(err)
	}
	step := resp.Steps[0]
	if step.CompanyDepartmentResolution != CompanyDepartmentResolutionMatched {
		t.Fatalf("resolution=%s want MATCHED", step.CompanyDepartmentResolution)
	}
	if step.ReminderFallbackRecipientType != "" {
		t.Fatalf("fallback=%q want empty", step.ReminderFallbackRecipientType)
	}
	if step.CompanyAdminRecipientAvailable != nil {
		t.Fatalf("admin availability should be omitted when matched")
	}
	if step.DepartmentName == "" {
		t.Fatal("expected department display name")
	}
}

func TestEnrichStepDepartmentResolution_MatchedByCode(t *testing.T) {
	repo := &stubRepo{
		departments: []DeadlineAlertFilterOptionDTO{
			{ID: "uuid-1", Code: "tpl_dept_phong_nhan_su", Name: "HR Local"},
		},
	}
	svc := &service{repo: repo}
	resp := &ListDeadlineStepsResponse{Steps: []DeadlineStepDTO{
		{StepCode: "s1", DepartmentID: "tpl_dept_phong_nhan_su", DepartmentName: "tpl_dept_phong_nhan_su"},
	}}
	if err := svc.enrichStepDepartmentNames(context.Background(), "c1", resp); err != nil {
		t.Fatal(err)
	}
	if resp.Steps[0].CompanyDepartmentResolution != CompanyDepartmentResolutionMatched {
		t.Fatalf("got %s", resp.Steps[0].CompanyDepartmentResolution)
	}
}

func TestEnrichStepDepartmentResolution_MissingWithAdmin(t *testing.T) {
	repo := &stubRepo{
		departments: []DeadlineAlertFilterOptionDTO{},
		templateDepts: []DeadlineAlertFilterOptionDTO{
			{ID: "tpl_dept_phong_nhan_su", Code: "tpl_dept_phong_nhan_su", Name: "Phòng Nhân sự"},
		},
		adminAvailable:    true,
		adminAvailableSet: true,
	}
	svc := &service{repo: repo}
	resp := &ListDeadlineStepsResponse{Steps: []DeadlineStepDTO{
		{StepCode: "s1", DepartmentID: "tpl_dept_phong_nhan_su", DepartmentName: "tpl_dept_phong_nhan_su"},
	}}
	if err := svc.enrichStepDepartmentNames(context.Background(), "c1", resp); err != nil {
		t.Fatal(err)
	}
	step := resp.Steps[0]
	if step.DepartmentName != "Phòng Nhân sự" {
		t.Fatalf("catalog label=%q", step.DepartmentName)
	}
	if step.CompanyDepartmentResolution != CompanyDepartmentResolutionMissing {
		t.Fatalf("resolution=%s", step.CompanyDepartmentResolution)
	}
	if step.ReminderFallbackRecipientType != ReminderFallbackRecipientTypeCompanyAdmin {
		t.Fatalf("fallback=%s", step.ReminderFallbackRecipientType)
	}
	if step.CompanyAdminRecipientAvailable == nil || !*step.CompanyAdminRecipientAvailable {
		t.Fatal("expected company_admin_recipient_available=true")
	}
}

func TestEnrichStepDepartmentResolution_MissingZeroAdmin(t *testing.T) {
	repo := &stubRepo{
		templateDepts: []DeadlineAlertFilterOptionDTO{
			{ID: "tpl_dept_phong_nhan_su", Code: "tpl_dept_phong_nhan_su", Name: "Phòng Nhân sự"},
		},
		adminAvailable:    false,
		adminAvailableSet: true,
	}
	svc := &service{repo: repo}
	resp := &ListDeadlineStepsResponse{Steps: []DeadlineStepDTO{
		{StepCode: "s1", DepartmentID: "tpl_dept_phong_nhan_su", DepartmentName: "tpl_dept_phong_nhan_su"},
	}}
	if err := svc.enrichStepDepartmentNames(context.Background(), "c1", resp); err != nil {
		t.Fatal(err)
	}
	step := resp.Steps[0]
	if step.CompanyDepartmentResolution != CompanyDepartmentResolutionMissing {
		t.Fatalf("resolution=%s", step.CompanyDepartmentResolution)
	}
	if step.CompanyAdminRecipientAvailable == nil || *step.CompanyAdminRecipientAvailable {
		t.Fatal("expected company_admin_recipient_available=false")
	}
}

func TestEnrichStepDepartmentResolution_InactiveTreatedAsMissing(t *testing.T) {
	// ListCompanyDepartments only returns active rows — inactive never appears → MISSING.
	repo := &stubRepo{
		departments: []DeadlineAlertFilterOptionDTO{},
		templateDepts: []DeadlineAlertFilterOptionDTO{
			{ID: "dept-inactive", Code: "dept-inactive", Name: "Phòng cũ"},
		},
		adminAvailable:    true,
		adminAvailableSet: true,
	}
	svc := &service{repo: repo}
	resp := &ListDeadlineStepsResponse{Steps: []DeadlineStepDTO{
		{StepCode: "s1", DepartmentID: "dept-inactive", DepartmentName: "dept-inactive"},
	}}
	if err := svc.enrichStepDepartmentNames(context.Background(), "c1", resp); err != nil {
		t.Fatal(err)
	}
	if resp.Steps[0].CompanyDepartmentResolution != CompanyDepartmentResolutionMissing {
		t.Fatalf("got %s", resp.Steps[0].CompanyDepartmentResolution)
	}
}

func TestEnrichStepDepartmentResolution_CatalogLabelNotCompanyMatch(t *testing.T) {
	repo := &stubRepo{
		departments: []DeadlineAlertFilterOptionDTO{
			{ID: "other", Code: "OTHER", Name: "Phòng khác"},
		},
		templateDepts: []DeadlineAlertFilterOptionDTO{
			{ID: "tpl_dept_phong_nhan_su", Code: "tpl_dept_phong_nhan_su", Name: "Phòng Nhân sự"},
		},
		adminAvailable:    true,
		adminAvailableSet: true,
	}
	svc := &service{repo: repo}
	resp := &ListDeadlineStepsResponse{Steps: []DeadlineStepDTO{
		{StepCode: "s1", DepartmentID: "tpl_dept_phong_nhan_su", DepartmentName: "tpl_dept_phong_nhan_su"},
	}}
	if err := svc.enrichStepDepartmentNames(context.Background(), "c1", resp); err != nil {
		t.Fatal(err)
	}
	step := resp.Steps[0]
	if step.DepartmentName != "Phòng Nhân sự" {
		t.Fatalf("name=%q", step.DepartmentName)
	}
	if step.CompanyDepartmentResolution != CompanyDepartmentResolutionMissing {
		t.Fatalf("resolution=%s — catalog label must not imply company match", step.CompanyDepartmentResolution)
	}
}

func TestEnrichStepDepartmentResolution_NoConfig(t *testing.T) {
	repo := &stubRepo{adminAvailableSet: true, adminAvailable: true}
	svc := &service{repo: repo}
	resp := &ListDeadlineStepsResponse{Steps: []DeadlineStepDTO{
		{StepCode: "s1", DepartmentID: "", DepartmentName: ""},
	}}
	if err := svc.enrichStepDepartmentNames(context.Background(), "c1", resp); err != nil {
		t.Fatal(err)
	}
	step := resp.Steps[0]
	if step.CompanyDepartmentResolution != CompanyDepartmentResolutionNoConfig {
		t.Fatalf("got %s", step.CompanyDepartmentResolution)
	}
	if step.ReminderFallbackRecipientType != "" {
		t.Fatalf("fallback should be empty for NO_CONFIG")
	}
	if step.CompanyAdminRecipientAvailable != nil {
		t.Fatal("admin availability must not drive NO_CONFIG warning")
	}
}

func TestCompanyDepartmentTokenMatched(t *testing.T) {
	depts := []DeadlineAlertFilterOptionDTO{
		{ID: "id-1", Code: "CODE_A", Name: "A"},
	}
	if !companyDepartmentTokenMatched(depts, "id-1") {
		t.Fatal("id match")
	}
	if !companyDepartmentTokenMatched(depts, "CODE_A") {
		t.Fatal("code match")
	}
	if companyDepartmentTokenMatched(depts, "A") {
		t.Fatal("name must not match")
	}
	if companyDepartmentTokenMatched(depts, "") {
		t.Fatal("empty")
	}
}
