package app

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

type failingConfigRepo struct {
	stubRepo
	badErr error
}

func (r *failingConfigRepo) GetTypeDeadlineConfig(_ context.Context, _, typeID string) (*disclosureapp.TemplateDeadlineConfig, error) {
	if typeID == "t-bad" {
		return nil, r.badErr
	}
	return nil, nil
}

func twoRowRepo(badErr error) *failingConfigRepo {
	return &failingConfigRepo{badErr: badErr, stubRepo: stubRepo{rows: []AlertRow{
		{CompanyID: "c1", RecordID: "r-bad", TypeID: "t-bad", Title: "Bad", RecordStatus: "Draft"},
		{CompanyID: "c1", RecordID: "r-ok", Title: "Has due", RecordStatus: "Draft", PlannedDate: "2026-09-01"},
	}}}
}

// DLA-05/DUE-05: one row whose due date cannot be resolved must not fail the whole list (and
// the dashboard that shares the enrichment); the row is skipped and logged.
func TestListDeadlineAlerts_OneUnresolvableRowDoesNotFailList(t *testing.T) {
	repo := twoRowRepo(fmt.Errorf("due: %w", disclosureapp.ErrHolidayYearMissing))
	svc := NewService(repo, allowAuthSvc(), disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
	svc.(*service).now = func() time.Time { return time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC) }

	resp, err := svc.ListDeadlineAlerts(context.Background(), ListDeadlineAlertsRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001"},
	})
	if err != nil {
		t.Fatalf("one bad row failed the list: %v", err)
	}
	if resp.Total != 1 || resp.Items[0].RecordID != "r-ok" {
		t.Fatalf("want only r-ok, got %+v", resp)
	}
}

// API-01: an infrastructure error (DB down, cancelled request) is not a per-row problem; the
// list must fail as before instead of returning a silently partial 200.
func TestListDeadlineAlerts_InfrastructureErrorStillFails(t *testing.T) {
	svc := NewService(twoRowRepo(errors.New("dial tcp: connection refused")), allowAuthSvc(), disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
	svc.(*service).now = func() time.Time { return time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC) }
	if _, err := svc.ListDeadlineAlerts(context.Background(), ListDeadlineAlertsRequest{
		Subject: Subject{UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001"},
	}); err == nil {
		t.Fatal("infrastructure error must fail the list")
	}
}
