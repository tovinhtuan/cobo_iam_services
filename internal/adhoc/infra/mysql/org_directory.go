package mysql

import (
	"context"
	"database/sql"
)

// OrgDirectory validates department and membership refs for proposal workflow steps.
type OrgDirectory struct {
	db *sql.DB
}

func NewOrgDirectory(db *sql.DB) *OrgDirectory {
	return &OrgDirectory{db: db}
}

func (d *OrgDirectory) IsActiveDepartmentInCompany(ctx context.Context, companyID, departmentID string) (bool, error) {
	if d == nil || d.db == nil {
		return false, nil
	}
	var n int
	err := d.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM departments
		WHERE department_id = ? AND company_id = ? AND status = 'active'
	`, departmentID, companyID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (d *OrgDirectory) IsActiveMembershipInCompany(ctx context.Context, companyID, membershipID string) (bool, error) {
	if d == nil || d.db == nil {
		return false, nil
	}
	var n int
	err := d.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM memberships
		WHERE membership_id = ? AND company_id = ? AND membership_status = 'active'
	`, membershipID, companyID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (d *OrgDirectory) MemberBelongsToDepartment(ctx context.Context, membershipID, departmentID string) (bool, error) {
	if d == nil || d.db == nil {
		return false, nil
	}
	var n int
	err := d.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM department_memberships
		WHERE membership_id = ? AND department_id = ? AND status = 'active'
	`, membershipID, departmentID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
