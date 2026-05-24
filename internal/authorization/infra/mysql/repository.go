package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListPermissionCodes(ctx context.Context, membershipID, companyID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT permission_code FROM (
			SELECT p.permission_code
			FROM memberships m
			INNER JOIN membership_roles mr ON mr.membership_id = m.membership_id AND mr.status = 'active'
			INNER JOIN roles r ON r.role_id = mr.role_id AND r.status = 'active'
			INNER JOIN role_permissions rp ON rp.role_id = r.role_id AND rp.status = 'active'
			INNER JOIN permissions p ON p.permission_id = rp.permission_id AND p.status = 'active'
			WHERE m.membership_id = ? AND m.company_id = ?
			  AND (r.company_id IS NULL OR r.company_id = m.company_id)

			UNION ALL

			SELECT mdp.permission_code
			FROM membership_direct_permissions mdp
			WHERE mdp.membership_id = ? AND mdp.company_id = ?
			  AND mdp.revoked_at IS NULL
		) AS combined
		ORDER BY permission_code
	`, membershipID, companyID, membershipID, companyID)
	if err != nil {
		return nil, fmt.Errorf("list permission codes: %w", err)
	}
	defer rows.Close()
	return scanStringCol(rows)
}

func (r *Repository) ListDepartmentScopes(ctx context.Context, membershipID, companyID string) ([]authapp.DepartmentScope, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT d.department_id, d.department_name
		FROM department_memberships dm
		INNER JOIN departments d ON d.department_id = dm.department_id AND d.company_id = ?
		WHERE dm.membership_id = ? AND dm.status = 'active' AND d.status = 'active'
		ORDER BY d.department_name
	`, companyID, membershipID)
	if err != nil {
		return nil, fmt.Errorf("list department scopes: %w", err)
	}
	defer rows.Close()
	var out []authapp.DepartmentScope
	for rows.Next() {
		var d authapp.DepartmentScope
		if err := rows.Scan(&d.DepartmentID, &d.DepartmentName); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repository) ListAssignments(ctx context.Context, membershipID, companyID string) ([]authapp.ResourceAssignment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT resource_type, resource_id
		FROM assignments
		WHERE company_id = ? AND assignee_type = 'membership' AND assignee_ref_id = ? AND status = 'active'
		ORDER BY resource_type, resource_id
	`, companyID, membershipID)
	if err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}
	defer rows.Close()
	var out []authapp.ResourceAssignment
	for rows.Next() {
		var a authapp.ResourceAssignment
		if err := rows.Scan(&a.ResourceType, &a.ResourceID); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *Repository) ListResponsibilities(ctx context.Context, membershipID, companyID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT responsibility_code
		FROM membership_effective_responsibilities
		WHERE company_id = ? AND membership_id = ?
		ORDER BY responsibility_code
	`, companyID, membershipID)
	if err != nil {
		return nil, fmt.Errorf("list responsibilities: %w", err)
	}
	defer rows.Close()
	return scanStringCol(rows)
}

func (r *Repository) ListPositionCodes(ctx context.Context, membershipID, companyID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT LOWER(t.title_code) AS position_code
		FROM membership_titles mt
		INNER JOIN titles t ON t.title_id = mt.title_id AND t.company_id = ? AND t.status = 'active'
		WHERE mt.membership_id = ? AND mt.status = 'active'
		ORDER BY position_code
	`, companyID, membershipID)
	if err != nil {
		return nil, fmt.Errorf("list position codes: %w", err)
	}
	defer rows.Close()
	return scanStringCol(rows)
}

func (r *Repository) ListOrgUnitIDs(ctx context.Context, membershipID, companyID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT oum.org_unit_id
		FROM org_unit_memberships oum
		INNER JOIN org_units ou ON ou.org_unit_id = oum.org_unit_id AND ou.company_id = ? AND ou.status = 'active'
		WHERE oum.membership_id = ? AND oum.status = 'active'
		ORDER BY oum.org_unit_id
	`, companyID, membershipID)
	if err != nil {
		return nil, fmt.Errorf("list org unit ids: %w", err)
	}
	defer rows.Close()
	return scanStringCol(rows)
}

