package app

import (
	"context"
	"math"
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// Regression: the handler clamps page=100000000000000000000 to MaxInt (Atoi), and
// (page-1)*page_size overflowed int, wrapped negative, passed the start > total guard
// and panicked on enriched[start:end] (no recover middleware: the client got no response).
func TestListDeadlineAlerts_hugePageReturnsEmptyPage(t *testing.T) {
	cases := []struct {
		name     string
		page     int
		pageSize int
		status   string
	}{
		{"overflow page", 500000000000000000, 20, ""},
		{"max int page", math.MaxInt, 100, ""},
		{"wraps to zero page", 1<<62 + 1, 4, ""},
		{"first page past end", 2, 2, ""},
		{"overflow page with status filter", math.MaxInt, 20, "UPCOMING"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubRepo{rows: []AlertRow{
				{CompanyID: "c1", RecordID: "r-a", Title: "A", RecordStatus: "Draft", PlannedDate: "2026-09-10"},
				{CompanyID: "c1", RecordID: "r-b", Title: "B", RecordStatus: "Draft", PlannedDate: "2026-09-20"},
			}}
			svc := NewService(repo, allowAuthSvc(), disclosureapp.NewDeadlineCalculator(disclosureapp.NewHolidayCalendarFileProvider("configs/non_trading_days")))
			svc.(*service).now = func() time.Time { return time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC) }

			resp, err := svc.ListDeadlineAlerts(context.Background(), ListDeadlineAlertsRequest{
				Subject:  Subject{UserID: "u1", MembershipID: "m_admin_001", CompanyID: "c_001"},
				Status:   tc.status,
				Page:     tc.page,
				PageSize: tc.pageSize,
			})
			if err != nil {
				t.Fatal(err)
			}
			if resp.Total != 2 || resp.Items == nil || len(resp.Items) != 0 {
				t.Fatalf("total=%d items=%v want 2 / []", resp.Total, resp.Items)
			}
			if resp.Page != tc.page || resp.PageSize != tc.pageSize {
				t.Fatalf("page=%d page_size=%d want %d / %d", resp.Page, resp.PageSize, tc.page, tc.pageSize)
			}
		})
	}
}
