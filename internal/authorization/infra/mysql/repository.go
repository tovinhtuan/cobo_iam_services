package mysql

import (
	"context"
	"database/sql"
	"fmt"

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
		SELECT DISTINCT p.permission_code
		FROM memberships m
		INNER JOIN membership_roles mr ON mr.membership_id = m.membership_id AND mr.status = 'active'
		INNER JOIN roles r ON r.role_id = mr.role_id AND r.status = 'active'
		INNER JOIN role_permissions rp ON rp.role_id = r.role_id AND rp.status = 'active'
		INNER JOIN permissions p ON p.permission_id = rp.permission_id AND p.status = 'active'
		WHERE m.membership_id = ? AND m.company_id = ?
		  AND (r.company_id IS NULL OR r.company_id = m.company_id)
		ORDER BY p.permission_code
	`, membershipID, companyID)
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
