package app

import (
	"context"
	"testing"
	"time"

	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
	"github.com/cobo/cobo_iam_services/internal/portaldashboard/domain"
)

// snapshotDeadlineService deliberately counts both APIs. The overview must use
// one internal snapshot, rather than rebuilding the list for each status/window.
type snapshotDeadlineService struct {
	deadlinealertsapp.Service
	items         []deadlinealertsapp.DeadlineAlertDTO
	snapshotCalls int
	listCalls     int
}

func (s *snapshotDeadlineService) ListDeadlineAlertSnapshot(_ context.Context, _ deadlinealertsapp.Subject) ([]deadlinealertsapp.DeadlineAlertDTO, error) {
	s.snapshotCalls++
	return s.items, nil
}

func (s *snapshotDeadlineService) ListDeadlineAlerts(_ context.Context, _ deadlinealertsapp.ListDeadlineAlertsRequest) (*deadlinealertsapp.ListDeadlineAlertsResponse, error) {
	s.listCalls++
	return &deadlinealertsapp.ListDeadlineAlertsResponse{}, nil
}

func TestFetchDeadlines_usesOneSnapshotInsteadOfRepeatedListPipeline(t *testing.T) {
	deadlines := &snapshotDeadlineService{items: []deadlinealertsapp.DeadlineAlertDTO{
		{RecordID: "overdue", DueDate: "2026-09-03", Status: "OVERDUE"},
		{RecordID: "due-soon", DueDate: "2026-09-03", Status: "DUE_SOON"},
		{RecordID: "upcoming", DueDate: "2026-09-08", Status: "UPCOMING"},
		{RecordID: "done", DueDate: "2026-09-04", Status: "DONE"},
	}}
	svc := &service{deadlines: deadlines}
	dr, err := domain.ParseRange(domain.ParseRangeInput{
		Range: "30d", From: "2026-09-03", To: "2026-10-02", Timezone: "Asia/Ho_Chi_Minh",
		Now: time.Date(2026, 9, 8, 8, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := svc.fetchDeadlines(context.Background(), deadlinealertsapp.Subject{CompanyID: "c_001"}, dr)
	if err != nil {
		t.Fatal(err)
	}
	if deadlines.snapshotCalls != 1 {
		t.Fatalf("snapshot calls=%d, want 1", deadlines.snapshotCalls)
	}
	if deadlines.listCalls != 0 {
		t.Fatalf("legacy list calls=%d, want 0", deadlines.listCalls)
	}
	if got.overdueTotal != 1 || got.dueSoonTotal != 1 || got.upcomingTotal != 1 || len(got.done) != 1 {
		t.Fatalf("unexpected snapshot aggregation: %+v", got)
	}
}

// MR-3b (contract v1.2 §2.2, PO Q-18): the open-state KPIs #1 needs_action_now, #3
// pending_approval and #5 blocked_or_exception are a current snapshot, the same for every range:
// an obligation overdue since before the range start or due after its end still counts. The range
// only selects the completion candidates (#4, #6).
func TestOverview_openKPIsAreACurrentSnapshotWhateverTheRange(t *testing.T) {
	now := time.Date(2026, 9, 8, 8, 0, 0, 0, time.UTC)
	deadlines := &snapshotDeadlineService{items: []deadlinealertsapp.DeadlineAlertDTO{
		{RecordID: "overdue-old", DueDate: "2026-06-30", Status: "OVERDUE"},
		{RecordID: "overdue", DueDate: "2026-09-07", Status: "OVERDUE"},
		{RecordID: "due-soon", DueDate: "2026-09-08", Status: "DUE_SOON"},
		{RecordID: "upcoming-late", DueDate: "2026-12-01", Status: "UPCOMING"},
		{RecordID: "done-in-7d", DueDate: "2026-09-05", Status: "DONE"},
		{RecordID: "done-old", DueDate: "2026-07-01", Status: "DONE"},
	}}
	svc := &service{deadlines: deadlines}
	kpi := func(resp domain.OverviewResponse, key string) float64 {
		m, ok := resp.Kpis[key]
		if !ok || m.Value == nil {
			t.Fatalf("kpi %s missing: %+v", key, m)
		}
		return *m.Value
	}
	for _, rng := range []string{"7d", "30d"} {
		dr, err := domain.ParseRange(domain.ParseRangeInput{Range: rng, Timezone: "Asia/Ho_Chi_Minh", Now: now})
		if err != nil {
			t.Fatal(err)
		}
		got, err := svc.fetchDeadlines(context.Background(), deadlinealertsapp.Subject{CompanyID: "c_001"}, dr)
		if err != nil {
			t.Fatal(err)
		}
		if got.overdueTotal != 2 || got.dueSoonTotal != 1 || got.upcomingTotal != 1 {
			t.Errorf("range %s: open buckets overdue=%d due_soon=%d upcoming=%d, want 2/1/1 whatever the range",
				rng, got.overdueTotal, got.dueSoonTotal, got.upcomingTotal)
		}
		if len(got.done) != 1 || got.done[0].RecordID != "done-in-7d" {
			t.Errorf("range %s: completion candidates=%v, want only the one due in the range", rng, got.done)
		}
		resp := buildOverview(domain.CompanyBrief{ID: "c_001"}, dr, got, adHocFetch{}, inAppFetch{}, completionFetch{ok: true})
		if v := kpi(resp, KpiNeedsActionNow); v != 3 {
			t.Errorf("range %s: needs_action_now=%v, want 3", rng, v)
		}
		if v := kpi(resp, KpiPendingApproval); v != 4 {
			t.Errorf("range %s: pending_approval=%v, want 4", rng, v)
		}
		if v := kpi(resp, KpiBlockedOrException); v != 2 {
			t.Errorf("range %s: blocked_or_exception=%v, want 2", rng, v)
		}
	}
}
