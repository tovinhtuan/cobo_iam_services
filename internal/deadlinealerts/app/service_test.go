package app

import (
	"context"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	authinmem "github.com/cobo/cobo_iam_services/internal/authorization/infra/inmemory"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

type stubRepo struct {
	rows []AlertRow
}

func (s *stubRepo) ListRows(_ context.Context, _ string, _ DeadlineAlertAccessScope) ([]AlertRow, error) {
	return s.rows, nil
}

func (s *stubRepo) GetCompanyDeadlineContext(_ context.Context, companyID string) (disclosureapp.CompanyDeadlineContext, error) {
	return disclosureapp.CompanyDeadlineContext{CompanyID: companyID, CurrentYear: 2026}, nil
}

func (s *stubRepo) GetCompanyTypeDeadlineContext(_ context.Context, companyID, _ string) (disclosureapp.CompanyDeadlineContext, error) {
	return disclosureapp.CompanyDeadlineContext{CompanyID: companyID, CurrentYear: 2026}, nil
}

func (s *stubRepo) GetTypeDeadlineConfig(_ context.Context, _, _ string) (*disclosureapp.TemplateDeadlineConfig, error) {
	return nil, nil
}

func (s *stubRepo) HasDisclosureRecord(_ context.Context, _, _ string) (bool, error) {
	return true, nil
}

func (s *stubRepo) ConfirmDeadlineAlert(_ context.Context, _, _, _, _, _ string, _ time.Time) error {
	return nil
}

func (s *stubRepo) GetWorkflowInstanceByRecord(_ context.Context, _, _ string) (*WorkflowInstanceRow, error) {
	return nil, nil
}

func (s *stubRepo) GetEffectiveWorkflowSnapshot(_ context.Context, _, _ string) ([]workflowapp.StepSnapshot, error) {
	return nil, nil
}

func (s *stubRepo) ListStepStates(_ context.Context, _ string) (map[string]StepRuntimeState, error) {
	return map[string]StepRuntimeState{}, nil
}

func (s *stubRepo) UpsertStepCompleted(_ context.Context, _, _, _, _ string, _ time.Time) error {
	return nil
}

func (s *stubRepo) UpsertStepIncomplete(_ context.Context, _, _, _, _, _ string, _ int, _ time.Time) error {
	return nil
}

func allowAuthSvc() authapp.Service {
	authRepo := authinmem.NewRepository()
	return authapp.NewService(authinmem.NewResolver(authRepo), authinmem.NewChecker(), authRepo)
}

func TestListDeadlineAlerts_filtersDraftAndPaginates(t *testing.T) {
	repo := &stubRepo{rows: []AlertRow{
		{CompanyID: "c1", RecordID: "r-draft", Title: "Draft", RecordStatus: "draft", PlannedDate: "2026-06-01"},
		{CompanyID: "c1", RecordID: "r-live", Title: "Live", RecordStatus: "submitted", PlannedDate: "2026-06-10"},
	}}
	svc := NewService(repo, allowAuthSvc(), disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
	svc.(*service).now = func() time.Time { return time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC) }

	resp, err := svc.ListDeadlineAlerts(context.Background(), ListDeadlineAlertsRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001"},
		Page:    1, PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 || len(resp.Items) != 1 || resp.Items[0].RecordID != "r-live" {
		t.Fatalf("got %+v", resp)
	}
	if resp.Items[0].Status != "UPCOMING" {
		t.Fatalf("status %s", resp.Items[0].Status)
	}
}

func TestListDeadlineAlerts_usesAdHocProposedDueDate(t *testing.T) {
	repo := &stubRepo{rows: []AlertRow{
		{
			CompanyID: "c1", RecordID: "r1", Title: "Ad-hoc BCTC",
			RecordStatus: "published", AdHocDeadlineDate: "2026-05-27",
		},
	}}
	svc := NewService(repo, allowAuthSvc(), disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
	svc.(*service).now = func() time.Time { return time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC) }

	resp, err := svc.ListDeadlineAlerts(context.Background(), ListDeadlineAlertsRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 || resp.Items[0].DueDate != "2026-05-27" || resp.Items[0].Status != "PENDING_CONFIRM" {
		t.Fatalf("got %+v", resp)
	}
}

func TestListDeadlineAlerts_confirmedRecordBecomesDone(t *testing.T) {
	confirmedAt := time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC)
	repo := &stubRepo{rows: []AlertRow{
		{
			CompanyID: "c1", RecordID: "r1", Title: "Ad-hoc BCTC",
			RecordStatus: "published", AdHocDeadlineDate: "2026-05-27",
			ConfirmedBy: "m_admin_001", ConfirmedAt: &confirmedAt,
		},
	}}
	svc := NewService(repo, allowAuthSvc(), disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
	svc.(*service).now = func() time.Time { return time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC) }

	resp, err := svc.ListDeadlineAlerts(context.Background(), ListDeadlineAlertsRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 || resp.Items[0].Status != "DONE" {
		t.Fatalf("got %+v", resp)
	}
}

func TestListDeadlineAlerts_populatesActiveDepartmentsFromRow(t *testing.T) {
	repo := &stubRepo{rows: []AlertRow{
		{
			CompanyID:               "c1",
			RecordID:                "r-live",
			Title:                   "Live",
			RecordStatus:            "submitted",
			PlannedDate:             "2026-06-10",
			CurrentStepCode:         "focal_confirm",
			CurrentStepDepartment:   "Phòng CBTT",
			CurrentStepName:         "Phòng ban lập hồ sơ",
		},
	}}
	svc := NewService(repo, allowAuthSvc(), disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
	svc.(*service).now = func() time.Time { return time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC) }

	resp, err := svc.ListDeadlineAlerts(context.Background(), ListDeadlineAlertsRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001"},
		Page:    1, PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("got %+v", resp)
	}
	if len(resp.Items[0].ActiveDepartments) != 1 || resp.Items[0].ActiveDepartments[0] != "Phòng CBTT" {
		t.Fatalf("active departments %+v", resp.Items[0].ActiveDepartments)
	}
	if resp.Items[0].CurrentStepName != "Phòng ban lập hồ sơ" {
		t.Fatalf("current step name %q", resp.Items[0].CurrentStepName)
	}
}

func TestListDeadlineAlerts_forbiddenWithoutPermission(t *testing.T) {
	repo := &stubRepo{rows: []AlertRow{
		{CompanyID: "c1", RecordID: "r1", Title: "X", RecordStatus: "submitted", PlannedDate: "2026-06-01"},
	}}
	authRepo := authinmem.NewRepository()
	authSvc := authapp.NewService(authinmem.NewResolver(authRepo), authinmem.NewChecker(), authRepo)
	svc := NewService(repo, authSvc, disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))

	_, err := svc.ListDeadlineAlerts(context.Background(), ListDeadlineAlertsRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m_001", CompanyID: "c_001"},
	})
	if err == nil {
		t.Fatal("expected forbidden")
	}
}

func scopedAuthSvc() authapp.Service {
	authRepo := authinmem.NewRepository()
	authRepo.Permissions["m_staff@c_001"] = []string{"deadline.view"}
	authRepo.Departments["m_staff@c_001"] = []authapp.DepartmentScope{
		{DepartmentID: "d_legal", DepartmentName: "Legal"},
	}
	authRepo.Assignments["m_staff@c_001"] = []authapp.ResourceAssignment{
		{ResourceType: "disclosure_record", ResourceID: "r-assigned"},
	}
	return authapp.NewService(authinmem.NewResolver(authRepo), authinmem.NewChecker(), authRepo)
}

func TestListDeadlineAlerts_scopedUserDoesNotSeeAllCompanyRows(t *testing.T) {
	repo := &stubRepo{rows: []AlertRow{
		{CompanyID: "c_001", RecordID: "r-legal", Title: "Legal dept", RecordStatus: "submitted", PlannedDate: "2026-06-10", RecordDepartmentID: "d_legal"},
		{CompanyID: "c_001", RecordID: "r-ir", Title: "IR dept", RecordStatus: "submitted", PlannedDate: "2026-06-11", RecordDepartmentID: "d_ir"},
		{CompanyID: "c_001", RecordID: "r-assigned", Title: "Assigned", RecordStatus: "submitted", PlannedDate: "2026-06-12", RecordDepartmentID: "d_ir"},
	}}
	svc := NewService(repo, scopedAuthSvc(), disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
	svc.(*service).now = func() time.Time { return time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC) }

	resp, err := svc.ListDeadlineAlerts(context.Background(), ListDeadlineAlertsRequest{
		Subject: Subject{UserID: "u_staff", MembershipID: "m_staff", CompanyID: "c_001"},
		Page:    1, PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 2 {
		t.Fatalf("expected 2 scoped rows, got total=%d items=%d", resp.Total, len(resp.Items))
	}
}

func TestListDeadlineAlerts_adminViewAllStillSeesAllRows(t *testing.T) {
	repo := &stubRepo{rows: []AlertRow{
		{CompanyID: "c_001", RecordID: "r-legal", Title: "Legal", RecordStatus: "submitted", PlannedDate: "2026-06-10", RecordDepartmentID: "d_legal"},
		{CompanyID: "c_001", RecordID: "r-ir", Title: "IR", RecordStatus: "submitted", PlannedDate: "2026-06-11", RecordDepartmentID: "d_ir"},
	}}
	svc := NewService(repo, allowAuthSvc(), disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
	svc.(*service).now = func() time.Time { return time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC) }

	resp, err := svc.ListDeadlineAlerts(context.Background(), ListDeadlineAlertsRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 2 {
		t.Fatalf("admin expected 2 rows, got %d", resp.Total)
	}
}

