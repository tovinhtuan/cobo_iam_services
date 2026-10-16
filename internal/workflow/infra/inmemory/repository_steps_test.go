package inmemory

import (
	"context"
	"net/http"
	"testing"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

// bug-workflow-step-skip-2026-10-16 (WF-01): same rule as MySQL, a transition that needs completed
// steps applies only once they are all completed.
func TestApplyTaskTransition_RequiresCompletedSteps(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository()
	inst := workflowapp.WorkflowInstanceDTO{WorkflowInstanceID: "wi-1", CompanyID: "c1", Status: "in_progress"}
	if _, _, err := repo.CreateInstanceWithFirstTask(ctx, inst, workflowapp.TaskDTO{
		TaskID: "t-1", CompanyID: "c1", WorkflowInstanceID: "wi-1", StepCode: "prepare", Status: "pending", AssigneeMembershipID: "m1",
	}); err != nil {
		t.Fatal(err)
	}
	done := inst
	done.Status = "approved"
	apply := workflowapp.TaskTransitionApply{
		CompanyID: "c1", TaskID: "t-1", FromStatus: "pending", ToStatus: "approved", Instance: &done,
		WorkflowInstanceID: "wi-1", StepCode: "prepare", RequireCompletedSteps: []string{"prepare", "review"},
	}

	repo.MarkStepCompleted("c1", "wi-1", "prepare")
	_, err := repo.ApplyTaskTransition(ctx, apply)
	if he, ok := perr.AsHTTPError(err); !ok || he.HTTPStatus != http.StatusConflict || he.Code != perr.CodeWorkflowStepsIncomplete {
		t.Fatalf("with review open: err=%v, want 409 %s", err, perr.CodeWorkflowStepsIncomplete)
	}
	if got, _ := repo.FindTask(ctx, "c1", "t-1"); got.Status != "pending" {
		t.Fatalf("task status=%q after a refused transition, want pending", got.Status)
	}

	repo.MarkStepCompleted("c1", "wi-1", "review")
	if _, err := repo.ApplyTaskTransition(ctx, apply); err != nil {
		t.Fatalf("with every step completed: %v", err)
	}
	if got, _ := repo.FindInstance(ctx, "c1", "wi-1"); got.Status != "approved" {
		t.Fatalf("instance status=%q, want approved", got.Status)
	}
}

// Review DB-01: CompleteStepBy completes the task's step, once.
func TestApplyTaskTransition_CompletesTheTaskStep(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository()
	inst := workflowapp.WorkflowInstanceDTO{WorkflowInstanceID: "wi-2", CompanyID: "c1", Status: "in_progress"}
	if _, _, err := repo.CreateInstanceWithFirstTask(ctx, inst, workflowapp.TaskDTO{
		TaskID: "t-2", CompanyID: "c1", WorkflowInstanceID: "wi-2", StepCode: "s1", Status: "pending", AssigneeMembershipID: "m1",
	}); err != nil {
		t.Fatal(err)
	}
	repo.MarkStepCompleted("c1", "wi-2", "s1")
	if _, err := repo.ApplyTaskTransition(ctx, workflowapp.TaskTransitionApply{
		CompanyID: "c1", TaskID: "t-2", FromStatus: "pending", ToStatus: "approved",
		WorkflowInstanceID: "wi-2", StepCode: "s1", CompleteStepBy: "m1",
	}); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.ListCompletedStepCodes(ctx, "c1", "wi-2"); len(got) != 1 || got[0] != "s1" {
		t.Fatalf("completed steps=%v, want [s1]", got)
	}
}