func (r *Repository) ListOrgSubtreeUnitIDs(ctx context.Context, membershipID, companyID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT c.descendant_org_unit_id
		FROM org_unit_memberships oum
		INNER JOIN org_unit_closure c ON c.ancestor_org_unit_id = oum.org_unit_id
		INNER JOIN org_units ou ON ou.org_unit_id = c.descendant_org_unit_id AND ou.company_id = ? AND ou.status = 'active'
		WHERE oum.membership_id = ? AND oum.status = 'active'
		ORDER BY c.descendant_org_unit_id
	`, companyID, membershipID)
	if err != nil {
		return nil, fmt.Errorf("list org subtree unit ids: %w", err)
	}
	defer rows.Close()
	return scanStringCol(rows)
}

func (r *Repository) GetActionPolicy(ctx context.Context, companyID, action string) (*authapp.ActionPolicy, error) {
	var p authapp.ActionPolicy
	err := r.db.QueryRowContext(ctx, `
		SELECT action_code, required_permission, scope_type, workflow_state, eligible_actor, effect_type, deny_reason_code
		FROM action_policy_matrix
		WHERE status = 'active' AND action_code = ? AND (company_id = ? OR company_id IS NULL)
		ORDER BY company_id IS NULL ASC
		LIMIT 1
	`, strings.TrimSpace(action), companyID).Scan(
		&p.ActionCode, &p.RequiredPermission, &p.ScopeType, &p.WorkflowState, &p.EligibleActor, &p.EffectType, &p.DenyReasonCode,
	)
	if err == nil {
		action = strings.TrimSpace(action)
		legacy := legacyPolicy(action)
		// Mis-seeded matrix rows often default to system.settings; prefer explicit legacy mapping
		// for actions like company.view when the user holds the action permission but not system.settings.
		if legacy.RequiredPermission != "system.settings" && p.RequiredPermission == "system.settings" {
			return legacy, nil
		}
		return &p, nil
	}
	if isMySQLMissingTable(err) {
		// Compatibility fallback for environments not yet migrated with action_policy_matrix.
		return legacyPolicy(strings.TrimSpace(action)), nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("get action policy: %w", err)
	}
	// Safe fallback for environments not yet seeded with action_policy_matrix.
	return legacyPolicy(strings.TrimSpace(action)), nil
}

func isMySQLMissingTable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "doesn't exist") || strings.Contains(msg, "error 1146")
}

func scanStringCol(rows *sql.Rows) ([]string, error) {
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func legacyPolicy(action string) *authapp.ActionPolicy {
	required := "system.settings"
	switch action {
	case "disclosure.approve":
		required = "disclosure.approve"
	case "disclosure.view":
		required = "disclosure.view"
	case "disclosure.create":
		required = "disclosure.create"
	case "disclosure.update", "disclosure.edit":
		required = "disclosure.edit"
	case "disclosure.submit":
		required = "disclosure.publish"
	case "workflow.create":
		required = "deadline.create"
	case "workflow.read":
		required = "disclosure.view"
	case "workflow.review":
		required = "deadline.assign"
	case "workflow.approve":
		required = "disclosure.approve"
	case "workflow.confirm":
		required = "deadline.manage"
	case "workflow.override":
		required = "workflow.step.override"
	case "workflow.resolve_assignees":
		required = "deadline.create"
	case "template.workflow.override.read":
		required = "template.workflow.override.read"
	case "template.workflow.override.write":
		required = "template.workflow.override.write"
	case "template.workflow.override.approve":
		required = "template.workflow.override.approve"
	case "template.workflow.override.reset":
		required = "template.workflow.override.reset"
	case "notification.enqueue":
		required = "alert.channels.manage"
	case "notification.dispatch":
		required = "alert.channels.manage"
	case "notification.resolve_recipients":
		required = "alert.channels.manage"
	case "admin.account.settings.read", "admin.account.settings.update":
		required = "system.settings"
	case "dashboard.view":
		required = "dashboard.view"
	case "company.view":
		required = "company.view"
	case "company.edit":
		required = "company.edit"
	case "ad_hoc_alert.read":
		required = "ad_hoc_alert.read"
	case "ad_hoc_alert.propose":
		required = "ad_hoc_alert.propose"
	case "disclosure_type.manage":
		required = "disclosure_type.manage"
	case "ad_hoc_alert.focal_review":
		required = "ad_hoc_alert.focal_review"
	case "ad_hoc_alert.admin_review":
		required = "ad_hoc_alert.admin_review"
	case "admin.membership.create",
		"admin.membership.update",
		"admin.membership.delete",
		"admin.membership.list",
		"admin.membership.role.assign",
		"admin.membership.role.remove",
		"admin.membership.department.assign",
		"admin.membership.department.remove",
		"admin.membership.title.assign",
		"admin.membership.title.remove",
		"admin.permissions.list",
		"admin.roles.list",
		"admin.role.permission.assign",
		"admin.role.permission.remove",
		"admin.resource_scope_rule.create",
		"admin.workflow_assignee_rule.create",
		"admin.notification_rule.create",
		"admin.notification_rule.list",
		"admin.notification_rule.update",
		"admin.notification_rule.delete":
		required = "rbac.manage"
	}
	return &authapp.ActionPolicy{
		ActionCode:         action,
		RequiredPermission: required,
		ScopeType:          "*",
		WorkflowState:      "*",
		EligibleActor:      "*",
		EffectType:         "allow",
		DenyReasonCode:     "permission_denied",
	}
}
