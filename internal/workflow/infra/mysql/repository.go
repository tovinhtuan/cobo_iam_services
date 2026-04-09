package mysql

import (
	"context"
	"database/sql"
	"fmt"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateInstance(ctx context.Context, in workflowapp.WorkflowInstanceDTO) (*workflowapp.WorkflowInstanceDTO, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO workflow_instances (
			workflow_instance_id, company_id, record_id, status, current_step_code, created_by
		) VALUES (?, ?, ?, ?, ?, ?)
	`, in.WorkflowInstanceID, in.CompanyID, in.RecordID, in.Status, in.CurrentStepCode, in.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("workflow instance insert: %w", err)
	}
	cp := in
	return &cp, nil
}

func (r *Repository) FindInstance(ctx context.Context, companyID, workflowInstanceID string) (*workflowapp.WorkflowInstanceDTO, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT workflow_instance_id, company_id, record_id, status, current_step_code, created_by
		FROM workflow_instances WHERE company_id = ? AND workflow_instance_id = ?
	`, companyID, workflowInstanceID)
	var in workflowapp.WorkflowInstanceDTO
	if err := row.Scan(&in.WorkflowInstanceID, &in.CompanyID, &in.RecordID, &in.Status, &in.CurrentStepCode, &in.CreatedBy); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(404, perr.CodeInvalidRequest, "workflow instance not found", nil)
		}
		return nil, err
	}
	return &in, nil
}

func (r *Repository) UpdateInstance(ctx context.Context, in workflowapp.WorkflowInstanceDTO) (*workflowapp.WorkflowInstanceDTO, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE workflow_instances SET status = ?, current_step_code = ?
		WHERE workflow_instance_id = ? AND company_id = ?
	`, in.Status, in.CurrentStepCode, in.WorkflowInstanceID, in.CompanyID)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, perr.NewHTTPError(404, perr.CodeInvalidRequest, "workflow instance not found", nil)
	}
	cp := in
	return &cp, nil
}

func (r *Repository) CreateTask(ctx context.Context, task workflowapp.TaskDTO) (*workflowapp.TaskDTO, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO workflow_tasks (
			task_id, company_id, workflow_instance_id, step_code, assignee_membership_id, status
		) VALUES (?, ?, ?, ?, ?, ?)
	`, task.TaskID, task.CompanyID, task.WorkflowInstanceID, task.StepCode, task.AssigneeMembershipID, task.Status)
	if err != nil {
		return nil, fmt.Errorf("workflow task insert: %w", err)
	}
	cp := task
	return &cp, nil
}

func (r *Repository) FindTask(ctx context.Context, companyID, taskID string) (*workflowapp.TaskDTO, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT task_id, company_id, workflow_instance_id, step_code, assignee_membership_id, status
		FROM workflow_tasks WHERE company_id = ? AND task_id = ?
	`, companyID, taskID)
	var t workflowapp.TaskDTO
	if err := row.Scan(&t.TaskID, &t.CompanyID, &t.WorkflowInstanceID, &t.StepCode, &t.AssigneeMembershipID, &t.Status); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(404, perr.CodeInvalidRequest, "task not found", nil)
		}
		return nil, err
	}
	return &t, nil
}

func (r *Repository) UpdateTask(ctx context.Context, task workflowapp.TaskDTO) (*workflowapp.TaskDTO, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE workflow_tasks SET status = ?
		WHERE task_id = ? AND company_id = ?
	`, task.Status, task.TaskID, task.CompanyID)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, perr.NewHTTPError(404, perr.CodeInvalidRequest, "task not found", nil)
	}
	cp := task
	return &cp, nil
}

func (r *Repository) ListTasksByInstance(ctx context.Context, companyID, workflowInstanceID string) ([]workflowapp.TaskDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT task_id, company_id, workflow_instance_id, step_code, assignee_membership_id, status
		FROM workflow_tasks WHERE company_id = ? AND workflow_instance_id = ?
	`, companyID, workflowInstanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []workflowapp.TaskDTO
	for rows.Next() {
		var t workflowapp.TaskDTO
		if err := rows.Scan(&t.TaskID, &t.CompanyID, &t.WorkflowInstanceID, &t.StepCode, &t.AssigneeMembershipID, &t.Status); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
