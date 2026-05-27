package app

import (
	"context"
	"testing"
	"time"
)

type fakeWorkflowRepository struct {
	createdInstance WorkflowInstanceDTO
	createdTask     TaskDTO
}

func (f *fakeWorkflowRepository) CreateInstance(ctx context.Context, in WorkflowInstanceDTO) (*WorkflowInstanceDTO, error) {
	f.createdInstance = in
	cp := in
	return &cp, nil
}

func (f *fakeWorkflowRepository) FindInstance(ctx context.Context, companyID, workflowInstanceID string) (*WorkflowInstanceDTO, error) {
	return nil, nil
}

func (f *fakeWorkflowRepository) UpdateInstance(ctx context.Context, in WorkflowInstanceDTO) (*WorkflowInstanceDTO, error) {
	cp := in
	return &cp, nil
}

func (f *fakeWorkflowRepository) CreateTask(ctx context.Context, task TaskDTO) (*TaskDTO, error) {
	f.createdTask = task
	cp := task
	return &cp, nil
}

func (f *fakeWorkflowRepository) FindTask(ctx context.Context, companyID, taskID string) (*TaskDTO, error) {
	return nil, nil
}

func (f *fakeWorkflowRepository) UpdateTask(ctx context.Context, task TaskDTO) (*TaskDTO, error) {
	cp := task
	return &cp, nil
}

func (f *fakeWorkflowRepository) ListTasksByInstance(ctx context.Context, companyID, workflowInstanceID string) ([]TaskDTO, error) {
	return nil, nil
}

type fakeWorkflowIDGen struct{}

func (fakeWorkflowIDGen) NewUUID() string { return "workflow-uuid" }

func TestCreateWorkflowInstanceInternalPersistsT0WithoutSnapshotFlag(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	svc := NewService(repo, nil, fakeWorkflowIDGen{})
	t0 := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)

	created, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject: Subject{
			UserID:       "user-001",
			MembershipID: "member-001",
			CompanyID:    "company-001",
		},
		RecordID: "record-001",
		T0Date:   &t0,
		T0Policy: "user_defined",
	})
	if err != nil {
		t.Fatalf("CreateWorkflowInstanceInternal() error = %v", err)
	}
	if created.T0Date == nil || !created.T0Date.Equal(t0) {
		t.Fatalf("expected returned workflow instance to keep T0Date, got %#v", created.T0Date)
	}
	if created.T0Policy != "user_defined" {
		t.Fatalf("expected returned workflow instance to keep T0Policy, got %q", created.T0Policy)
	}
	if repo.createdInstance.T0Date == nil || !repo.createdInstance.T0Date.Equal(t0) {
		t.Fatalf("expected repository insert to receive T0Date, got %#v", repo.createdInstance.T0Date)
	}
	if repo.createdInstance.T0Policy != "user_defined" {
		t.Fatalf("expected repository insert to receive T0Policy, got %q", repo.createdInstance.T0Policy)
	}
}

func TestCreateWorkflowInstanceInternalUsesFirstSnapshotStep(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	svc := NewService(repo, nil, fakeWorkflowIDGen{}, WithFlags(Flags{SnapshotEnabled: true}))

	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject: Subject{
			UserID:       "user-001",
			MembershipID: "member-001",
			CompanyID:    "company-001",
		},
		RecordID: "record-001",
		Snapshot: []StepSnapshot{
			{StepID: "step_b", StepCode: "review", DisplayOrder: 2},
			{StepID: "step_a", StepCode: "prepare", DisplayOrder: 1},
		},
		WorkflowSource: "global_template",
	})
	if err != nil {
		t.Fatalf("CreateWorkflowInstanceInternal() error = %v", err)
	}
	if repo.createdInstance.CurrentStepCode != "prepare" {
		t.Fatalf("CurrentStepCode = %q, want prepare", repo.createdInstance.CurrentStepCode)
	}
	if repo.createdTask.StepCode != "prepare" {
		t.Fatalf("task StepCode = %q, want prepare", repo.createdTask.StepCode)
	}
	if len(repo.createdInstance.Snapshot) != 2 {
		t.Fatalf("snapshot len = %d, want 2", len(repo.createdInstance.Snapshot))
	}
}
