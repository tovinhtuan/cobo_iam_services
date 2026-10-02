package app

import (
	"context"
	"testing"

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
