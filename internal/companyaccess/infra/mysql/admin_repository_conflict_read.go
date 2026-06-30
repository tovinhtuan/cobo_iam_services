package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cobo/cobo_iam_services/internal/companyaccess/conflict"
)

// ListStaleWorkflowOverridesByCompany returns active overrides with stale_status=stale.
func (r *AdminRepository) ListStaleWorkflowOverridesByCompany(ctx context.Context, companyID string) ([]conflict.StaleWorkflowOverrideRow, error) {
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT type_id, stale_status, active_version_no, last_rebase_check_at
		FROM company_template_workflow_overrides
		WHERE company_id = ? AND active_version_no > 0 AND stale_status = 'stale'
		ORDER BY type_id
		LIMIT 20
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list stale workflow overrides: %w", err)
	}
	defer rows.Close()
	var out []conflict.StaleWorkflowOverrideRow
	for rows.Next() {
		var row conflict.StaleWorkflowOverrideRow
		var lastCheck sql.NullTime
		if err := rows.Scan(&row.TypeID, &row.StaleStatus, &row.ActiveVersionNo, &lastCheck); err != nil {
			return nil, fmt.Errorf("scan stale override: %w", err)
		}
		if lastCheck.Valid {
			t := lastCheck.Time.UTC()
			row.LastRebaseCheckAt = &t
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ListWorkflowAssigneeRulesByCompany returns active workflow assignee rules for a company.
func (r *AdminRepository) ListWorkflowAssigneeRulesByCompany(ctx context.Context, companyID string) ([]conflict.WorkflowAssigneeRuleRow, error) {
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT workflow_assignee_rule_id, rule_code, payload_json
		FROM workflow_assignee_rules
		WHERE company_id = ? AND status = 'active'
		ORDER BY rule_code
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list workflow assignee rules: %w", err)
	}
	defer rows.Close()
	var out []conflict.WorkflowAssigneeRuleRow
	for rows.Next() {
		var id, code string
		var raw []byte
		if err := rows.Scan(&id, &code, &raw); err != nil {
			return nil, fmt.Errorf("scan workflow assignee rule: %w", err)
		}
		payload := map[string]any{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &payload)
		}
		out = append(out, conflict.WorkflowAssigneeRuleRow{
			RuleID:   id,
			RuleCode: code,
			Payload:  payload,
		})
	}
	return out, rows.Err()
}

// ListInactiveDepartmentsWithMembers returns inactive departments that still have active members.
func (r *AdminRepository) ListInactiveDepartmentsWithMembers(ctx context.Context, companyID string) ([]conflict.InactiveDepartmentRow, error) {
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT d.department_id, d.department_name,
		       (SELECT COUNT(*) FROM department_memberships dm
		        WHERE dm.department_id = d.department_id AND dm.status = 'active') AS member_count
		FROM departments d
		WHERE d.company_id = ? AND d.status = 'inactive'
		HAVING member_count > 0
		ORDER BY d.department_name
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list inactive departments with members: %w", err)
	}
	defer rows.Close()
	var out []conflict.InactiveDepartmentRow
	for rows.Next() {
		var row conflict.InactiveDepartmentRow
		if err := rows.Scan(&row.DepartmentID, &row.DepartmentName, &row.MemberCount); err != nil {
			return nil, fmt.Errorf("scan inactive department: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ListActiveDirectPermissionsByCompany returns non-revoked direct grants for a company.
func (r *AdminRepository) ListActiveDirectPermissionsByCompany(ctx context.Context, companyID string) ([]conflict.DirectPermissionRow, error) {
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT membership_id, permission_code
		FROM membership_direct_permissions
		WHERE company_id = ? AND revoked_at IS NULL
		ORDER BY permission_code, membership_id
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list direct permissions by company: %w", err)
	}
	defer rows.Close()
	var out []conflict.DirectPermissionRow
	for rows.Next() {
		var row conflict.DirectPermissionRow
		if err := rows.Scan(&row.MembershipID, &row.PermissionCode); err != nil {
			return nil, fmt.Errorf("scan direct permission: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}