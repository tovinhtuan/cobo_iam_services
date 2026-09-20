package app

import (
	"strings"
	"testing"
	"time"

	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
	inappapp "github.com/cobo/cobo_iam_services/internal/inappnotification/app"
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

func TestBuildWorkflowRiskRows_prefersTypeTitleOverPeriodicCategory(t *testing.T) {
	rows := buildWorkflowRiskRows(
		[]deadlinealertsapp.DeadlineAlertDTO{
			{
				RecordID:         "r1",
				TypeID:           "type-bctc",
				Title:            "Báo cáo tài chính quý",
				TemplateCategory: "periodic",
				Status:           "OVERDUE",
				CurrentStepName:  "Soát xét",
				ActiveDepartments: []string{"TC"},
			},
			{
				RecordID:         "r2",
				TypeID:           "type-bctc",
				Title:            "Báo cáo tài chính quý",
				TemplateCategory: "periodic",
				Status:           "OVERDUE",
				CurrentStepName:  "Phê duyệt",
			},
		},
		nil,
		10,
	)
	if len(rows) != 1 {
		t.Fatalf("want 1 aggregate row, got %d", len(rows))
	}
	if rows[0].Key != "type-bctc" {
		t.Fatalf("key=%q", rows[0].Key)
	}
	if rows[0].WorkflowName != "Báo cáo tài chính quý" {
		t.Fatalf("workflow_name=%q (must not be periodic enum)", rows[0].WorkflowName)
	}
	if rows[0].WorkflowName == "periodic" {
		t.Fatal("aggregate must not display TemplateCategory enum as name")
	}
	if rows[0].OverdueCount != 2 {
		t.Fatalf("overdue_count=%d", rows[0].OverdueCount)
	}
}

func TestBuildRecentActivities_preservesTitleBodyAndDeadlineURL(t *testing.T) {
	rid := "52698f3f-53c0-53e7-9e40-d99f90774f41"
	rt := "disclosure"
	title := "QA DEF006 Milestones 20260904213636"
	body := "Bước: Xác định nghĩa vụ · Hạn: 17/09/2026"
	items := buildRecentActivities([]inappapp.InAppNotification{
		{
			ID:           "n1",
			Kind:         "reminder.workflow_step_due",
			Title:        title,
			Body:         body,
			ResourceType: &rt,
			ResourceID:   &rid,
			CreatedAt:    time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC),
		},
	}, nil, 8)
	if len(items) != 1 {
		t.Fatalf("len=%d", len(items))
	}
	if items[0].Title != title {
		t.Fatalf("title=%q", items[0].Title)
	}
	if items[0].Summary != body {
		t.Fatalf("summary=%q", items[0].Summary)
	}
	if items[0].TargetURL != "/app/deadlines/"+rid {
		t.Fatalf("target_url=%q", items[0].TargetURL)
	}
	if !items[0].DetailAvailable {
		t.Fatal("detail_available want true")
	}
	if items[0].IsLegacy {
		t.Fatal("is_legacy want false for specific title")
	}
	if items[0].ResourceID != rid {
		t.Fatalf("resource_id=%q", items[0].ResourceID)
	}
}

func TestBuildRecentActivities_legacyWithoutResourceIDUnavailable(t *testing.T) {
	rt := "disclosure"
	empty := ""
	items := buildRecentActivities([]inappapp.InAppNotification{
		{
			ID:           "legacy-1",
			Kind:         "reminder.workflow_step_due",
			Title:        "Bước phê duyệt đến hạn: Xác định nghĩa vụ",
			Body:         "Deadline: 17/09/2026",
			ResourceType: &rt,
			ResourceID:   &empty,
			CreatedAt:    time.Date(2026, 9, 16, 0, 1, 39, 0, time.UTC),
		},
	}, nil, 8)
	if len(items) != 1 {
		t.Fatalf("len=%d", len(items))
	}
	got := items[0]
	if got.DetailAvailable {
		t.Fatal("detail_available must be false")
	}
	if !got.IsLegacy {
		t.Fatal("is_legacy want true")
	}
	if got.TargetURL != "" {
		t.Fatalf("target_url must be empty, got %q", got.TargetURL)
	}
	if got.Title != "Thông báo lịch sử — không còn đủ dữ liệu để mở chi tiết" {
		t.Fatalf("title=%q", got.Title)
	}
	if !strings.Contains(got.Summary, "Hoạt động cũ chưa có liên kết") {
		t.Fatalf("summary=%q", got.Summary)
	}
	if strings.Contains(got.Title, "Bước phê duyệt đến hạn") {
		t.Fatal("must not keep generic title as primary")
	}
}

func TestBuildRecentActivities_legacyWithResourceIDEnrichedFromIndex(t *testing.T) {
	rt := "disclosure"
	rid := "rec-enrich-1"
	items := buildRecentActivities([]inappapp.InAppNotification{
		{
			ID:           "legacy-res",
			Kind:         "reminder.workflow_step_due",
			Title:        "Bước phê duyệt đến hạn: Xác định nghĩa vụ",
			Body:         "Deadline: 17/09/2026",
			ResourceType: &rt,
			ResourceID:   &rid,
			CreatedAt:    time.Date(2026, 9, 16, 0, 1, 39, 0, time.UTC),
		},
	}, map[string]string{rid: "QA DEF006 Milestones 20260904213636"}, 8)
	got := items[0]
	if !got.DetailAvailable {
		t.Fatal("detail_available want true")
	}
	if got.TargetURL != "/app/deadlines/"+rid {
		t.Fatalf("target_url=%q", got.TargetURL)
	}
	if got.Title != "QA DEF006 Milestones 20260904213636" {
		t.Fatalf("title=%q", got.Title)
	}
	if !strings.Contains(got.Summary, "Bước:") || !strings.Contains(got.Summary, "Hạn:") {
		t.Fatalf("summary=%q", got.Summary)
	}
}

func TestBuildRecentActivities_doesNotEnrichUnknownRecordID(t *testing.T) {
	rt := "disclosure"
	rid := "missing-rec"
	items := buildRecentActivities([]inappapp.InAppNotification{
		{
			ID:           "legacy-miss",
			Kind:         "reminder.workflow_step_due",
			Title:        "Bước phê duyệt đến hạn: Soát xét",
			Body:         "Deadline: 01/01/2026",
			ResourceType: &rt,
			ResourceID:   &rid,
			CreatedAt:    time.Now().UTC(),
		},
	}, map[string]string{"other": "Other Title"}, 8)
	got := items[0]
	if !got.DetailAvailable {
		t.Fatal("still detail available via resource_id")
	}
	if got.Title != "Bước phê duyệt đến hạn: Soát xét" {
		t.Fatalf("title unchanged when index miss: %q", got.Title)
	}
}

type testErr struct{}

func (testErr) Error() string { return "test" }
func errTest() error          { return testErr{} }
