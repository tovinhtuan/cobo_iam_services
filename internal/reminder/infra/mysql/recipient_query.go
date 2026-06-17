package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
)

// MembershipEmailQuerier implements reminderapp.MembershipEmailQuerier via MySQL.
// All queries are scoped to companyID to enforce tenant isolation.
type MembershipEmailQuerier struct {
	db *sql.DB
}

func NewMembershipEmailQuerier(db *sql.DB) *MembershipEmailQuerier {
	return &MembershipEmailQuerier{db: db}
}

var _ reminderapp.MembershipEmailQuerier = (*MembershipEmailQuerier)(nil)
var _ reminderapp.WorkflowStepReader = (*GlobalWorkflowStepReader)(nil)

// EmailsByDepartments returns active user emails for memberships in the given departments,
// scoped to companyID. Falls back to login_id when email is empty.
func (q *MembershipEmailQuerier) EmailsByDepartments(ctx context.Context, companyID string, departmentIDs []string) ([]string, error) {
	if companyID == "" || len(departmentIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(departmentIDs))
	placeholders = placeholders[:len(placeholders)-1]

	query := `
		SELECT DISTINCT COALESCE(NULLIF(TRIM(u.email), ''), u.login_id)
		FROM department_memberships dm
		JOIN memberships m ON dm.membership_id = m.membership_id
		JOIN users u ON m.user_id = u.user_id
		WHERE dm.department_id IN (` + placeholders + `)
		  AND m.company_id = ?
		  AND m.membership_status = 'active'
		  AND dm.status = 'active'
		  AND u.account_status = 'active'
	`
	args := make([]any, 0, len(departmentIDs)+1)
	for _, id := range departmentIDs {
		args = append(args, id)
	}
	args = append(args, companyID)

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("emails by departments: %w", err)
	}
	defer rows.Close()
	return scanEmailRows(rows)
}

// EmailsByRoles returns active user emails for memberships holding the given role IDs,
// scoped to companyID. If departmentID is non-empty, further filters to users in that department.
func (q *MembershipEmailQuerier) EmailsByRoles(ctx context.Context, companyID string, roleIDs []string, departmentID string) ([]string, error) {
	if companyID == "" || len(roleIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(roleIDs))
	placeholders = placeholders[:len(placeholders)-1]

	args := make([]any, 0, len(roleIDs)+3)
	for _, id := range roleIDs {
		args = append(args, id)
	}
	args = append(args, companyID)

	deptFilter := ""
	if strings.TrimSpace(departmentID) != "" {
		deptFilter = `
		  AND m.membership_id IN (
		      SELECT membership_id FROM department_memberships
		      WHERE department_id = ? AND status = 'active'
		  )`
		args = append(args, departmentID)
	}

	query := `
		SELECT DISTINCT COALESCE(NULLIF(TRIM(u.email), ''), u.login_id)
		FROM membership_roles mr
		JOIN memberships m ON mr.membership_id = m.membership_id
		JOIN users u ON m.user_id = u.user_id
		WHERE mr.role_id IN (` + placeholders + `)
		  AND m.company_id = ?
		  AND m.membership_status = 'active'
		  AND mr.status = 'active'
		  AND u.account_status = 'active'` + deptFilter

	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("emails by roles: %w", err)
	}
	defer rows.Close()
	return scanEmailRows(rows)
}

// AdminEmailsByCompany returns emails of active members with role_code='admin_doanh_nghiep'.
// Used as the last-resort fallback when no department/role/task-assignee resolves to a recipient.
func (q *MembershipEmailQuerier) AdminEmailsByCompany(ctx context.Context, companyID string) ([]string, error) {
	if companyID == "" {
		return nil, nil
	}
	rows, err := q.db.QueryContext(ctx, `
		SELECT DISTINCT COALESCE(NULLIF(TRIM(u.email), ''), u.login_id)
		FROM memberships m
		JOIN membership_roles mr ON mr.membership_id = m.membership_id
		JOIN roles r ON r.role_id = mr.role_id
		JOIN users u ON u.user_id = m.user_id
		WHERE m.company_id = ?
		  AND r.role_code = 'admin_doanh_nghiep'
		  AND m.membership_status = 'active'
		  AND mr.status = 'active'
		  AND r.status = 'active'
		  AND u.account_status = 'active'
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("admin emails by company: %w", err)
	}
	defer rows.Close()
	return scanEmailRows(rows)
}

// AssigneeEmailsByStep returns the assignee email for a workflow task on the given step.
func (q *MembershipEmailQuerier) AssigneeEmailsByStep(ctx context.Context, companyID, workflowInstanceID, stepCode string) ([]string, error) {
	if companyID == "" || workflowInstanceID == "" || strings.TrimSpace(stepCode) == "" {
		return nil, nil
	}
	rows, err := q.db.QueryContext(ctx, `
		SELECT DISTINCT COALESCE(NULLIF(TRIM(u.email), ''), u.login_id)
		FROM workflow_tasks wt
		JOIN memberships m ON m.membership_id = wt.assignee_membership_id
		JOIN users u ON u.user_id = m.user_id
		WHERE wt.company_id = ?
		  AND wt.workflow_instance_id = ?
		  AND wt.step_code = ?
		  AND wt.status IN ('pending', 'reviewed')
		  AND m.company_id = ?
		  AND m.membership_status = 'active'
		  AND u.account_status = 'active'
	`, companyID, workflowInstanceID, stepCode, companyID)
	if err != nil {
		return nil, fmt.Errorf("assignee emails by workflow step: %w", err)
	}
	defer rows.Close()
	return scanEmailRows(rows)
}

func scanEmailRows(rows *sql.Rows) ([]string, error) {
	var out []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		if e := strings.TrimSpace(email); e != "" {
			out = append(out, e)
		}
	}
	return out, rows.Err()
}

// GlobalWorkflowStepReader implements reminderapp.globalWorkflowStepReader via MySQL.
type GlobalWorkflowStepReader struct {
	db *sql.DB
}

func NewGlobalWorkflowStepReader(db *sql.DB) *GlobalWorkflowStepReader {
	return &GlobalWorkflowStepReader{db: db}
}

// GetStepByID looks up a global_workflow_step by step_id and returns stage name,
// assignee role IDs and optional department_id.
func (r *GlobalWorkflowStepReader) GetStepByID(ctx context.Context, stepID string) (*reminderapp.WorkflowStepConfig, error) {
	var stageName sql.NullString
	var assigneeRoleIDsJSON []byte
	var deptID sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT NULLIF(stage, ''), assignee_role_ids, NULLIF(department_id, '')
		FROM global_workflow_steps
		WHERE step_id = ?
	`, stepID).Scan(&stageName, &assigneeRoleIDsJSON, &deptID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get workflow step %s: %w", stepID, err)
	}
	return &reminderapp.WorkflowStepConfig{
		StepID:          stepID,
		StageName:       stageName.String,
		AssigneeRoleIDs: reminderapp.ParseAssigneeRoleIDs(assigneeRoleIDsJSON),
		DepartmentID:    deptID.String,
	}, nil
}
