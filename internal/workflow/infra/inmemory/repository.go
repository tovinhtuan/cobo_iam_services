package inmemory

import (
	"context"
	"net/http"
	"sync"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

type Repository struct {
	mu        sync.RWMutex
	instances map[string]workflowapp.WorkflowInstanceDTO
	tasks     map[string]workflowapp.TaskDTO
}

func NewRepository() *Repository {
	return &Repository{instances: map[string]workflowapp.WorkflowInstanceDTO{}, tasks: map[string]workflowapp.TaskDTO{}}
}

func ikey(companyID, id string) string { return companyID + ":" + id }

func (r *Repository) CreateInstance(_ context.Context, in workflowapp.WorkflowInstanceDTO) (*workflowapp.WorkflowInstanceDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.instances[ikey(in.CompanyID, in.WorkflowInstanceID)] = in
	cp := in
	return &cp, nil
}

func (r *Repository) FindInstance(_ context.Context, companyID, workflowInstanceID string) (*workflowapp.WorkflowInstanceDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	in, ok := r.instances[ikey(companyID, workflowInstanceID)]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "workflow instance not found", nil)
	}
	cp := in
	return &cp, nil
}

func (r *Repository) UpdateInstance(_ context.Context, in workflowapp.WorkflowInstanceDTO) (*workflowapp.WorkflowInstanceDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := ikey(in.CompanyID, in.WorkflowInstanceID)
	if _, ok := r.instances[k]; !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "workflow instance not found", nil)
	}
	r.instances[k] = in
	cp := in
	return &cp, nil
}

func (r *Repository) CreateTask(_ context.Context, task workflowapp.TaskDTO) (*workflowapp.TaskDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[ikey(task.CompanyID, task.TaskID)] = task
	cp := task
	return &cp, nil
}

func (r *Repository) FindTask(_ context.Context, companyID, taskID string) (*workflowapp.TaskDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tasks[ikey(companyID, taskID)]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "task not found", nil)
	}
	cp := t
	return &cp, nil
}

func (r *Repository) UpdateTask(_ context.Context, task workflowapp.TaskDTO) (*workflowapp.TaskDTO, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := ikey(task.CompanyID, task.TaskID)
	if _, ok := r.tasks[k]; !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "task not found", nil)
	}
	r.tasks[k] = task
	cp := task
	return &cp, nil
}

func (r *Repository) ListTasksByInstance(_ context.Context, companyID, workflowInstanceID string) ([]workflowapp.TaskDTO, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]workflowapp.TaskDTO, 0)
	for _, t := range r.tasks {
		if t.CompanyID == companyID && t.WorkflowInstanceID == workflowInstanceID {
			out = append(out, t)
		}
	}
	return out, nil
}
