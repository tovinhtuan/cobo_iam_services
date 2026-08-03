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

func TestBuildOverview_completionMissingKeepsOnTimeUnavailable(t *testing.T) {
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	dr, _ := domain.ParseRange(domain.ParseRangeInput{Range: "30d", Now: now})
	resp := buildOverview(
		domain.CompanyBrief{ID: "c1"},
		dr,
		deadlineFetch{overdueTotal: 2, overdue: []deadlinealertsapp.DeadlineAlertDTO{{RecordID: "r1", DueDate: "2026-07-01", Status: "OVERDUE"}}},
		adHocFetch{skipped: true},
		inAppFetch{},
		completionFetch{ok: false},
	)
	if resp.Kpis[KpiOnTimeRate].Accuracy != AccuracyUnavailable {
		t.Fatalf("on_time_rate accuracy: %s", resp.Kpis[KpiOnTimeRate].Accuracy)
	}
	// incomplete overdue remapped onto blocked_or_exception
	if resp.Kpis[KpiBlockedOrException].Value == nil || *resp.Kpis[KpiBlockedOrException].Value != 2 {
		t.Fatalf("incomplete overdue: %+v", resp.Kpis[KpiBlockedOrException])
	}
	// completed overdue unavailable without completion source
	if resp.Kpis[KpiOpenOverdue].Accuracy != AccuracyUnavailable {
		t.Fatalf("completed overdue: %+v", resp.Kpis[KpiOpenOverdue])
	}
	if resp.DeadlineHealth.OnTimeCount != 0 {
		t.Fatalf("on_time_count want 0 got %d", resp.DeadlineHealth.OnTimeCount)
	}
}

func TestBuildOverview_onTimeCountDedupesOpenNonOverdue(t *testing.T) {
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	dr, _ := domain.ParseRange(domain.ParseRangeInput{Range: "30d", Now: now})
	resp := buildOverview(
		domain.CompanyBrief{ID: "c1"},
		dr,
		deadlineFetch{
			overdue: []deadlinealertsapp.DeadlineAlertDTO{{RecordID: "late1", Status: "OVERDUE", DueDate: "2026-07-01"}},
			dueSoon: []deadlinealertsapp.DeadlineAlertDTO{
				{RecordID: "a", Status: "DUE_SOON", DueDate: "2026-07-11"},
				{RecordID: "b", Status: "DUE_SOON", DueDate: "2026-07-12"},
			},
			pendingConfirm: []deadlinealertsapp.DeadlineAlertDTO{{RecordID: "a", Status: "PENDING_CONFIRM", DueDate: "2026-07-11"}},
			upcoming:       []deadlinealertsapp.DeadlineAlertDTO{{RecordID: "c", Status: "UPCOMING", DueDate: "2026-07-20"}},
		},
		adHocFetch{skipped: true},
		inAppFetch{},
		completionFetch{ok: true},
	)
	if resp.DeadlineHealth.OnTimeCount != 3 {
		t.Fatalf("on_time_count: %d", resp.DeadlineHealth.OnTimeCount)
	}
	if resp.DeadlineHealth.OnTimeRate.Accuracy != AccuracyUnavailable {
		t.Fatalf("percent on_time_rate still unavailable on deadline_health: %+v", resp.DeadlineHealth.OnTimeRate)
	}
}

func TestCountOnTimeOpenAlerts_empty(t *testing.T) {
	if n := countOnTimeOpenAlerts(nil, nil); n != 0 {
		t.Fatalf("empty want 0 got %d", n)
	}
}

func TestBuildOverview_needsActionUsesPanelMembership(t *testing.T) {
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	dr, _ := domain.ParseRange(domain.ParseRangeInput{Range: "30d", Now: now})
	resp := buildOverview(
		domain.CompanyBrief{ID: "c1"},
		dr,
		deadlineFetch{
			overdueTotal:        1,
			dueSoonTotal:        1,
			pendingConfirmTotal: 1,
			overdue:             []deadlinealertsapp.DeadlineAlertDTO{{RecordID: "r1", Status: "OVERDUE", DueDate: "2026-07-01"}},
			dueSoon:             []deadlinealertsapp.DeadlineAlertDTO{{RecordID: "r2", Status: "DUE_SOON", DueDate: "2026-07-10"}},
			pendingConfirm:      []deadlinealertsapp.DeadlineAlertDTO{{RecordID: "r3", Status: "PENDING_CONFIRM", DueDate: "2026-07-15"}},
		},
		adHocFetch{},
		inAppFetch{},
		completionFetch{ok: true},
	)
	if resp.Kpis[KpiNeedsActionNow].Value == nil || *resp.Kpis[KpiNeedsActionNow].Value != 3 {
		t.Fatalf("needs_action_now: %+v", resp.Kpis[KpiNeedsActionNow])
	}
}

