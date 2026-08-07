package app

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type fakeWorkflowRepository struct {
	mu        sync.Mutex
	instances map[string]WorkflowInstanceDTO
	tasks     map[string]TaskDTO

	createdInstance WorkflowInstanceDTO
	createdTask     TaskDTO

	failNextCreateTask error
	createTaskCalls    int
}

func (f *fakeWorkflowRepository) ensure() {
	if f.instances == nil {
		f.instances = map[string]WorkflowInstanceDTO{}
	}
	if f.tasks == nil {
		f.tasks = map[string]TaskDTO{}
	}
}

func (f *fakeWorkflowRepository) CreateInstance(_ context.Context, in WorkflowInstanceDTO) (*WorkflowInstanceDTO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensure()
	f.createdInstance = in
	f.instances[in.CompanyID+":"+in.WorkflowInstanceID] = in
	cp := in
	return &cp, nil
}

func (f *fakeWorkflowRepository) FindInstance(_ context.Context, companyID, workflowInstanceID string) (*WorkflowInstanceDTO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensure()
	in, ok := f.instances[companyID+":"+workflowInstanceID]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "workflow instance not found", nil)
	}
	cp := in
	return &cp, nil
}

func (f *fakeWorkflowRepository) UpdateInstance(_ context.Context, in WorkflowInstanceDTO) (*WorkflowInstanceDTO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensure()
	k := in.CompanyID + ":" + in.WorkflowInstanceID
	if _, ok := f.instances[k]; !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "workflow instance not found", nil)
	}
	f.instances[k] = in
	cp := in
	return &cp, nil
}

func (f *fakeWorkflowRepository) CreateTask(_ context.Context, task TaskDTO) (*TaskDTO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensure()
	f.createTaskCalls++
	if f.failNextCreateTask != nil {
		err := f.failNextCreateTask
		f.failNextCreateTask = nil
		return nil, err
	}
	f.createdTask = task
	f.tasks[task.CompanyID+":"+task.TaskID] = task
	cp := task
	return &cp, nil
}

func (f *fakeWorkflowRepository) FindTask(_ context.Context, companyID, taskID string) (*TaskDTO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensure()
	t, ok := f.tasks[companyID+":"+taskID]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "task not found", nil)
	}
	cp := t
	return &cp, nil
}

func (f *fakeWorkflowRepository) UpdateTask(_ context.Context, task TaskDTO) (*TaskDTO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensure()
	k := task.CompanyID + ":" + task.TaskID
	if _, ok := f.tasks[k]; !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "task not found", nil)
	}
	f.tasks[k] = task
	cp := task
	return &cp, nil
}

func (f *fakeWorkflowRepository) ListTasksByInstance(_ context.Context, companyID, workflowInstanceID string) ([]TaskDTO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensure()
	out := make([]TaskDTO, 0)
	for _, t := range f.tasks {
		if t.CompanyID == companyID && t.WorkflowInstanceID == workflowInstanceID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeWorkflowRepository) ApplyTaskTransition(_ context.Context, in TaskTransitionApply) (*TaskDTO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ensure()

	tk := in.CompanyID + ":" + in.TaskID
	cur, ok := f.tasks[tk]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "task not found", nil)
	}
	if cur.Status != in.FromStatus {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "task is not pending", nil)
	}
	if in.NextTask != nil && f.failNextCreateTask != nil {
		err := f.failNextCreateTask
		f.failNextCreateTask = nil
		return nil, err
	}
	cur.Status = in.ToStatus
	f.tasks[tk] = cur
	if in.NextTask != nil {
		f.createTaskCalls++
		nt := *in.NextTask
		f.createdTask = nt
		f.tasks[nt.CompanyID+":"+nt.TaskID] = nt
	}
	if in.Instance != nil {
		ik := in.Instance.CompanyID + ":" + in.Instance.WorkflowInstanceID
		if _, ok := f.instances[ik]; !ok {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "workflow instance not found", nil)
		}
		f.instances[ik] = *in.Instance
	}
	cp := cur
	return &cp, nil
}

type fakeWorkflowIDGen struct{}

func (fakeWorkflowIDGen) NewUUID() string { return "workflow-uuid" }

type seqIDGen struct{ n int }

func (g *seqIDGen) NewUUID() string {
	g.n++
	return fmt.Sprintf("id-%d", g.n)
}

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

func TestCreateWorkflowInstanceInternal_UsesExplicitFirstTaskAssignee(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	svc := NewService(repo, nil, fakeWorkflowIDGen{}, WithFlags(Flags{SnapshotEnabled: true}))

	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject: Subject{
			UserID:       "creator-user",
			MembershipID: "member-creator",
			CompanyID:    "company-001",
		},
		RecordID: "record-001",
		Snapshot: []StepSnapshot{
			{StepID: "ps-1", StepCode: "ps-1", DisplayOrder: 1, ProcessingDays: 3},
		},
		WorkflowSource:                WorkflowSourceProposalSnapshotV2,
		FirstTaskAssigneeMembershipID: "member-assignee-b",
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if repo.createdTask.AssigneeMembershipID != "member-assignee-b" {
		t.Fatalf("assignee = %q, want member-assignee-b (no creator fallback)", repo.createdTask.AssigneeMembershipID)
	}
	if repo.createdInstance.WorkflowSource != WorkflowSourceProposalSnapshotV2 {
		t.Fatalf("source = %q", repo.createdInstance.WorkflowSource)
	}
}

func TestCreateWorkflowInstanceInternal_LegacyUsesSubjectMembership(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	svc := NewService(repo, nil, fakeWorkflowIDGen{}, WithFlags(Flags{SnapshotEnabled: true}))

	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject: Subject{
			UserID:       "creator-user",
			MembershipID: "member-creator",
			CompanyID:    "company-001",
		},
		RecordID: "record-001",
		Snapshot: []StepSnapshot{
			{StepID: "step_a", StepCode: "prepare", DisplayOrder: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.createdTask.AssigneeMembershipID != "member-creator" {
		t.Fatalf("legacy assignee = %q", repo.createdTask.AssigneeMembershipID)
	}
}
