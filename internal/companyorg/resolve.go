// Package companyorg provides company-scoped organization resolution helpers
// shared by reminder recipient routing and Tenant deadline step read models.
// Matching semantics must stay identical across callers.
package companyorg

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ResolveActiveDepartment maps a configured/snapshot department token to an
// active company department. Match keys: department_id OR department_code.
// Inactive/missing → matched=false (no error).
func ResolveActiveDepartment(ctx context.Context, db *sql.DB, companyID, snapshotDepartmentKey string) (departmentID string, matched bool, err error) {
	companyID = strings.TrimSpace(companyID)
	key := strings.TrimSpace(snapshotDepartmentKey)
	if db == nil || companyID == "" || key == "" {
		return "", false, nil
	}
	err = db.QueryRowContext(ctx, `
		SELECT department_id
		FROM departments
		WHERE company_id = ?
		  AND status = 'active'
		  AND (department_id = ? OR department_code = ?)
		LIMIT 1
	`, companyID, key, key).Scan(&departmentID)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("resolve active company department: %w", err)
	}
	departmentID = strings.TrimSpace(departmentID)
	if departmentID == "" {
		return "", false, nil
	}
	return departmentID, true, nil
}

// HasActiveEnterpriseAdmin reports whether the company has at least one
// eligible AdminEmailsByCompany recipient (role_code=admin_doanh_nghiep).
// Does not return emails/PII.
func HasActiveEnterpriseAdmin(ctx context.Context, db *sql.DB, companyID string) (bool, error) {
	companyID = strings.TrimSpace(companyID)
	if db == nil || companyID == "" {
		return false, nil
	}
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
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
			LIMIT 1
		)
	`, companyID).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("has active enterprise admin: %w", err)
	}
	return n == 1, nil
}
