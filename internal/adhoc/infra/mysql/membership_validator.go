package mysql

import (
	"context"
	"database/sql"
	"fmt"

	adhocapp "github.com/cobo/cobo_iam_services/internal/adhoc/app"
)

// MembershipValidator implements adhocapp.MembershipValidator using direct DB queries.
// It validates a membership's existence, active status, and permission eligibility
// without coupling to the authorization module's internal service.
type MembershipValidator struct {
	db *sql.DB
}

func NewMembershipValidator(db *sql.DB) *MembershipValidator {
	return &MembershipValidator{db: db}
}

func (v *MembershipValidator) IsActiveMembership(ctx context.Context, companyID, membershipID string) (bool, error) {
	var count int
	err := v.db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM memberships
		WHERE membership_id = ? AND company_id = ? AND membership_status = 'active'
	`, membershipID, companyID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check active membership: %w", err)
	}
	return count > 0, nil
}

func (v *MembershipValidator) HasPermission(ctx context.Context, companyID, membershipID, permissionCode string) (bool, error) {
	var count int
	err := v.db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM memberships m
		INNER JOIN membership_roles mr ON mr.membership_id = m.membership_id AND mr.status = 'active'
		INNER JOIN roles r ON r.role_id = mr.role_id AND r.status = 'active'
		INNER JOIN role_permissions rp ON rp.role_id = r.role_id AND rp.status = 'active'
		INNER JOIN permissions p ON p.permission_id = rp.permission_id AND p.status = 'active' AND p.permission_code = ?
		WHERE m.membership_id = ? AND m.company_id = ?
		  AND (r.company_id IS NULL OR r.company_id = m.company_id)
	`, permissionCode, membershipID, companyID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check permission: %w", err)
	}
	return count > 0, nil
}

func (v *MembershipValidator) ListMembersWithPermission(ctx context.Context, companyID, permissionCode, excludeMembershipID string) ([]adhocapp.EligibleController, error) {
	rows, err := v.db.QueryContext(ctx, `
		SELECT DISTINCT m.membership_id, u.full_name, COALESCE(u.email, '') AS email
		FROM memberships m
		INNER JOIN users u ON u.user_id = m.user_id
		INNER JOIN membership_roles mr ON mr.membership_id = m.membership_id AND mr.status = 'active'
		INNER JOIN roles r ON r.role_id = mr.role_id AND r.status = 'active'
		INNER JOIN role_permissions rp ON rp.role_id = r.role_id AND rp.status = 'active'
		INNER JOIN permissions p ON p.permission_id = rp.permission_id AND p.status = 'active' AND p.permission_code = ?
		WHERE m.company_id = ?
		  AND m.membership_status = 'active'
		  AND m.membership_id != ?
		  AND (r.company_id IS NULL OR r.company_id = m.company_id)
		ORDER BY u.full_name
	`, permissionCode, companyID, excludeMembershipID)
	if err != nil {
		return nil, fmt.Errorf("list members with permission: %w", err)
	}
	defer rows.Close()

	var out []adhocapp.EligibleController
	for rows.Next() {
		var c adhocapp.EligibleController
		if err := rows.Scan(&c.MembershipID, &c.FullName, &c.Email); err != nil {
			return nil, fmt.Errorf("scan eligible controller: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
