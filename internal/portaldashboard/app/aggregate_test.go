package app

import (
	"testing"
	"time"

	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
	"github.com/cobo/cobo_iam_services/internal/portaldashboard/domain"
)

func TestBucketOverdueAge(t *testing.T) {
	ref := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	items := []deadlinealertsapp.DeadlineAlertDTO{
		{DueDate: "2026-07-09"}, // 1 day
		{DueDate: "2026-07-06"}, // 4 days
		{DueDate: "2026-06-25"}, // 15 days
	}
	buckets := bucketOverdueAge(items, ref)
	if buckets[0].Count != 1 || buckets[1].Count != 1 || buckets[3].Count != 1 {
		t.Fatalf("buckets: %+v", buckets)
	}
}

func TestBuildOverview_onTimeUnavailable(t *testing.T) {
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	dr, _ := domain.ParseRange(domain.ParseRangeInput{Range: "30d", Now: now})
	resp := buildOverview(
		domain.CompanyBrief{ID: "c1"},
		dr,
		deadlineFetch{overdueTotal: 2, overdue: []deadlinealertsapp.DeadlineAlertDTO{{RecordID: "r1", DueDate: "2026-07-01", Status: "OVERDUE"}}},
		adHocFetch{skipped: true},
		inAppFetch{},
	)
	if resp.Kpis[KpiOnTimeRate].Accuracy != AccuracyUnavailable {
		t.Fatalf("on_time_rate accuracy: %s", resp.Kpis[KpiOnTimeRate].Accuracy)
	}
	if resp.Kpis[KpiOpenOverdue].Value == nil || *resp.Kpis[KpiOpenOverdue].Value != 2 {
		t.Fatalf("open_overdue: %+v", resp.Kpis[KpiOpenOverdue])
	}
}

func TestBuildOverview_needsActionDedupe(t *testing.T) {
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	dr, _ := domain.ParseRange(domain.ParseRangeInput{Range: "30d", Now: now})
	resp := buildOverview(
		domain.CompanyBrief{ID: "c1"},
		dr,
		deadlineFetch{
			overdue: []deadlinealertsapp.DeadlineAlertDTO{{RecordID: "r1", Status: "OVERDUE", DueDate: "2026-07-01"}},
			dueSoon: []deadlinealertsapp.DeadlineAlertDTO{{RecordID: "r1", Status: "DUE_SOON", DueDate: "2026-07-10"}},
			pendingConfirm: []deadlinealertsapp.DeadlineAlertDTO{{RecordID: "r2", Status: "PENDING_CONFIRM", DueDate: "2026-07-15"}},
		},
		adHocFetch{},
		inAppFetch{},
	)
	if resp.Kpis[KpiNeedsActionNow].Value == nil || *resp.Kpis[KpiNeedsActionNow].Value != 2 {
		t.Fatalf("needs_action_now: %+v", resp.Kpis[KpiNeedsActionNow])
	}
}

func TestBuildOverview_blockedUnavailableWhenNoSources(t *testing.T) {
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	dr, _ := domain.ParseRange(domain.ParseRangeInput{Range: "30d", Now: now})
	resp := buildOverview(domain.CompanyBrief{ID: "c1"}, dr, deadlineFetch{}, adHocFetch{skipped: true}, inAppFetch{err: errTest()})
	if resp.Kpis[KpiBlockedOrException].Accuracy != AccuracyUnavailable {
		t.Fatalf("blocked: %+v", resp.Kpis[KpiBlockedOrException])
	}
}

type testErr struct{}

func (testErr) Error() string { return "test" }
func errTest() error          { return testErr{} }
