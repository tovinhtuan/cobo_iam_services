package mysql

import (
	"context"
	"database/sql"
	"fmt"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
)

// applyRBACRestorePlanTx executes the role_permissions part of an RBAC matrix restore.
//
// The plan (caapp.ComputeRBACMatrixRestorePlan) already limits the change to active,
// non-protected tenant_custom roles of the company and to enterprise-scope permissions. The
// statements below repeat that boundary in SQL so that a mistake in the plan can never reach
// a global role, a protected default role or a role of another company.
func (r *AdminRepository) applyRBACRestorePlanTx(ctx context.Context, tx *sql.Tx, companyID string, plan caapp.RBACRestorePlan) error {
	for _, op := range plan.Ops {
		if op.Add {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO role_permissions (role_id, permission_id, status)
				SELECT t.role_id, t.permission_id, 'active'
				FROM (
					SELECT r.role_id AS role_id, ? AS permission_id
					FROM roles r
					WHERE r.role_id = ? AND r.company_id = ?
					  AND r.role_type = 'tenant_custom' AND r.is_protected = 0 AND r.status = 'active'
				) AS t
				ON DUPLICATE KEY UPDATE status = 'active'
			`, op.PermissionID, op.RoleID, companyID); err != nil {
				return fmt.Errorf("restore add role permission: %w", err)
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			DELETE rp FROM role_permissions rp
			INNER JOIN roles r ON r.role_id = rp.role_id
			WHERE rp.role_id = ? AND rp.permission_id = ?
			  AND r.company_id = ? AND r.role_type = 'tenant_custom' AND r.is_protected = 0
		`, op.RoleID, op.PermissionID, companyID); err != nil {
			return fmt.Errorf("restore remove role permission: %w", err)
		}
	}
	return nil
}

// restoreDirectGrantsTx converges the direct permission grants of one company to target.
// caapp.ComputeRBACDirectRestorePlan limits it to the permissions a tenant admin may grant
// directly; grants already in place are not repeated, and a grant is only written for a
// membership that belongs to the company.
func (r *AdminRepository) restoreDirectGrantsTx(ctx context.Context, tx *sql.Tx, companyID, actorUserID string, target []configversion.DirectPermissionEntry) error {
	rows, err := r.ListActiveDirectPermissionsByCompany(ctx, companyID)
	if err != nil {
		return err
	}
	current := make([]configversion.DirectPermissionEntry, 0, len(rows))
	for _, row := range rows {
		current = append(current, configversion.DirectPermissionEntry{MembershipID: row.MembershipID, PermissionCode: row.PermissionCode})
	}
	plan := caapp.ComputeRBACDirectRestorePlan(current, target)
	for _, d := range plan.Revoke {
		if _, err := tx.ExecContext(ctx, `
			UPDATE membership_direct_permissions
			SET revoked_at = CURRENT_TIMESTAMP, revoked_by = ?
			WHERE membership_id = ? AND permission_code = ? AND company_id = ? AND revoked_at IS NULL
		`, actorUserID, d.MembershipID, d.PermissionCode, companyID); err != nil {
			return fmt.Errorf("restore revoke direct permission: %w", err)
		}
	}
	for _, d := range plan.Grant {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO membership_direct_permissions (membership_id, company_id, permission_code, granted_by)
			SELECT t.membership_id, t.company_id, t.permission_code, t.granted_by
			FROM (
				SELECT m.membership_id AS membership_id, m.company_id AS company_id, ? AS permission_code, ? AS granted_by
				FROM memberships m
				WHERE m.membership_id = ? AND m.company_id = ?
			) AS t
			ON DUPLICATE KEY UPDATE revoked_at = NULL, revoked_by = NULL, granted_by = VALUES(granted_by), granted_at = CURRENT_TIMESTAMP
		`, d.PermissionCode, actorUserID, d.MembershipID, companyID); err != nil {
			return fmt.Errorf("restore grant direct permission: %w", err)
		}
	}
	return nil
}
