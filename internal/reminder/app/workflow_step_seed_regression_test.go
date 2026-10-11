package app

import (
	"context"
	"sort"
	"testing"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// mysqlLikeOccurrenceRepo models reminder_occurrences the way MySQL enforces it:
// two unique keys (occurrence_id, idempotency_key). SeedOccurrence reproduces the
// repository's INSERT ... ON DUPLICATE KEY UPDATE status/scheduled_at followed by a
// read-back by occurrence_id. EnsureOccurrence is insert-if-absent, read back by key.
// The previous fake errored on a duplicate key without side effects, which hid REM-01.
type mysqlLikeOccurrenceRepo struct {
	fakeOccurrenceRepo
	byID  map[string]*ReminderOccurrenceDTO
	byKey map[string]string // idempotency_key -> occurrence_id
}

func newMySQLLikeOccurrenceRepo() *mysqlLikeOccurrenceRepo {
	return &mysqlLikeOccurrenceRepo{byID: map[string]*ReminderOccurrenceDTO{}, byKey: map[string]string{}}
}

func (r *mysqlLikeOccurrenceRepo) SeedOccurrence(_ context.Context, in ReminderOccurrenceDTO) (*ReminderOccurrenceDTO, error) {
	if in.OccurrenceID == "" {
		in.OccurrenceID = in.IdempotencyKey
	}
	if id, ok := r.byKey[in.IdempotencyKey]; ok {
		row := r.byID[id]
		row.ScheduledAt = in.ScheduledAt
		row.Status = in.Status
	} else if row, ok := r.byID[in.OccurrenceID]; ok {
		row.ScheduledAt = in.ScheduledAt
		row.Status = in.Status
	} else {
		cp := in
		r.byID[in.OccurrenceID] = &cp
		r.byKey[in.IdempotencyKey] = in.OccurrenceID
	}
	row, ok := r.byID[in.OccurrenceID]
	if !ok {
		return nil, perr.NewHTTPError(404, perr.CodeInvalidRequest, "occurrence not found", nil)
	}
	out := *row
	return &out, nil
}

func (r *mysqlLikeOccurrenceRepo) EnsureOccurrence(_ context.Context, in ReminderOccurrenceDTO) (*ReminderOccurrenceDTO, error) {
	if in.OccurrenceID == "" {
		in.OccurrenceID = in.IdempotencyKey
	}
	if id, ok := r.byKey[in.IdempotencyKey]; ok {
		out := *r.byID[id]
		return &out, nil
	}
	if _, ok := r.byID[in.OccurrenceID]; !ok {
		cp := in
		r.byID[in.OccurrenceID] = &cp
		r.byKey[in.IdempotencyKey] = in.OccurrenceID
	}
	out := *r.byID[in.OccurrenceID]
	return &out, nil
}

func (r *mysqlLikeOccurrenceRepo) statusByKey(key string) ReminderStatus {
	id, ok := r.byKey[key]
	if !ok {
		return ""
	}
	return r.byID[id].Status
}

// orderedMilestoneScanner honours limit and ORDER BY scheduled_date, milestone_id like
// the MySQL scanner (the older fake ignored limit, which hid REM-06).
type orderedMilestoneScanner struct {
	due      []DueMilestone
	marked   map[string]string
	obsolete map[string]bool
}

func newOrderedMilestoneScanner(due []DueMilestone) *orderedMilestoneScanner {
	return &orderedMilestoneScanner{due: due, marked: map[string]string{}, obsolete: map[string]bool{}}
}

func (f *orderedMilestoneScanner) open() []DueMilestone {
	out := make([]DueMilestone, 0, len(f.due))
	for _, m := range f.due {
		if _, ok := f.marked[m.MilestoneID]; ok {
			continue
		}
		if f.obsolete[m.MilestoneID] {
			continue
		}
		out = append(out, m)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].ScheduledDate.Equal(out[j].ScheduledDate) {
			return out[i].ScheduledDate.Before(out[j].ScheduledDate)
		}
		return out[i].MilestoneID < out[j].MilestoneID
	})
	return out
}

func (f *orderedMilestoneScanner) ListDueMilestones(_ context.Context, asOf time.Time, limit int) ([]DueMilestone, error) {
	var out []DueMilestone
	for _, m := range f.open() {
		if m.ScheduledDate.After(asOf) {
			continue
		}
		out = append(out, m)
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out, nil
}

func (f *orderedMilestoneScanner) MarkMilestoneSent(_ context.Context, milestoneID, occurrenceID string) error {
	f.marked[milestoneID] = occurrenceID
	return nil
}

// REM-01: two milestones with the same (instance, step, due_minus_Nd) must not reset an
// already SENT occurrence back to PENDING on every tick.
func TestSeedOccurrences_DuplicateMilestonesKeepSentOccurrence(t *testing.T) {
	now := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	scanner := newOrderedMilestoneScanner([]DueMilestone{
		{MilestoneID: "m1", CompanyID: "c1", WorkflowInstanceID: "inst-1", StepID: "step-a", MilestoneType: "due_minus_3d", ScheduledDate: now},
		{MilestoneID: "m2", CompanyID: "c1", WorkflowInstanceID: "inst-1", StepID: "step-a", MilestoneType: "due_minus_3d", ScheduledDate: now},
	})
	repo := newMySQLLikeOccurrenceRepo()
	svc := NewService(fakeConfigRepo{}, repo, &fakeAttemptRepo{},
		WithMilestoneScanner(scanner),
		WithWorkflowStepTaskStateReader(fakeStepTaskStateReader{states: map[string]WorkflowStepTaskState{"step-a": WorkflowStepTaskPending}}),
	)
	key := WorkflowStepReminderIdempotencyKey("inst-1", "step-a", "due_minus_3d")

	if _, err := svc.SeedOccurrencesFromDueMilestones(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	// The dispatcher sends the reminder.
	repo.byID[repo.byKey[key]].Status = ReminderStatusSent

	for tick := 0; tick < 3; tick++ {
		if _, err := svc.SeedOccurrencesFromDueMilestones(context.Background(), now); err != nil {
			t.Fatal(err)
		}
		if got := repo.statusByKey(key); got != ReminderStatusSent {
			t.Fatalf("tick %d: occurrence status=%s, want SENT (seed must never re-arm a sent reminder)", tick, got)
		}
	}
	if len(scanner.marked) != 2 {
		t.Fatalf("both duplicate milestones must be marked handled, marked=%v", scanner.marked)
	}
	if len(repo.byID) != 1 {
		t.Fatalf("one occurrence per key, got %d", len(repo.byID))
	}
}
