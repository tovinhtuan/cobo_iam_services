package app

import (
	"context"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	authinmem "github.com/cobo/cobo_iam_services/internal/authorization/infra/inmemory"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

type stubRepo struct {
	rows []AlertRow
}

func (s *stubRepo) ListRows(_ context.Context, _ string) ([]AlertRow, error) {
	return s.rows, nil
}

func (s *stubRepo) GetCompanyDeadlineContext(_ context.Context, companyID string) (disclosureapp.CompanyDeadlineContext, error) {
	return disclosureapp.CompanyDeadlineContext{CompanyID: companyID, CurrentYear: 2026}, nil
}

func (s *stubRepo) GetTypeDeadlineConfig(_ context.Context, _, _ string) (*disclosureapp.TemplateDeadlineConfig, error) {
	return nil, nil
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
	if len(resp.Items) != 1 || resp.Items[0].DueDate != "2026-05-27" || resp.Items[0].Status != "DONE" {
		t.Fatalf("got %+v", resp)
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
