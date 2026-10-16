package app

import (
	"context"
	"net/http"
	"slices"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// bug-workflow-step-skip-2026-10-16 (WF-01): a workflow materialized from a template only gets a
// task for step 1; step progress lives in workflow_instance_step_states. Approving that task used to
// complete the whole instance and approve the record, skipping steps 2..N. On manual submit the task
// is assigned to the submitter, so they could approve their own record.

var templateSubmitter = Subject{UserID: "u-submitter", MembershipID: "m-submitter", CompanyID: "c1"}

func newTemplateWorkflow(t *testing.T, source string, stepCodes ...string) (*fakeWorkflowRepository, *countingRecordUpdater, Service, string) {
	t.Helper()
	repo := &fakeWorkflowRepository{}
	updater := &countingRecordUpdater{}
	svc := NewService(repo, allowAuth{}, &seqIDGen{},
		WithFlags(Flags{SnapshotEnabled: true}),
		WithRecordStatusUpdater(updater),
	)
	snapshot := make([]StepSnapshot, 0, len(stepCodes))
	for i, code := range stepCodes {
		snapshot = append(snapshot, StepSnapshot{StepID: code, StepCode: code, DisplayOrder: i + 1, ProcessingDays: 1})
	}
	if _, err := svc.CreateWorkflowInstanceInternal(context.Background(), CreateWorkflowInstanceRequest{
		Subject: templateSubmitter, RecordID: "rec-" + source, Snapshot: snapshot, WorkflowSource: source,
	}); err != nil {
		t.Fatal(err)
	}
	return repo, updater, svc, repo.createdInstance.WorkflowInstanceID
}

func TestTemplateWorkflow_CompletingStepOneTaskDoesNotSkipRemainingSteps(t *testing.T) {
	ctx := context.Background()
	actions := map[string]func(Service, TaskActionRequest) (*TaskDTO, error){
		"approve": func(s Service, r TaskActionRequest) (*TaskDTO, error) { return s.ApproveTask(ctx, r) },
		"review":  func(s Service, r TaskActionRequest) (*TaskDTO, error) { return s.ReviewTask(ctx, r) },
		"confirm": func(s Service, r TaskActionRequest) (*TaskDTO, error) { return s.ConfirmTask(ctx, r) },
	}
	for _, source := range []string{"global_template", "company_override"} {
		for name, act := range actions {
			repo, updater, svc, instanceID := newTemplateWorkflow(t, source, "prepare", "review", "publish")
			first := pendingTask(t, repo, "c1", instanceID)

			repo.completedSteps = []string{"prepare"}
			_, err := act(svc, TaskActionRequest{Subject: templateSubmitter, TaskID: first.TaskID})
			he, ok := perr.AsHTTPError(err)
			if !ok || he.HTTPStatus != http.StatusConflict {
				t.Fatalf("%s/%s on step 1 with steps 2..3 open: err=%v, want 409", source, name, err)
			}
			if he.Code != perr.CodeWorkflowStepsIncomplete || !slices.Equal(he.Details["missing_step_codes"].([]string), []string{"review", "publish"}) {
				t.Fatalf("%s/%s: code=%s details=%v, want %s missing [review publish]", source, name, he.Code, he.Details, perr.CodeWorkflowStepsIncomplete)
			}
			inst, _ := repo.FindInstance(ctx, "c1", instanceID)
			if inst.Status != "in_progress" {
				t.Fatalf("%s/%s: instance status=%q, want in_progress", source, name, inst.Status)
			}
			if updater.calls != 0 {
				t.Fatalf("%s/%s: record approved %d times before its steps were done", source, name, updater.calls)
			}
			if task, _ := repo.FindTask(ctx, "c1", first.TaskID); task.Status != "pending" {
				t.Fatalf("%s/%s: task status=%q, want pending", source, name, task.Status)
			}
		}
	}
}

func TestTemplateWorkflow_StepOneTaskOffersNoCompletionWhileStepsOpen(t *testing.T) {
	_, _, svc, instanceID := newTemplateWorkflow(t, "global_template", "prepare", "review")
	tasks, err := svc.ListInstanceTasks(context.Background(), ListInstanceTasksRequest{Subject: templateSubmitter, WorkflowInstanceID: instanceID})
	if err != nil || len(tasks) != 1 {
		t.Fatalf("tasks=%v err=%v", tasks, err)
	}
	for _, action := range []string{"approve", "review", "confirm"} {
		if slices.Contains(tasks[0].AvailableActions, action) {
			t.Fatalf("available_actions=%v offers %q while steps are open", tasks[0].AvailableActions, action)
		}
	}
}

func TestTemplateWorkflow_CompletesOnceEveryStepIsDone(t *testing.T) {
	ctx := context.Background()
	repo, updater, svc, instanceID := newTemplateWorkflow(t, "global_template", "prepare", "review")
	repo.completedSteps = []string{"prepare", "review"}

	tasks, err := svc.ListInstanceTasks(ctx, ListInstanceTasksRequest{Subject: templateSubmitter, WorkflowInstanceID: instanceID})
	if err != nil || !slices.Contains(tasks[0].AvailableActions, "approve") {
		t.Fatalf("available_actions=%v err=%v, want approve once every step is done", tasks[0].AvailableActions, err)
	}
	if _, err := svc.ApproveTask(ctx, TaskActionRequest{Subject: templateSubmitter, TaskID: tasks[0].TaskID}); err != nil {
		t.Fatalf("approve after every step is done: %v", err)
	}
	inst, _ := repo.FindInstance(ctx, "c1", instanceID)
	if inst.Status != "approved" || updater.calls != 1 {
		t.Fatalf("instance=%q record approvals=%d, want approved once", inst.Status, updater.calls)
	}
}

// WF-02: completing a task completes its step, so the repository must run the required-document
// gate for that step. Rejecting does not complete anything.
func TestTaskCompletion_AsksForTheRequiredDocumentGate(t *testing.T) {
	ctx := context.Background()
	repo, _, svc, instanceID := newTemplateWorkflow(t, "global_template", "prepare")
	repo.completedSteps = []string{"prepare"}
	first := pendingTask(t, repo, "c1", instanceID)
	if _, err := svc.ApproveTask(ctx, TaskActionRequest{Subject: templateSubmitter, TaskID: first.TaskID}); err != nil {
		t.Fatal(err)
	}
	got := repo.lastTransition
	if !got.GateRequiredDocuments || got.StepCode != "prepare" || got.WorkflowInstanceID != instanceID {
		t.Fatalf("approve transition=%+v, want the gate for step prepare", got)
	}

	repo2, _, svc2, instanceID2 := newTemplateWorkflow(t, "global_template", "prepare")
	second := pendingTask(t, repo2, "c1", instanceID2)
	if _, err := svc2.RejectTask(ctx, TaskActionRequest{Subject: templateSubmitter, TaskID: second.TaskID}); err != nil {
		t.Fatal(err)
	}
	if repo2.lastTransition.GateRequiredDocuments {
		t.Fatal("reject must not run the required-document gate")
	}
}

// Review DB-01: a frozen proposal advances by task, while the fulfillment upload takes the current
// step from the step runtime. Each completed task must complete its step there, or step 2 never
// becomes current and its required documents can never be uploaded.
func TestProposalTask_CompletesItsStepInTheStepRuntime(t *testing.T) {
	ctx := context.Background()
	repo, svc, _ := seedV2ThreeStep(t)
	instID := repo.createdInstance.WorkflowInstanceID
	first := pendingTask(t, repo, "c1", instID)

	if _, err := svc.ApproveTask(ctx, TaskActionRequest{Subject: Subject{UserID: "u", MembershipID: "m1", CompanyID: "c1"}, TaskID: first.TaskID}); err != nil {
		t.Fatal(err)
	}
	got := repo.lastTransition
	if got.CompleteStepBy != "m1" || got.StepCode != "s1" || got.StepCompletedAt.IsZero() {
		t.Fatalf("approve transition=%+v, want step s1 completed by m1", got)
	}
	if !slices.Equal(repo.completedSteps, []string{"s1"}) {
		t.Fatalf("completed steps=%v, want [s1]", repo.completedSteps)
	}
}

// A template workflow completes its steps on the deadline screen; its step-1 task writes none.
func TestTemplateTask_DoesNotWriteTheStepRuntime(t *testing.T) {
	repo, _, svc, instanceID := newTemplateWorkflow(t, "global_template", "prepare")
	repo.completedSteps = []string{"prepare"}
	first := pendingTask(t, repo, "c1", instanceID)
	if _, err := svc.ApproveTask(context.Background(), TaskActionRequest{Subject: templateSubmitter, TaskID: first.TaskID}); err != nil {
		t.Fatal(err)
	}
	if repo.lastTransition.CompleteStepBy != "" {
		t.Fatalf("template approve transition=%+v, want no step completion", repo.lastTransition)
	}
}
