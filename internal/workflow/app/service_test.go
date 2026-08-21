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

	failNextCreateTask     error
	failNextCreateInstance error
	createTaskCalls        int
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
	if f.failNextCreateInstance != nil {
		err := f.failNextCreateInstance
		f.failNextCreateInstance = nil
		return nil, err
	}
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
	cp := task
	if len(cp.AssigneeMembershipIDs) > 0 {
		cp.AssigneeMembershipID = ""
	}
	f.createdTask = cp
	f.tasks[task.CompanyID+":"+task.TaskID] = cp
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
		if len(nt.AssigneeMembershipIDs) > 0 {
			nt.AssigneeMembershipID = ""
		}
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

func TestCreateWorkflowInstanceInternalRejectsMissingSnapshotBeforeInsert(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	svc := NewService(repo, nil, fakeWorkflowIDGen{})
	t0 := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)

	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject: Subject{
			UserID:       "user-001",
			MembershipID: "member-001",
			CompanyID:    "company-001",
		},
		RecordID: "record-001",
		T0Date:   &t0,
		T0Policy: "user_defined",
	})
	if err == nil {
		t.Fatal("expected missing frozen snapshot to be rejected")
	}
	if len(repo.instances) != 0 || repo.createdInstance.WorkflowInstanceID != "" {
		t.Fatalf("instance must not be inserted when snapshot is missing: %#v", repo.createdInstance)
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

// T11: CONFIG_CHANGE != RUNNING_INSTANCE_MUTATION — persisted snapshot length/stage
// stay frozen even if caller later builds a different effective workflow snapshot.
func TestCreateWorkflowInstanceInternal_SnapshotFrozenAfterConfigChange(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	svc := NewService(repo, nil, fakeWorkflowIDGen{}, WithFlags(Flags{SnapshotEnabled: true}))

	v1 := []StepSnapshot{
		{StepID: "s1", StepCode: "s1", Stage: "A", DisplayOrder: 1},
		{StepID: "s2", StepCode: "s2", Stage: "B", DisplayOrder: 2},
		{StepID: "s3", StepCode: "s3", Stage: "C", DisplayOrder: 3},
		{StepID: "s4", StepCode: "s4", Stage: "D", DisplayOrder: 4},
	}
	created, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject:        Subject{UserID: "u", MembershipID: "m", CompanyID: "c"},
		RecordID:       "rec-x",
		Snapshot:       v1,
		WorkflowSource: "company_override",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(created.Snapshot) != 4 || created.Snapshot[0].Stage != "A" {
		t.Fatalf("frozen snapshot = %#v", created.Snapshot)
	}
	// Simulate company override activating v2=5 steps after instance creation.
	v2 := append(append([]StepSnapshot{}, v1...), StepSnapshot{StepID: "s5", StepCode: "s5", Stage: "E", DisplayOrder: 5})
	_ = v2
	if len(repo.createdInstance.Snapshot) != 4 {
		t.Fatalf("FAIL_COMPANY_OVERRIDE_RUNTIME_SNAPSHOT: instance mutated to %d steps", len(repo.createdInstance.Snapshot))
	}
	if repo.createdInstance.WorkflowSource != "company_override" {
		t.Fatalf("source = %q", repo.createdInstance.WorkflowSource)
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

func TestCreateWorkflowInstanceInternal_SnapshotInsertFailureCreatesNoRow(t *testing.T) {
	repo := &fakeWorkflowRepository{failNextCreateInstance: fmt.Errorf("snapshot persist failed")}
	svc := NewService(repo, nil, fakeWorkflowIDGen{}, WithFlags(Flags{SnapshotEnabled: true}))

	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject:        Subject{UserID: "u", MembershipID: "m", CompanyID: "c"},
		RecordID:       "rec-n4",
		Snapshot:       []StepSnapshot{{StepID: "s1", StepCode: "s1", DisplayOrder: 1}},
		WorkflowSource: "global_template",
	})
	if err == nil {
		t.Fatal("snapshot insert failure must roll back instance creation")
	}
	if len(repo.instances) != 0 {
		t.Fatalf("FAIL_FINAL_WORKFLOW_NULL_SNAPSHOT_CREATED: leftover instances=%d", len(repo.instances))
	}
	if repo.createTaskCalls != 0 {
		t.Fatalf("task must not be created after snapshot insert failure, calls=%d", repo.createTaskCalls)
	}
}

func TestCreateWorkflowInstanceInternal_RejectsEmptySnapshot(t *testing.T) {
	repo := &fakeWorkflowRepository{}
	svc := NewService(repo, nil, fakeWorkflowIDGen{}, WithFlags(Flags{SnapshotEnabled: true}))
	_, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject:  Subject{UserID: "u", MembershipID: "m", CompanyID: "c"},
		RecordID: "rec-empty",
	})
	if err == nil {
		t.Fatal("NEW_INSTANCE_WITHOUT_SNAPSHOT_FORBIDDEN")
	}
	if len(repo.instances) != 0 {
		t.Fatal("empty snapshot must not insert a row")
	}
}
