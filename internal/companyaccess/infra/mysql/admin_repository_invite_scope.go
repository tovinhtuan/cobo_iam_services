package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (r *AdminRepository) MembershipHasPermissionFromRole(ctx context.Context, membershipID, companyID, permissionCode string) (bool, error) {
	membershipID = strings.TrimSpace(membershipID)
	companyID = strings.TrimSpace(companyID)
	permissionCode = strings.TrimSpace(permissionCode)
	if membershipID == "" || permissionCode == "" {
		return false, nil
	}
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT 1 FROM membership_roles mr
		INNER JOIN role_permissions rp ON rp.role_id = mr.role_id AND rp.status = 'active'
		INNER JOIN permissions p ON p.permission_id = rp.permission_id AND p.status = 'active'
		  AND p.permission_code = ?
		INNER JOIN roles ro ON ro.role_id = mr.role_id AND ro.status = 'active'
		  AND (ro.company_id IS NULL OR ro.company_id = ?)
		WHERE mr.membership_id = ?
		LIMIT 1
	`, permissionCode, companyID, membershipID).Scan(&n)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("membership has permission from role: %w", err)
	}
	return true, nil
}

func (r *AdminRepository) HasActiveDirectPermission(ctx context.Context, membershipID, permissionCode string) (bool, error) {
	membershipID = strings.TrimSpace(membershipID)
	permissionCode = strings.TrimSpace(permissionCode)
	if membershipID == "" || permissionCode == "" {
		return false, nil
	}
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT 1 FROM membership_direct_permissions
		WHERE membership_id = ? AND permission_code = ? AND revoked_at IS NULL
		LIMIT 1
	`, membershipID, permissionCode).Scan(&n)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("has active direct permission: %w", err)
	}
	return true, nil
}

func (r *AdminRepository) ListDepartmentIDsByHeadMembership(ctx context.Context, companyID, headMembershipID string) ([]string, error) {
	companyID = strings.TrimSpace(companyID)
	headMembershipID = strings.TrimSpace(headMembershipID)
	if companyID == "" || headMembershipID == "" {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT department_id FROM departments
		WHERE company_id = ? AND head_membership_id = ? AND status = 'active'
		ORDER BY sort_order, name
	`, companyID, headMembershipID)
	if err != nil {
		return nil, fmt.Errorf("list departments by head: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *AdminRepository) MembershipInAnyDepartment(ctx context.Context, membershipID string, departmentIDs []string) (bool, error) {
	membershipID = strings.TrimSpace(membershipID)
	if membershipID == "" || len(departmentIDs) == 0 {
		return false, nil
	}
	placeholders := strings.Repeat("?,", len(departmentIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, 0, len(departmentIDs)+1)
	args = append(args, membershipID)
	for _, d := range departmentIDs {
		args = append(args, d)
	}
	q := fmt.Sprintf(`
		SELECT 1 FROM department_memberships
		WHERE membership_id = ? AND department_id IN (%s)
		LIMIT 1
	`, placeholders)
	var n int
	err := r.db.QueryRowContext(ctx, q, args...).Scan(&n)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("membership in department: %w", err)
	}
	return true, nil
}

func (r *AdminRepository) GetMembershipIDForUserCompany(ctx context.Context, userID, companyID string) (string, error) {
	var mid string
	err := r.db.QueryRowContext(ctx, `
		SELECT membership_id FROM memberships WHERE user_id = ? AND company_id = ? LIMIT 1
	`, userID, companyID).Scan(&mid)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return mid, nil
}
