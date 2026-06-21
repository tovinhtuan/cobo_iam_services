// Package mysql provides a real AssigneeDirectory over the existing identity/org tables
// (memberships, membership_roles, roles, department_memberships). No schema change.
package mysql

import (
	"context"
	"database/sql"
	"fmt"

	wfcapp "github.com/cobo/cobo_iam_services/internal/workflowconfig/app"
)

// Directory implements workflowconfig/app.AssigneeDirectory using existing tables.
type Directory struct {
	db *sql.DB
}

func NewDirectory(db *sql.DB) *Directory { return &Directory{db: db} }

// FindMembershipsByRole returns active memberships in the company holding the given role.
// The role parameter may be a roles.role_id OR roles.role_code (workflow role codes resolve by code).
func (d *Directory) FindMembershipsByRole(ctx context.Context, companyID, roleIDOrCode string) ([]string, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT DISTINCT m.membership_id
		FROM memberships m
		JOIN membership_roles mr ON mr.membership_id = m.membership_id AND mr.status = 'active'
		JOIN roles r ON r.role_id = mr.role_id AND r.status = 'active'
		WHERE m.company_id = ?
		  AND m.membership_status = 'active'
		  AND (r.role_id = ? OR r.role_code = ?)
	`, companyID, roleIDOrCode, roleIDOrCode)
	if err != nil {
		return nil, fmt.Errorf("find memberships by role: %w", err)
	}
	return scanIDs(rows)
}

// FindMembershipsByDepartment returns active memberships of the company in the given department.
func (d *Directory) FindMembershipsByDepartment(ctx context.Context, companyID, departmentID string) ([]string, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT DISTINCT m.membership_id
		FROM department_memberships dm
		JOIN memberships m ON m.membership_id = dm.membership_id
		WHERE m.company_id = ?
		  AND dm.department_id = ?
		  AND dm.status = 'active'
		  AND m.membership_status = 'active'
	`, companyID, departmentID)
	if err != nil {
		return nil, fmt.Errorf("find memberships by department: %w", err)
	}
	return scanIDs(rows)
}

// FindMembershipsByGroup is unsupported: there is no first-class group→membership mapping in source
// (groups/org_units map via departments). Conservatively report unsupported rather than guess.
func (d *Directory) FindMembershipsByGroup(_ context.Context, _, _ string) ([]string, error) {
	return nil, wfcapp.ErrUnsupportedScope
}

// FindCompanyDefaultAssignees is unsupported: there is no company-default-assignee source table.
func (d *Directory) FindCompanyDefaultAssignees(_ context.Context, _ string) ([]string, error) {
	return nil, wfcapp.ErrUnsupportedScope
}

func scanIDs(rows *sql.Rows) ([]string, error) {
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan membership id: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// compile-time assertion that Directory satisfies the interface.
var _ wfcapp.AssigneeDirectory = (*Directory)(nil)
