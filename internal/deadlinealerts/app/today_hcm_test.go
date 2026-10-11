package app

import (
	"testing"
	"time"

	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

// DLA-01/DUE-04: "today" is the Asia/Ho_Chi_Minh date of the instant. A UTC instant between
// 17:00 and 23:59 is already the next day in Vietnam.
func TestRemainingDaysFromDue_UTCInstantEarlyHCM(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	cases := []struct {
		now  time.Time
		due  string
		want int
	}{
		{time.Date(2026, 10, 19, 18, 30, 0, 0, time.UTC), "2026-10-20", 0},  // 20/10 01:30 HCM
		{time.Date(2026, 10, 19, 18, 30, 0, 0, time.UTC), "2026-10-19", -1}, // overdue since midnight HCM
		{time.Date(2026, 10, 19, 16, 59, 59, 0, time.UTC), "2026-10-20", 1}, // 19/10 23:59:59 HCM
		{time.Date(2026, 10, 19, 17, 0, 0, 0, time.UTC), "2026-10-20", 0},   // 20/10 00:00 HCM
	}
	for _, tc := range cases {
		if got := remainingDaysFromDue(tc.due, tc.now, loc); got != tc.want {
			t.Fatalf("now=%s due=%s: got %d want %d", tc.now.Format(time.RFC3339), tc.due, got, tc.want)
		}
	}
}

func TestComputeDeadlineSteps_TodayFromUTCInstant(t *testing.T) {
	ctx := WorkflowInstanceContext{
		RecordID: "rec-1",
		T0Date:   time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC),
		Snapshot: []workflowapp.StepSnapshot{
			{StepID: "s1", Stage: "Bước một", DisplayOrder: 1, ProcessingDays: 3},
			{StepID: "s2", Stage: "Bước hai", DisplayOrder: 2, ProcessingDays: 5},
		},
		Timezone: "Asia/Ho_Chi_Minh",
	}
	// 2026-10-19T18:00Z is 2026-10-20 01:00 in Vietnam: step 1 starts today.
	resp, err := ComputeDeadlineSteps(ctx, nil, time.Date(2026, 10, 19, 18, 0, 0, 0, time.UTC), ctx.Timezone, true)
	if err != nil {
		t.Fatal(err)
	}
	if resp.CurrentStepCode != "s1" || resp.Steps[0].Status != "current" {
		t.Fatalf("step 1 must be current on its HCM start date, got current=%q status=%s", resp.CurrentStepCode, resp.Steps[0].Status)
	}
}
