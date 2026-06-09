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
		SELECT COUNT(1) FROM (
			SELECT 1
			FROM memberships m
			INNER JOIN membership_roles mr ON mr.membership_id = m.membership_id AND mr.status = 'active'
			INNER JOIN roles r ON r.role_id = mr.role_id AND r.status = 'active'
			INNER JOIN role_permissions rp ON rp.role_id = r.role_id AND rp.status = 'active'
			INNER JOIN permissions p ON p.permission_id = rp.permission_id AND p.status = 'active' AND p.permission_code = ?
			WHERE m.membership_id = ? AND m.company_id = ?
			  AND (r.company_id IS NULL OR r.company_id = m.company_id)

			UNION ALL

			SELECT 1
			FROM membership_direct_permissions mdp
			WHERE mdp.membership_id = ? AND mdp.company_id = ?
			  AND mdp.permission_code = ?
			  AND mdp.revoked_at IS NULL
		) AS grants
	`, permissionCode, membershipID, companyID, membershipID, companyID, permissionCode).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check permission: %w", err)
	}
	return count > 0, nil
}

func (v *MembershipValidator) HasActiveRoleCode(ctx context.Context, companyID, membershipID, roleCode string) (bool, error) {
	var count int
	err := v.db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM memberships m
		INNER JOIN membership_roles mr ON mr.membership_id = m.membership_id AND mr.status = 'active'
		INNER JOIN roles r ON r.role_id = mr.role_id AND r.status = 'active' AND r.role_code = ?
		WHERE m.membership_id = ? AND m.company_id = ?
		  AND m.membership_status = 'active'
		  AND (r.company_id IS NULL OR r.company_id = m.company_id)
	`, roleCode, membershipID, companyID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check active role code: %w", err)
	}
	return count > 0, nil
}

func (v *MembershipValidator) ListMembersWithPermission(ctx context.Context, companyID, permissionCode, excludeMembershipID string) ([]adhocapp.EligibleController, error) {
	query := `
		SELECT DISTINCT m.membership_id, u.full_name, COALESCE(NULLIF(TRIM(u.email), ''), u.login_id) AS email
		FROM memberships m
		INNER JOIN users u ON u.user_id = m.user_id
		WHERE m.company_id = ?
		  AND m.membership_status = 'active'
		  AND (
		    EXISTS (
		      SELECT 1
		      FROM membership_roles mr
		      INNER JOIN roles r ON r.role_id = mr.role_id AND r.status = 'active'
		      INNER JOIN role_permissions rp ON rp.role_id = r.role_id AND rp.status = 'active'
		      INNER JOIN permissions p ON p.permission_id = rp.permission_id AND p.status = 'active' AND p.permission_code = ?
		      WHERE mr.membership_id = m.membership_id
		        AND (r.company_id IS NULL OR r.company_id = m.company_id)
		    )
		    OR EXISTS (
		      SELECT 1
		      FROM membership_direct_permissions mdp
		      WHERE mdp.membership_id = m.membership_id
		        AND mdp.company_id = m.company_id
		        AND mdp.permission_code = ?
		        AND mdp.revoked_at IS NULL
		    )
		  )`
	args := []any{companyID, permissionCode, permissionCode}
	if excludeMembershipID != "" {
		query += ` AND m.membership_id <> ?`
		args = append(args, excludeMembershipID)
	}
	query += ` ORDER BY u.full_name`

	rows, err := v.db.QueryContext(ctx, query, args...)
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

// ResolveMembership returns identity fields for a single active membership.
// Returns nil, nil when the membership is not found or inactive.
func (v *MembershipValidator) ResolveMembership(ctx context.Context, companyID, membershipID string) (*adhocapp.MemberInfo, error) {
	var m adhocapp.MemberInfo
	err := v.db.QueryRowContext(ctx, `
		SELECT u.user_id, m.membership_id,
		       COALESCE(NULLIF(TRIM(u.email), ''), u.login_id) AS email,
		       COALESCE(u.full_name, '') AS full_name
		FROM memberships m
		INNER JOIN users u ON u.user_id = m.user_id
		WHERE m.membership_id = ? AND m.company_id = ?
		  AND m.membership_status = 'active'
	`, membershipID, companyID).Scan(&m.UserID, &m.MembershipID, &m.Email, &m.FullName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("resolve membership: %w", err)
	}
	return &m, nil
}

// ResolveCompanyName returns the display name of a company.
// Returns "", nil when the company is not found — callers fall back to a safe
// placeholder rather than surfacing the UUID in user-facing content.
func (v *MembershipValidator) ResolveCompanyName(ctx context.Context, companyID string) (string, error) {
	var name string
	err := v.db.QueryRowContext(ctx,
		`SELECT company_name FROM companies WHERE company_id = ? LIMIT 1`,
		companyID,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("resolve company name: %w", err)
	}
	return name, nil
}

// ListMembersWithPermissionFull returns all active members holding permissionCode,
// including UserID for in-app notification dispatch.
func (v *MembershipValidator) ListMembersWithPermissionFull(ctx context.Context, companyID, permissionCode string) ([]adhocapp.MemberInfo, error) {
	rows, err := v.db.QueryContext(ctx, `
		SELECT DISTINCT u.user_id, m.membership_id,
		       COALESCE(NULLIF(TRIM(u.email), ''), u.login_id) AS email,
		       COALESCE(u.full_name, '') AS full_name
		FROM memberships m
		INNER JOIN users u ON u.user_id = m.user_id
		WHERE m.company_id = ?
		  AND m.membership_status = 'active'
		  AND (
		    EXISTS (
		      SELECT 1
		      FROM membership_roles mr
		      INNER JOIN roles r ON r.role_id = mr.role_id AND r.status = 'active'
		      INNER JOIN role_permissions rp ON rp.role_id = r.role_id AND rp.status = 'active'
		      INNER JOIN permissions p ON p.permission_id = rp.permission_id AND p.status = 'active' AND p.permission_code = ?
		      WHERE mr.membership_id = m.membership_id
		        AND (r.company_id IS NULL OR r.company_id = m.company_id)
		    )
		    OR EXISTS (
		      SELECT 1
		      FROM membership_direct_permissions mdp
		      WHERE mdp.membership_id = m.membership_id
		        AND mdp.company_id = m.company_id
		        AND mdp.permission_code = ?
		        AND mdp.revoked_at IS NULL
		    )
		  )
		ORDER BY full_name
	`, companyID, permissionCode, permissionCode)
	if err != nil {
		return nil, fmt.Errorf("list members with permission full: %w", err)
	}
	defer rows.Close()

	var out []adhocapp.MemberInfo
	for rows.Next() {
		var m adhocapp.MemberInfo
		if err := rows.Scan(&m.UserID, &m.MembershipID, &m.Email, &m.FullName); err != nil {
			return nil, fmt.Errorf("scan member info: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
