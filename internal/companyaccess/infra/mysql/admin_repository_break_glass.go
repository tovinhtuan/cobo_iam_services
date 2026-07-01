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

func (r *AdminRepository) InsertEmergencyAccessGrant(ctx context.Context, in caapp.InsertEmergencyAccessGrantInput) (*caapp.EmergencyAccessGrant, error) {
	active, err := r.HasActiveEmergencyGrantForTarget(ctx, in.CompanyID, in.TargetMembershipID)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "active emergency access grant already exists for target", nil)
	}
	capJSON, err := json.Marshal(in.CapabilitySet)
	if err != nil {
		return nil, fmt.Errorf("marshal capability_set: %w", err)
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO emergency_access_grants (
			id, company_id, target_membership_id, requester_membership_id,
			reason, scope, capability_set_json, requested_duration_seconds, status, requested_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, in.SessionID, in.CompanyID, in.TargetMembershipID, in.RequesterMembershipID,
		in.Reason, in.Scope, capJSON, in.RequestedDurationSec, caapp.EmergencyStatusPendingFirst, now)
	if err != nil {
		return nil, fmt.Errorf("insert emergency_access_grants: %w", err)
	}
	return r.GetEmergencyAccessGrant(ctx, in.CompanyID, in.SessionID)
}

func (r *AdminRepository) GetEmergencyAccessGrant(ctx context.Context, companyID, sessionID string) (*caapp.EmergencyAccessGrant, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, company_id, target_membership_id, requester_membership_id,
			approver_membership_id_1, approver_membership_id_2,
			reason, scope, capability_set_json, requested_duration_seconds, status,
			requested_at, activated_at, expires_at, revoked_at
		FROM emergency_access_grants
		WHERE id = ? AND company_id = ?
	`, sessionID, companyID)
	return scanEmergencyGrant(row)
}

func (r *AdminRepository) ListEmergencyAccessGrants(ctx context.Context, companyID, status, targetMembershipID string, limit int) ([]caapp.EmergencyAccessGrant, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := `
		SELECT id, company_id, target_membership_id, requester_membership_id,
			approver_membership_id_1, approver_membership_id_2,
			reason, scope, capability_set_json, requested_duration_seconds, status,
			requested_at, activated_at, expires_at, revoked_at
		FROM emergency_access_grants
		WHERE company_id = ?
	`
	args := []any{companyID}
	if s := strings.TrimSpace(status); s != "" {
		q += ` AND status = ?`
		args = append(args, s)
	}
	if t := strings.TrimSpace(targetMembershipID); t != "" {
		q += ` AND target_membership_id = ?`
		args = append(args, t)
	}
	q += ` ORDER BY requested_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]caapp.EmergencyAccessGrant, 0)
	for rows.Next() {
		item, err := scanEmergencyGrantRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *AdminRepository) GetActiveEmergencyGrantForTarget(ctx context.Context, companyID, targetMembershipID string) (*caapp.EmergencyAccessGrant, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, company_id, target_membership_id, requester_membership_id,
			approver_membership_id_1, approver_membership_id_2,
			reason, scope, capability_set_json, requested_duration_seconds, status,
			requested_at, activated_at, expires_at, revoked_at
		FROM emergency_access_grants
		WHERE company_id = ? AND target_membership_id = ? AND status = ?
		ORDER BY activated_at DESC LIMIT 1
	`, companyID, targetMembershipID, caapp.EmergencyStatusActive)
	grant, err := scanEmergencyGrant(row)
	if err != nil {
		if he, ok := perr.AsHTTPError(err); ok && he.HTTPStatus == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	if grant != nil && grant.ExpiresAt != nil && grant.ExpiresAt.Before(time.Now().UTC()) {
		expired, expErr := r.ExpireEmergencyGrant(ctx, companyID, grant.SessionID)
		if expErr != nil {
			return nil, expErr
		}
		_ = expired
		return nil, nil
	}
	return grant, nil
}

func (r *AdminRepository) HasActiveEmergencyGrantForTarget(ctx context.Context, companyID, targetMembershipID string) (bool, error) {
	g, err := r.GetActiveEmergencyGrantForTarget(ctx, companyID, targetMembershipID)
	if err != nil {
		return false, err
	}
	return g != nil, nil
}

func (r *AdminRepository) RecordEmergencyFirstApproval(ctx context.Context, companyID, sessionID, approverMembershipID string) (*caapp.EmergencyAccessGrant, error) {
	row, err := r.GetEmergencyAccessGrant(ctx, companyID, sessionID)
	if err != nil {
		return nil, err
	}
	if row.Status != caapp.EmergencyStatusPendingFirst {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "grant is not pending first approval", nil)
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE emergency_access_grants
		SET approver_membership_id_1 = ?, status = ?, updated_at = ?
		WHERE id = ? AND company_id = ?
	`, approverMembershipID, caapp.EmergencyStatusPendingSecond, time.Now().UTC(), sessionID, companyID)
	if err != nil {
		return nil, err
	}
	return r.GetEmergencyAccessGrant(ctx, companyID, sessionID)
}

