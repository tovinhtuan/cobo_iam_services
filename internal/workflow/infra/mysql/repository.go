package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

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
	var snapshotJSON []byte
	if len(in.Snapshot) > 0 {
		b, err := json.Marshal(in.Snapshot)
		if err != nil {
			return nil, fmt.Errorf("marshal snapshot: %w", err)
		}
		snapshotJSON = b
	}
	var t0Date any
	if in.T0Date != nil {
		t0Date = in.T0Date.Format("2006-01-02")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO workflow_instances (
			workflow_instance_id, company_id, record_id, status, current_step_code, created_by,
			snapshot_json, t0_date, t0_policy, workflow_source
		) VALUES (?, ?, ?, ?, ?, ?, CAST(? AS JSON), ?, ?, ?)
	`, in.WorkflowInstanceID, in.CompanyID, in.RecordID, in.Status, in.CurrentStepCode, in.CreatedBy,
		nullableJSON(snapshotJSON), t0Date, nullStr(in.T0Policy), nullStr(in.WorkflowSource))
	if err != nil {
		return nil, fmt.Errorf("workflow instance insert: %w", err)
	}
	cp := in
	return &cp, nil
}

func (r *Repository) FindInstance(ctx context.Context, companyID, workflowInstanceID string) (*workflowapp.WorkflowInstanceDTO, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT workflow_instance_id, company_id, record_id, status, current_step_code, created_by,
		       t0_date, t0_policy, workflow_source, snapshot_json
		FROM workflow_instances WHERE company_id = ? AND workflow_instance_id = ?
	`, companyID, workflowInstanceID)
	var in workflowapp.WorkflowInstanceDTO
	var t0Date sql.NullTime
	var t0Policy sql.NullString
	var workflowSource sql.NullString
	var snapshotJSON sql.NullString
	if err := row.Scan(
		&in.WorkflowInstanceID,
		&in.CompanyID,
		&in.RecordID,
		&in.Status,
		&in.CurrentStepCode,
		&in.CreatedBy,
		&t0Date,
		&t0Policy,
		&workflowSource,
		&snapshotJSON,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(404, perr.CodeInvalidRequest, "workflow instance not found", nil)
		}
		return nil, err
	}
	if t0Date.Valid {
		v := t0Date.Time
		in.T0Date = &v
	}
	if t0Policy.Valid {
		in.T0Policy = t0Policy.String
	}
	if workflowSource.Valid {
		in.WorkflowSource = workflowSource.String
	}
	if snapshotJSON.Valid && snapshotJSON.String != "" && snapshotJSON.String != "null" {
		if err := json.Unmarshal([]byte(snapshotJSON.String), &in.Snapshot); err != nil {
			return nil, fmt.Errorf("unmarshal snapshot_json: %w", err)
		}
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

func (r *Repository) ApplyTaskTransition(ctx context.Context, in workflowapp.TaskTransitionApply) (*workflowapp.TaskDTO, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin task transition: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
		UPDATE workflow_tasks SET status = ?
		WHERE task_id = ? AND company_id = ? AND status = ?
	`, in.ToStatus, in.TaskID, in.CompanyID, in.FromStatus)
	if err != nil {
		return nil, fmt.Errorf("update task status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "task is not pending", nil)
	}

	if in.NextTask != nil {
		nt := in.NextTask
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO workflow_tasks (
				task_id, company_id, workflow_instance_id, step_code, assignee_membership_id, status
			) VALUES (?, ?, ?, ?, ?, ?)
		`, nt.TaskID, nt.CompanyID, nt.WorkflowInstanceID, nt.StepCode, nt.AssigneeMembershipID, nt.Status); err != nil {
			return nil, fmt.Errorf("insert next task: %w", err)
		}
	}

	if in.Instance != nil {
		inst := in.Instance
		ures, err := tx.ExecContext(ctx, `
			UPDATE workflow_instances SET status = ?, current_step_code = ?
			WHERE workflow_instance_id = ? AND company_id = ?
		`, inst.Status, inst.CurrentStepCode, inst.WorkflowInstanceID, inst.CompanyID)
		if err != nil {
			return nil, fmt.Errorf("update instance: %w", err)
		}
		un, _ := ures.RowsAffected()
		if un == 0 {
			return nil, perr.NewHTTPError(404, perr.CodeInvalidRequest, "workflow instance not found", nil)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit task transition: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT task_id, company_id, workflow_instance_id, step_code, assignee_membership_id, status
		FROM workflow_tasks WHERE company_id = ? AND task_id = ?
	`, in.CompanyID, in.TaskID)
	var t workflowapp.TaskDTO
	if err := row.Scan(&t.TaskID, &t.CompanyID, &t.WorkflowInstanceID, &t.StepCode, &t.AssigneeMembershipID, &t.Status); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *Repository) ListTasksByInstance(ctx context.Context, companyID, workflowInstanceID string) ([]workflowapp.TaskDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			wt.task_id,
			wt.company_id,
			wt.workflow_instance_id,
			wt.step_code,
			wt.assignee_membership_id,
			wt.status,
			COALESCE(NULLIF(u.full_name, ''), '') AS assignee_display_name,
			COALESCE(NULLIF(u.email, ''), NULLIF(u.login_id, ''), '') AS assignee_email,
			COALESCE((
				SELECT COALESCE(NULLIF(d.department_name, ''), '')
				FROM department_memberships dm
				INNER JOIN departments d
					ON d.department_id = dm.department_id
					AND d.company_id = wt.company_id
				WHERE dm.membership_id = wt.assignee_membership_id
					AND dm.status = 'active'
				ORDER BY d.department_name ASC
				LIMIT 1
			), '') AS assignee_department_name
		FROM workflow_tasks wt
		LEFT JOIN memberships m
			ON m.membership_id = wt.assignee_membership_id
			AND m.company_id = wt.company_id
		LEFT JOIN users u ON u.user_id = m.user_id
		WHERE wt.company_id = ? AND wt.workflow_instance_id = ?
		ORDER BY wt.created_at ASC, wt.task_id ASC
	`, companyID, workflowInstanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []workflowapp.TaskDTO
	for rows.Next() {
		var t workflowapp.TaskDTO
		var displayName, email, departmentName string
		if err := rows.Scan(
			&t.TaskID,
			&t.CompanyID,
			&t.WorkflowInstanceID,
			&t.StepCode,
			&t.AssigneeMembershipID,
			&t.Status,
			&displayName,
			&email,
			&departmentName,
		); err != nil {
			return nil, err
		}
		t.Assignee = workflowapp.BuildTaskAssignee(
			t.AssigneeMembershipID,
			displayName,
			email,
			departmentName,
		)
		out = append(out, t)
	}
	return out, rows.Err()
}

func nullableJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
