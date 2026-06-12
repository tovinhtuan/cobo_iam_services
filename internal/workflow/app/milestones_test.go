package app

import (
	"context"
	"testing"
	"time"
)

type fakeMilestoneRepository struct {
	inserted []StepMilestoneRow
}

func (f *fakeMilestoneRepository) InsertStepMilestones(_ context.Context, rows []StepMilestoneRow) error {
	f.inserted = append(f.inserted, rows...)
	return nil
}

func (f *fakeMilestoneRepository) ListByInstance(context.Context, string, string) ([]InstanceReminderDTO, error) {
	return nil, nil
}

func TestCreateWorkflowInstanceInternalSeedsMilestonesWhenTimelineEnabled(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	milestones := &fakeMilestoneRepository{}
	svc := NewService(repo, nil, fakeWorkflowIDGen{},
		WithMilestoneRepository(milestones),
		WithFlags(Flags{TimelineEnabled: true}),
	)
	t0 := time.Date(2026, time.June, 12, 0, 0, 0, 0, time.UTC)

	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject: Subject{
			UserID:       "user-001",
			MembershipID: "member-001",
			CompanyID:    "company-001",
		},
		RecordID: "record-001",
		T0Date:   &t0,
		T0Policy: "user_defined",
		Snapshot: []StepSnapshot{{
			StepID:       "step-review",
			StepCode:     "step-review",
			DueRule:      "T+1",
			DisplayOrder: 1,
		}},
	})
	if err != nil {
		t.Fatalf("CreateWorkflowInstanceInternal() error = %v", err)
	}
	if len(milestones.inserted) == 0 {
		t.Fatal("expected milestone rows to be seeded when timeline is enabled")
	}
	if milestones.inserted[0].InstanceID != repo.createdInstance.WorkflowInstanceID {
		t.Fatalf("expected milestone instance_id %q, got %q", repo.createdInstance.WorkflowInstanceID, milestones.inserted[0].InstanceID)
	}
}
