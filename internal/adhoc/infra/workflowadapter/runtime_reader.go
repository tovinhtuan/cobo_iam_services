package workflowadapter

import (
	"context"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

// RuntimeReader adapts workflow.Repository to adhoc WorkflowRuntimeReader (read-only).
type RuntimeReader struct {
	Repo workflowapp.Repository
}

func NewRuntimeReader(repo workflowapp.Repository) *RuntimeReader {
	if repo == nil {
		return nil
	}
	return &RuntimeReader{Repo: repo}
}

func (r *RuntimeReader) FindInstanceRuntime(ctx context.Context, companyID, workflowInstanceID string) (currentStepCode, status string, err error) {
	inst, err := r.Repo.FindInstance(ctx, companyID, workflowInstanceID)
	if err != nil {
		return "", "", err
	}
	if inst == nil {
		return "", "", nil
	}
	return inst.CurrentStepCode, inst.Status, nil
}

func (r *RuntimeReader) ListInstanceTasks(ctx context.Context, companyID, workflowInstanceID string) ([]adhocapp.RuntimeTaskView, error) {
	tasks, err := r.Repo.ListTasksByInstance(ctx, companyID, workflowInstanceID)
	if err != nil {
		return nil, err
	}
	out := make([]adhocapp.RuntimeTaskView, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, adhocapp.RuntimeTaskView{
			TaskID:                t.TaskID,
			StepCode:              t.StepCode,
			Status:                t.Status,
			AssigneeMembershipID:  t.AssigneeMembershipID,
			AssigneeMembershipIDs: append([]string(nil), t.AssigneeMembershipIDs...),
		})
	}
	return out, nil
}
