package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
	relationIDs := normalizeAssigneeIDs(task.AssigneeMembershipIDs)
	singular := strings.TrimSpace(task.AssigneeMembershipID)
	if len(relationIDs) > 0 && singular != "" {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest,
			"task_assignment_contract_conflict: singular and relation assignees cannot both be set", nil)
	}
	if len(relationIDs) == 0 && singular == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "task assignee is required", nil)
	}

	if len(relationIDs) == 0 {
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO workflow_tasks (
				task_id, company_id, workflow_instance_id, step_code, assignee_membership_id, status
			) VALUES (?, ?, ?, ?, ?, ?)
		`, task.TaskID, task.CompanyID, task.WorkflowInstanceID, task.StepCode, singular, task.Status)
		if err != nil {
			return nil, fmt.Errorf("workflow task insert: %w", err)
		}
		cp := task
		cp.AssigneeMembershipID = singular
		cp.AssigneeMembershipIDs = nil
		return &cp, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create task: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_tasks (
			task_id, company_id, workflow_instance_id, step_code, assignee_membership_id, status
		) VALUES (?, ?, ?, ?, NULL, ?)
	`, task.TaskID, task.CompanyID, task.WorkflowInstanceID, task.StepCode, task.Status); err != nil {
		return nil, fmt.Errorf("workflow task insert: %w", err)
	}
	if err := insertTaskAssigneesTx(ctx, tx, task.TaskID, relationIDs); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit create task: %w", err)
	}
	cp := task
	cp.AssigneeMembershipID = ""
	cp.AssigneeMembershipIDs = relationIDs
	return &cp, nil
}

func (r *Repository) FindTask(ctx context.Context, companyID, taskID string) (*workflowapp.TaskDTO, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT task_id, company_id, workflow_instance_id, step_code, assignee_membership_id, status
		FROM workflow_tasks WHERE company_id = ? AND task_id = ?
	`, companyID, taskID)
	var t workflowapp.TaskDTO
	var assignee sql.NullString
	if err := row.Scan(&t.TaskID, &t.CompanyID, &t.WorkflowInstanceID, &t.StepCode, &assignee, &t.Status); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(404, perr.CodeInvalidRequest, "task not found", nil)
		}
		return nil, err
	}
	if assignee.Valid {
		t.AssigneeMembershipID = assignee.String
	}
	ids, err := r.listTaskAssigneeMembershipIDs(ctx, t.TaskID)
	if err != nil {
		return nil, err
	}
	t.AssigneeMembershipIDs = ids
	if len(ids) > 0 && strings.TrimSpace(t.AssigneeMembershipID) != "" {
		// Contract drift: do not merge; relation wins for auth via ResolveTaskAssigneeMembershipIDs.
		_ = fmt.Errorf("task assignment dual authority detected task_id=%s", t.TaskID)
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
		relationIDs := normalizeAssigneeIDs(nt.AssigneeMembershipIDs)
		singular := strings.TrimSpace(nt.AssigneeMembershipID)
		if len(relationIDs) > 0 && singular != "" {
			return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest,
				"task_assignment_contract_conflict: singular and relation assignees cannot both be set", nil)
		}
		if len(relationIDs) == 0 && singular == "" {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "next task assignee is required", nil)
		}
		assigneeArg := any(nil)
		if len(relationIDs) == 0 {
			assigneeArg = singular
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO workflow_tasks (
				task_id, company_id, workflow_instance_id, step_code, assignee_membership_id, status
			) VALUES (?, ?, ?, ?, ?, ?)
		`, nt.TaskID, nt.CompanyID, nt.WorkflowInstanceID, nt.StepCode, assigneeArg, nt.Status); err != nil {
			return nil, fmt.Errorf("insert next task: %w", err)
		}
		if len(relationIDs) > 0 {
			if err := insertTaskAssigneesTx(ctx, tx, nt.TaskID, relationIDs); err != nil {
				return nil, err
			}
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

	return r.FindTask(ctx, in.CompanyID, in.TaskID)
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
	var taskIDs []string
	for rows.Next() {
		var t workflowapp.TaskDTO
		var assignee sql.NullString
		var displayName, email, departmentName string
		if err := rows.Scan(
			&t.TaskID,
			&t.CompanyID,
			&t.WorkflowInstanceID,
			&t.StepCode,
			&assignee,
			&t.Status,
			&displayName,
			&email,
			&departmentName,
		); err != nil {
			return nil, err
		}
		if assignee.Valid {
			t.AssigneeMembershipID = assignee.String
		}
		if t.AssigneeMembershipID != "" {
			t.Assignee = workflowapp.BuildTaskAssignee(
				t.AssigneeMembershipID,
				displayName,
				email,
				departmentName,
			)
		}
		out = append(out, t)
		taskIDs = append(taskIDs, t.TaskID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	byTask, err := r.listAssigneesForTasks(ctx, taskIDs)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if ids := byTask[out[i].TaskID]; len(ids) > 0 {
			out[i].AssigneeMembershipIDs = ids
		}
	}
	return out, nil
}

func (r *Repository) listTaskAssigneeMembershipIDs(ctx context.Context, taskID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT membership_id FROM workflow_task_assignees
		WHERE task_id = ?
		ORDER BY membership_id ASC
	`, taskID)
	if err != nil {
		// Table may not exist until migration applied; treat as empty for mixed-version safety during M2 source-only.
		if isUnknownTable(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if id = strings.TrimSpace(id); id != "" {
			out = append(out, id)
		}
	}
	return out, rows.Err()
}

func (r *Repository) listAssigneesForTasks(ctx context.Context, taskIDs []string) (map[string][]string, error) {
	out := map[string][]string{}
	if len(taskIDs) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(taskIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, 0, len(taskIDs))
	for _, id := range taskIDs {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT task_id, membership_id FROM workflow_task_assignees
		WHERE task_id IN (`+placeholders+`)
		ORDER BY task_id ASC, membership_id ASC
	`, args...)
	if err != nil {
		if isUnknownTable(err) {
			return out, nil
		}
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var taskID, membershipID string
		if err := rows.Scan(&taskID, &membershipID); err != nil {
			return nil, err
		}
		out[taskID] = append(out[taskID], membershipID)
	}
	return out, rows.Err()
}

func insertTaskAssigneesTx(ctx context.Context, tx *sql.Tx, taskID string, membershipIDs []string) error {
	for _, mid := range membershipIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO workflow_task_assignees (task_id, membership_id) VALUES (?, ?)
		`, taskID, mid); err != nil {
			return fmt.Errorf("insert workflow_task_assignees: %w", err)
		}
	}
	return nil
}

func normalizeAssigneeIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func isUnknownTable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "workflow_task_assignees") &&
		(strings.Contains(msg, "doesn't exist") || strings.Contains(msg, "does not exist") || strings.Contains(msg, "unknown table"))
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
