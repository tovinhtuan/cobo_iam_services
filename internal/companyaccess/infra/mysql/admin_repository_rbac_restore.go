package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
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

// applyRBACDirectRestorePlanTx executes the direct grant part of an RBAC matrix restore.
// caapp.ComputeRBACDirectRestorePlan limits it to the permissions a tenant admin may grant
// directly; grants already in place are not repeated, and a grant is only written for an
// active membership that belongs to the company.
func (r *AdminRepository) applyRBACDirectRestorePlanTx(ctx context.Context, tx *sql.Tx, companyID, actorUserID string, plan caapp.RBACDirectRestorePlan) error {
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
				WHERE m.membership_id = ? AND m.company_id = ? AND m.membership_status = 'active'
			) AS t
			ON DUPLICATE KEY UPDATE revoked_at = NULL, revoked_by = NULL, granted_by = VALUES(granted_by), granted_at = CURRENT_TIMESTAMP
		`, d.PermissionCode, actorUserID, d.MembershipID, companyID); err != nil {
			return fmt.Errorf("restore grant direct permission: %w", err)
		}
	}
	return nil
}

// txPlanReader lets caapp.BuildRBACRestorePlan read through the restore transaction.
type txPlanReader struct{ tx *sql.Tx }

func (t txPlanReader) ListRoles(ctx context.Context, companyID string) ([]caapp.RoleListItem, error) {
	return listRolesQ(ctx, t.tx, companyID)
}

func (t txPlanReader) ListRolePermissions(ctx context.Context, companyID, roleID string) (*caapp.RolePermissionsView, error) {
	return listRolePermissionsQ(ctx, t.tx, companyID, roleID)
}

func (t txPlanReader) ListPermissions(ctx context.Context) ([]caapp.PermissionListItem, error) {
	return listPermissionsQ(ctx, t.tx)
}

// drainRows runs a locking SELECT and discards the rows: the point is the row locks it takes.
func drainRows(ctx context.Context, tx *sql.Tx, query string, args ...any) error {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
	}
	return rows.Err()
}

// restoreRBACMatrixTx restores one RBAC matrix snapshot inside tx. It is the only place that
// writes a restore, shared by rollback and approval apply:
//
//  1. lock the company row, then its tenant_custom roles and active direct grants (fixed order, so two
//     restores of the same company queue up instead of interleaving, and concurrent grants of
//     a permission to those roles wait on the foreign key's lock of the role row);
//  2. read the state through tx and compute the plan from it;
//  3. when the plan touches a critical permission and the caller did not allow that, refuse
//     with caapp.ErrRBACRestoreNeedsApproval and change nothing;
//  4. apply the plan.
func (r *AdminRepository) restoreRBACMatrixTx(ctx context.Context, tx *sql.Tx, companyID, actorUserID string, raw []byte, opts caapp.RBACRestoreOptions) error {
	var snap configversion.RBACMatrixSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid snapshot_json", nil)
	}
	// One real row per company serialises restores of the same company, also when it has no
	// custom roles or active grants yet (where the locks below would only be gap locks, which do
	// not conflict and let two restores deadlock on their inserts).
	if err := drainRows(ctx, tx, `SELECT company_id FROM companies WHERE company_id = ? FOR UPDATE`, companyID); err != nil {
		return fmt.Errorf("lock company: %w", err)
	}
	// FORCE INDEX keeps the lock on this company's rows instead of a scan over other companies.
	if err := drainRows(ctx, tx, `
		SELECT role_id FROM roles FORCE INDEX (uk_roles_company_code)
		WHERE company_id = ? AND role_type = 'tenant_custom' AND is_protected = 0 AND status = 'active'
		ORDER BY role_id FOR UPDATE`, companyID); err != nil {
		return fmt.Errorf("lock custom roles: %w", err)
	}
	if err := drainRows(ctx, tx, `
		SELECT id FROM membership_direct_permissions FORCE INDEX (idx_mdp_lookup)
		WHERE company_id = ? AND revoked_at IS NULL
		ORDER BY id FOR UPDATE`, companyID); err != nil {
		return fmt.Errorf("lock direct grants: %w", err)
	}

	plan, err := caapp.BuildRBACRestorePlan(ctx, txPlanReader{tx: tx}, companyID, raw)
	if err != nil {
		return err
	}
	rows, err := listActiveDirectPermissionsByCompanyQ(ctx, tx, companyID)
	if err != nil {
		return err
	}
	current := make([]configversion.DirectPermissionEntry, 0, len(rows))
	for _, row := range rows {
		current = append(current, configversion.DirectPermissionEntry{MembershipID: row.MembershipID, PermissionCode: row.PermissionCode})
	}
	directPlan := caapp.RBACDirectPlanFor(current, snap)
	// An approval applies only what the approver reviewed: the plan computed here, under the
	// locks, must be the one fingerprinted when the request was queued.
	if snap.PlanDigest != "" && caapp.RBACRestorePlanDigest(plan, directPlan) != snap.PlanDigest {
		return caapp.ErrRBACRestorePlanChanged
	}
	if !opts.AllowCritical && (plan.TouchesCritical || directPlan.TouchesCritical) {
		return caapp.ErrRBACRestoreNeedsApproval
	}
	if err := r.applyRBACRestorePlanTx(ctx, tx, companyID, plan); err != nil {
		return err
	}
	return r.applyRBACDirectRestorePlanTx(ctx, tx, companyID, actorUserID, directPlan)
}
