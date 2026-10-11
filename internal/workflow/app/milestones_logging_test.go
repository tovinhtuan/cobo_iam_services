package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type failingMilestoneRepository struct{ fakeMilestoneRepository }

func (f *failingMilestoneRepository) InsertStepMilestones(context.Context, []StepMilestoneRow) error {
	return errors.New("insert boom")
}

// PER-05: a milestone insert failure must be logged (the request still succeeds), otherwise the
// instance silently never gets reminders.
func TestCreateWorkflowInstance_MilestoneInsertFailureIsLogged(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	repo := &fakeWorkflowRepository{}
	svc := NewService(repo, nil, fakeWorkflowIDGen{},
		WithMilestoneRepository(&failingMilestoneRepository{}),
		WithFlags(Flags{TimelineEnabled: true}),
	)
	t0 := time.Date(2026, time.June, 12, 0, 0, 0, 0, time.UTC)
	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject:  Subject{UserID: "user-001", MembershipID: "member-001", CompanyID: "company-001"},
		RecordID: "record-001",
		T0Date:   &t0,
		T0Policy: "user_defined",
		Snapshot: []StepSnapshot{{StepID: "step-review", StepCode: "step-review", DueRule: "T+1", DisplayOrder: 1}},
	})
	if err != nil {
		t.Fatalf("instance creation must not fail on milestone error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"level":"ERROR"`) || !strings.Contains(out, "workflow milestones not seeded") {
		t.Fatalf("expected an ERROR log for the milestone failure, got: %s", out)
	}
	if !strings.Contains(out, repo.createdInstance.WorkflowInstanceID) || !strings.Contains(out, `"stage":"insert"`) {
		t.Fatalf("log must carry workflow_instance_id and stage, got: %s", out)
	}
}