func (r *AdminRepository) ActivateEmergencyGrant(ctx context.Context, companyID, sessionID, approverMembershipID string, expiresAt time.Time) (*caapp.EmergencyAccessGrant, error) {
	row, err := r.GetEmergencyAccessGrant(ctx, companyID, sessionID)
	if err != nil {
		return nil, err
	}
	if row.Status != caapp.EmergencyStatusPendingSecond {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "grant is not pending second approval", nil)
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
		UPDATE emergency_access_grants
		SET approver_membership_id_2 = ?, status = ?, activated_at = ?, expires_at = ?, updated_at = ?
		WHERE id = ? AND company_id = ?
	`, approverMembershipID, caapp.EmergencyStatusActive, now, expiresAt.UTC(), now, sessionID, companyID)
	if err != nil {
		return nil, err
	}
	return r.GetEmergencyAccessGrant(ctx, companyID, sessionID)
}

func (r *AdminRepository) DenyEmergencyGrant(ctx context.Context, companyID, sessionID string) (*caapp.EmergencyAccessGrant, error) {
	row, err := r.GetEmergencyAccessGrant(ctx, companyID, sessionID)
	if err != nil {
		return nil, err
	}
	if row.Status != caapp.EmergencyStatusPendingFirst && row.Status != caapp.EmergencyStatusPendingSecond {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "grant is not pending approval", nil)
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE emergency_access_grants SET status = ?, updated_at = ? WHERE id = ? AND company_id = ?
	`, caapp.EmergencyStatusDenied, time.Now().UTC(), sessionID, companyID)
	if err != nil {
		return nil, err
	}
	return r.GetEmergencyAccessGrant(ctx, companyID, sessionID)
}

func (r *AdminRepository) CancelEmergencyGrant(ctx context.Context, companyID, sessionID string) (*caapp.EmergencyAccessGrant, error) {
	row, err := r.GetEmergencyAccessGrant(ctx, companyID, sessionID)
	if err != nil {
		return nil, err
	}
	if row.Status != caapp.EmergencyStatusPendingFirst && row.Status != caapp.EmergencyStatusPendingSecond {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "grant is not pending approval", nil)
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE emergency_access_grants SET status = ?, updated_at = ? WHERE id = ? AND company_id = ?
	`, caapp.EmergencyStatusCancelled, time.Now().UTC(), sessionID, companyID)
	if err != nil {
		return nil, err
	}
	return r.GetEmergencyAccessGrant(ctx, companyID, sessionID)
}

func (r *AdminRepository) RevokeEmergencyGrant(ctx context.Context, companyID, sessionID string) (*caapp.EmergencyAccessGrant, error) {
	row, err := r.GetEmergencyAccessGrant(ctx, companyID, sessionID)
	if err != nil {
		return nil, err
	}
	if row.Status != caapp.EmergencyStatusActive {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "grant is not active", nil)
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
		UPDATE emergency_access_grants SET status = ?, revoked_at = ?, updated_at = ? WHERE id = ? AND company_id = ?
	`, caapp.EmergencyStatusRevoked, now, now, sessionID, companyID)
	if err != nil {
		return nil, err
	}
	return r.GetEmergencyAccessGrant(ctx, companyID, sessionID)
}

