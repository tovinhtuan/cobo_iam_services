package mysql

import (
	"os"
	"strings"
	"testing"
	"time"

	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
)

const threeStepSnapshot = `[
 {"step_id":"s1","step_code":"S1","stage":"Soạn","department":"legal","due_rule":"T+2","display_order":1,"processing_days":2},
 {"step_id":"s2","step_code":"S2","stage":"Duyệt","department":"legal","due_rule":"T+2","display_order":2,"processing_days":2},
 {"step_id":"s3","step_code":"","stage":"Công bố","department":"legal","due_rule":"T+1","display_order":3,"processing_days":1}
]`

func TestBuildWorkflowStepRuntime_StepTwoCurrentAfterStepOneCompleted(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	done := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	states := map[string]deadlinealertsapp.StepRuntimeState{"S1": {StepCode: "S1", CompletedAt: &done}}
	// 2026-10-02 18:30Z is already 2026-10-03 in Asia/Ho_Chi_Minh.
	rt, err := buildWorkflowStepRuntime("in_progress", "global_template", &t0, []byte(threeStepSnapshot), states, time.Date(2026, 10, 2, 18, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(rt.Steps) != 3 {
		t.Fatalf("steps=%d", len(rt.Steps))
	}
	s1, s2, s3 := rt.Steps[0], rt.Steps[1], rt.Steps[2]
	if !s1.Completed || s1.Current {
		t.Fatalf("s1=%+v", s1)
	}
	if !s2.Current || s2.StepID != "s2" || s2.StepCode != "S2" || !s2.HasEnd {
		t.Fatalf("s2=%+v", s2)
	}
	if s3.StepID != "s3" || s3.StepCode != "s3" || s3.Current {
		t.Fatalf("s3 (code falls back to step_id)=%+v", s3)
	}
	if !s1.EndDate.Before(s2.EndDate) || !s2.EndDate.Before(s3.EndDate) {
		t.Fatalf("end dates not increasing: %v %v %v", s1.EndDate, s2.EndDate, s3.EndDate)
	}
}

func TestBuildWorkflowStepRuntime_NoT0KeepsIdentityOnly(t *testing.T) {
	rt, err := buildWorkflowStepRuntime("in_progress", "global_template", nil, []byte(threeStepSnapshot), nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range rt.Steps {
		if st.Current || st.HasEnd {
			t.Fatalf("no T0 must not invent progress: %+v", st)
		}
	}
}

// REM-01 source guard: EnsureOccurrence must never overwrite state of an existing occurrence.
func TestEnsureOccurrence_DoesNotOverwriteState(t *testing.T) {
	src, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	start := strings.Index(body, "func (r *Repository) EnsureOccurrence(")
	if start < 0 {
		t.Fatal("EnsureOccurrence missing")
	}
	end := strings.Index(body[start:], "\nfunc ")
	fn := body[start : start+end]
	for _, bad := range []string{"status = VALUES(status)", "scheduled_at = VALUES(scheduled_at)"} {
		if strings.Contains(fn, bad) {
			t.Fatalf("EnsureOccurrence must not contain %q", bad)
		}
	}
	if !strings.Contains(fn, "WHERE idempotency_key = ?") {
		t.Fatal("EnsureOccurrence must read back by idempotency_key")
	}
}

// REM-06 source guard: paging keyset and obsolete marking keep reminder_sent boolean.
func TestMilestoneScanner_PagingAndObsoleteShape(t *testing.T) {
	src, err := os.ReadFile("milestone_scanner.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	if !strings.Contains(body, "ORDER BY scheduled_date ASC, milestone_id ASC") {
		t.Fatal("scanner must order by a unique keyset")
	}
	if strings.Contains(body, "reminder_sent = 2") {
		t.Fatal("reminder_sent must stay 0/1 (ListByInstance scans it as bool)")
	}
	if !strings.Contains(body, "SET reminder_sent = 1, reminder_occurrence_id = NULL") {
		t.Fatal("obsolete milestones use reminder_sent=1 with NULL occurrence")
	}
}