func TestBuildOverview_processingAndIncompleteOverdue(t *testing.T) {
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	dr, _ := domain.ParseRange(domain.ParseRangeInput{Range: "30d", Now: now})
	resp := buildOverview(
		domain.CompanyBrief{ID: "c1"},
		dr,
		deadlineFetch{
			overdueTotal:        2,
			dueSoonTotal:        1,
			upcomingTotal:       3,
			pendingConfirmTotal: 1,
			dueIn7Days:          4,
		},
		adHocFetch{skipped: true},
		inAppFetch{},
		completionFetch{ok: true, completedOnTime: 8, completedTotal: 10, completedOverdue: 2},
	)
	if resp.Kpis[KpiPendingApproval].Value == nil || *resp.Kpis[KpiPendingApproval].Value != 7 {
		t.Fatalf("processing: %+v", resp.Kpis[KpiPendingApproval])
	}
	if resp.Kpis[KpiBlockedOrException].Value == nil || *resp.Kpis[KpiBlockedOrException].Value != 2 {
		t.Fatalf("incomplete overdue: %+v", resp.Kpis[KpiBlockedOrException])
	}
	if resp.Kpis[KpiOpenOverdue].Value == nil || *resp.Kpis[KpiOpenOverdue].Value != 2 {
		t.Fatalf("completed overdue: %+v", resp.Kpis[KpiOpenOverdue])
	}
	if resp.Kpis[KpiDueNext7Days].Value == nil || *resp.Kpis[KpiDueNext7Days].Value != 4 {
		t.Fatalf("due 7d: %+v", resp.Kpis[KpiDueNext7Days])
	}
	if resp.Kpis[KpiOnTimeRate].Value == nil || *resp.Kpis[KpiOnTimeRate].Value != 80 {
		t.Fatalf("on_time_rate: %+v", resp.Kpis[KpiOnTimeRate])
	}
	if resp.Kpis[KpiOnTimeRate].CompletedTotal == nil || *resp.Kpis[KpiOnTimeRate].CompletedTotal != 10 {
		t.Fatalf("completed_total: %+v", resp.Kpis[KpiOnTimeRate])
	}
}

func TestBuildOverview_zeroDenominatorOnTimeRate(t *testing.T) {
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	dr, _ := domain.ParseRange(domain.ParseRangeInput{Range: "30d", Now: now})
	resp := buildOverview(
		domain.CompanyBrief{ID: "c1"},
		dr,
		deadlineFetch{},
		adHocFetch{skipped: true},
		inAppFetch{},
		completionFetch{ok: true, completedOnTime: 0, completedTotal: 0, completedOverdue: 0},
	)
	if resp.Kpis[KpiOnTimeRate].Accuracy != AccuracyExact {
		t.Fatalf("accuracy: %s", resp.Kpis[KpiOnTimeRate].Accuracy)
	}
	if resp.Kpis[KpiOnTimeRate].Value == nil || *resp.Kpis[KpiOnTimeRate].Value != 0 {
		t.Fatalf("want 0%% got %+v", resp.Kpis[KpiOnTimeRate])
	}
}

func TestComputeCompletion_boundariesAndExclusions(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	due := "2026-07-10"
	onTimeEarly := time.Date(2026, 7, 9, 15, 0, 0, 0, loc)
	onTimeExact := time.Date(2026, 7, 10, 23, 0, 0, 0, loc)
	late := time.Date(2026, 7, 11, 1, 0, 0, 0, loc)

	alerts := []deadlinealertsapp.DeadlineAlertDTO{
		{RecordID: "a", DueDate: due},
		{RecordID: "b", DueDate: due},
		{RecordID: "c", DueDate: due},
		{RecordID: "d", DueDate: ""},          // missing due
		{RecordID: "e", DueDate: due},         // missing completed_at
		{RecordID: "a", DueDate: due},         // dup
	}
	completed := map[string]time.Time{
		"a": onTimeEarly,
		"b": onTimeExact,
		"c": late,
	}
	stats := computeCompletionFromAlerts(alerts, completed, loc)
	if !stats.ok || stats.completedTotal != 3 || stats.completedOnTime != 2 || stats.completedOverdue != 1 {
		t.Fatalf("stats: %+v", stats)
	}
	if onTimeRatePercent(2, 3) != 67 {
		t.Fatalf("round percent")
	}
	if onTimeRatePercent(0, 0) != 0 {
		t.Fatalf("zero denom")
	}
}

func TestBuildOverview_noDoubleCountCompletedVsIncomplete(t *testing.T) {
	now := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	dr, _ := domain.ParseRange(domain.ParseRangeInput{Range: "30d", Now: now})
	resp := buildOverview(
		domain.CompanyBrief{ID: "c1"},
		dr,
		deadlineFetch{overdueTotal: 5},
		adHocFetch{skipped: true},
		inAppFetch{},
		completionFetch{ok: true, completedOverdue: 3, completedTotal: 10, completedOnTime: 7},
	)
	incomplete := *resp.Kpis[KpiBlockedOrException].Value
	completedLate := *resp.Kpis[KpiOpenOverdue].Value
	if incomplete == completedLate && incomplete == 5 {
		t.Fatal("should not use same source for both exclusive KPIs when completion present")
	}
	if incomplete != 5 || completedLate != 3 {
		t.Fatalf("incomplete=%v completedLate=%v", incomplete, completedLate)
	}
}

type testErr struct{}

func (testErr) Error() string { return "test" }
func errTest() error          { return testErr{} }