func (r *AdminRepository) ExpireEmergencyGrant(ctx context.Context, companyID, sessionID string) (*caapp.EmergencyAccessGrant, error) {
	row, err := r.GetEmergencyAccessGrant(ctx, companyID, sessionID)
	if err != nil {
		return nil, err
	}
	if row.Status != caapp.EmergencyStatusActive {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "grant is not active", nil)
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
		UPDATE emergency_access_grants SET status = ?, updated_at = ? WHERE id = ? AND company_id = ?
	`, caapp.EmergencyStatusExpired, now, sessionID, companyID)
	if err != nil {
		return nil, err
	}
	return r.GetEmergencyAccessGrant(ctx, companyID, sessionID)
}

func (r *AdminRepository) ExpireDueEmergencyGrants(ctx context.Context, companyID string) (int, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		UPDATE emergency_access_grants
		SET status = ?, updated_at = ?
		WHERE company_id = ? AND status = ? AND expires_at IS NOT NULL AND expires_at <= ?
	`, caapp.EmergencyStatusExpired, now, companyID, caapp.EmergencyStatusActive, now)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func scanEmergencyGrant(row *sql.Row) (*caapp.EmergencyAccessGrant, error) {
	var (
		id, companyID, targetID, requesterID string
		approver1, approver2                 sql.NullString
		reason, scope, status                string
		capJSON                              []byte
		requestedDuration                      int
		requestedAt                          time.Time
		activatedAt, expiresAt, revokedAt    sql.NullTime
	)
	err := row.Scan(&id, &companyID, &targetID, &requesterID,
		&approver1, &approver2, &reason, &scope, &capJSON, &requestedDuration, &status,
		&requestedAt, &activatedAt, &expiresAt, &revokedAt)
	if err == sql.ErrNoRows {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "emergency access grant not found", nil)
	}
	if err != nil {
		return nil, err
	}
	return buildEmergencyGrant(id, companyID, targetID, requesterID, approver1, approver2,
		reason, scope, capJSON, requestedDuration, status, requestedAt, activatedAt, expiresAt, revokedAt)
}

func scanEmergencyGrantRows(rows *sql.Rows) (*caapp.EmergencyAccessGrant, error) {
	var (
		id, companyID, targetID, requesterID string
		approver1, approver2                 sql.NullString
		reason, scope, status                string
		capJSON                              []byte
		requestedDuration                      int
		requestedAt                          time.Time
		activatedAt, expiresAt, revokedAt    sql.NullTime
	)
	err := rows.Scan(&id, &companyID, &targetID, &requesterID,
		&approver1, &approver2, &reason, &scope, &capJSON, &requestedDuration, &status,
		&requestedAt, &activatedAt, &expiresAt, &revokedAt)
	if err != nil {
		return nil, err
	}
	return buildEmergencyGrant(id, companyID, targetID, requesterID, approver1, approver2,
		reason, scope, capJSON, requestedDuration, status, requestedAt, activatedAt, expiresAt, revokedAt)
}

func buildEmergencyGrant(
	id, companyID, targetID, requesterID string,
	approver1, approver2 sql.NullString,
	reason, scope string, capJSON []byte, requestedDuration int, status string,
	requestedAt time.Time,
	activatedAt, expiresAt, revokedAt sql.NullTime,
) (*caapp.EmergencyAccessGrant, error) {
	var caps []string
	if len(capJSON) > 0 {
		if err := json.Unmarshal(capJSON, &caps); err != nil {
			return nil, fmt.Errorf("unmarshal capability_set: %w", err)
		}
	}
	out := &caapp.EmergencyAccessGrant{
		SessionID:             id,
		CompanyID:             companyID,
		TargetMembershipID:    targetID,
		RequesterMembershipID: requesterID,
		Reason:                reason,
		Scope:                 scope,
		CapabilitySet:            caps,
		RequestedDurationSeconds: requestedDuration,
		Status:                   status,
		RequestedAt:           requestedAt.UTC(),
	}
	if approver1.Valid {
		out.ApproverMembershipID1 = approver1.String
	}
	if approver2.Valid {
		out.ApproverMembershipID2 = approver2.String
	}
	if activatedAt.Valid {
		t := activatedAt.Time.UTC()
		out.ActivatedAt = &t
	}
	if expiresAt.Valid {
		t := expiresAt.Time.UTC()
		out.ExpiresAt = &t
	}
	if revokedAt.Valid {
		t := revokedAt.Time.UTC()
		out.RevokedAt = &t
	}
	return out, nil
}
