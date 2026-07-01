package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func (r *AdminRepository) InsertDelegationGrant(ctx context.Context, in caapp.InsertDelegationGrantInput) (*caapp.DelegatedAdminGrant, error) {
	exists, err := r.HasActiveDelegationGrant(ctx, in.CompanyID, in.DelegateeMembershipID, in.ScopeType, in.ScopeID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "active delegation grant already exists for delegatee and scope", nil)
	}
	permJSON, err := json.Marshal(in.PermissionSet)
	if err != nil {
		return nil, fmt.Errorf("marshal permission_set: %w", err)
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO delegated_admin_grants (
			id, company_id, delegatee_membership_id, delegator_membership_id,
			scope_type, scope_id, permission_set_json, status,
			created_at, created_by, updated_at, updated_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, in.ID, in.CompanyID, in.DelegateeMembershipID, in.DelegatorMembershipID,
		in.ScopeType, in.ScopeID, permJSON, caapp.DelegationStatusActive,
		now, in.CreatedBy, now, in.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("insert delegated_admin_grants: %w", err)
	}
	return r.GetDelegationGrant(ctx, in.CompanyID, in.ID)
}

func (r *AdminRepository) GetDelegationGrant(ctx context.Context, companyID, delegationID string) (*caapp.DelegatedAdminGrant, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, company_id, delegatee_membership_id, delegator_membership_id,
			scope_type, scope_id, permission_set_json, status,
			created_at, created_by, updated_at, updated_by
		FROM delegated_admin_grants
		WHERE id = ? AND company_id = ?
	`, delegationID, companyID)
	return scanDelegationGrant(row)
}

func (r *AdminRepository) ListDelegationGrants(ctx context.Context, companyID, status, delegateeMembershipID, scopeID string, limit int) ([]caapp.DelegatedAdminGrant, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := `
		SELECT id, company_id, delegatee_membership_id, delegator_membership_id,
			scope_type, scope_id, permission_set_json, status,
			created_at, created_by, updated_at, updated_by
		FROM delegated_admin_grants
		WHERE company_id = ?
	`
	args := []any{companyID}
	if s := strings.TrimSpace(status); s != "" {
		q += ` AND status = ?`
		args = append(args, s)
	}
	if d := strings.TrimSpace(delegateeMembershipID); d != "" {
		q += ` AND delegatee_membership_id = ?`
		args = append(args, d)
	}
	if sid := strings.TrimSpace(scopeID); sid != "" {
		q += ` AND scope_id = ?`
		args = append(args, sid)
	}
	q += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]caapp.DelegatedAdminGrant, 0)
	for rows.Next() {
		item, err := scanDelegationGrantRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *AdminRepository) ListActiveDelegationsForDelegatee(ctx context.Context, companyID, delegateeMembershipID string) ([]caapp.DelegatedAdminGrant, error) {
	return r.ListDelegationGrants(ctx, companyID, caapp.DelegationStatusActive, delegateeMembershipID, "", 100)
}

func (r *AdminRepository) HasActiveDelegationGrant(ctx context.Context, companyID, delegateeMembershipID, scopeType, scopeID string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM delegated_admin_grants
		WHERE company_id = ? AND delegatee_membership_id = ? AND scope_type = ? AND scope_id = ? AND status = ?
	`, companyID, delegateeMembershipID, scopeType, scopeID, caapp.DelegationStatusActive).Scan(&n)
	return n > 0, err
}

func (r *AdminRepository) UpdateDelegationGrantPermissions(ctx context.Context, companyID, delegationID string, permissionSet []string, updatedBy string) (*caapp.DelegatedAdminGrant, error) {
	row, err := r.GetDelegationGrant(ctx, companyID, delegationID)
	if err != nil {
		return nil, err
	}
	if row.Status != caapp.DelegationStatusActive {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "delegation is not active", nil)
	}
	permJSON, err := json.Marshal(permissionSet)
	if err != nil {
		return nil, fmt.Errorf("marshal permission_set: %w", err)
	}
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		UPDATE delegated_admin_grants
		SET permission_set_json = ?, updated_at = ?, updated_by = ?
		WHERE id = ? AND company_id = ? AND status = ?
	`, permJSON, now, updatedBy, delegationID, companyID, caapp.DelegationStatusActive)
	if err != nil {
		return nil, fmt.Errorf("update delegated_admin_grants: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "delegation not found", nil)
	}
	return r.GetDelegationGrant(ctx, companyID, delegationID)
}

func (r *AdminRepository) RevokeDelegationGrant(ctx context.Context, companyID, delegationID, updatedBy string) (*caapp.DelegatedAdminGrant, error) {
	row, err := r.GetDelegationGrant(ctx, companyID, delegationID)
	if err != nil {
		return nil, err
	}
	if row.Status == caapp.DelegationStatusRevoked {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "delegation already revoked", nil)
	}
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		UPDATE delegated_admin_grants
		SET status = ?, updated_at = ?, updated_by = ?
		WHERE id = ? AND company_id = ? AND status = ?
	`, caapp.DelegationStatusRevoked, now, updatedBy, delegationID, companyID, caapp.DelegationStatusActive)
	if err != nil {
		return nil, fmt.Errorf("revoke delegated_admin_grants: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "delegation not found", nil)
	}
	return r.GetDelegationGrant(ctx, companyID, delegationID)
}

func scanDelegationGrant(row *sql.Row) (*caapp.DelegatedAdminGrant, error) {
	var g caapp.DelegatedAdminGrant
	var permJSON []byte
	err := row.Scan(
		&g.DelegationID, &g.CompanyID, &g.DelegateeMembershipID, &g.DelegatorMembershipID,
		&g.ScopeType, &g.ScopeID, &permJSON, &g.Status,
		&g.CreatedAt, &g.CreatedBy, &g.UpdatedAt, &g.UpdatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "delegation not found", nil)
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(permJSON, &g.PermissionSet); err != nil {
		return nil, fmt.Errorf("unmarshal permission_set: %w", err)
	}
	return &g, nil
}

func scanDelegationGrantRows(rows *sql.Rows) (*caapp.DelegatedAdminGrant, error) {
	var g caapp.DelegatedAdminGrant
	var permJSON []byte
	err := rows.Scan(
		&g.DelegationID, &g.CompanyID, &g.DelegateeMembershipID, &g.DelegatorMembershipID,
		&g.ScopeType, &g.ScopeID, &permJSON, &g.Status,
		&g.CreatedAt, &g.CreatedBy, &g.UpdatedAt, &g.UpdatedBy,
	)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(permJSON, &g.PermissionSet); err != nil {
		return nil, fmt.Errorf("unmarshal permission_set: %w", err)
	}
	return &g, nil
}
