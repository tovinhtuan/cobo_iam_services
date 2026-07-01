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
	"github.com/cobo/cobo_iam_services/internal/companyaccess/configversion"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func (r *AdminRepository) InsertPendingAdminChange(ctx context.Context, in caapp.InsertPendingAdminChangeInput) (*caapp.PendingAdminChange, error) {
	exists, err := r.HasPendingForAggregateStream(ctx, in.CompanyID, in.AggregateType, in.AggregateID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodePendingApprovalExists, "pending approval already exists for aggregate stream", nil)
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO pending_admin_changes (
			id, company_id, approval_subject_type, aggregate_type, aggregate_id, change_type,
			proposed_snapshot_json, base_live_version_no, status, requested_by, requested_at, reason
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, in.ID, in.CompanyID, in.ApprovalSubjectType, in.AggregateType, in.AggregateID, in.ChangeType,
		in.ProposedSnapshotJSON, nullInt(in.BaseLiveVersionNo), configversion.ApprovalStatusPending,
		in.RequestedBy, now, nullString(in.Reason))
	if err != nil {
		return nil, fmt.Errorf("insert pending_admin_changes: %w", err)
	}
	return r.GetPendingAdminChange(ctx, in.CompanyID, in.ID)
}

func (r *AdminRepository) GetPendingAdminChange(ctx context.Context, companyID, approvalID string) (*caapp.PendingAdminChange, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, company_id, approval_subject_type, aggregate_type, aggregate_id, change_type,
			proposed_snapshot_json, base_live_version_no, status, requested_by, requested_at,
			reviewed_by, reviewed_at, reason, reject_reason
		FROM pending_admin_changes
		WHERE id = ? AND company_id = ?
	`, approvalID, companyID)
	return scanPendingAdminChange(row)
}

func (r *AdminRepository) ListPendingAdminChanges(ctx context.Context, companyID, status, aggregateType string, limit int) ([]caapp.PendingAdminChange, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := `
		SELECT id, company_id, approval_subject_type, aggregate_type, aggregate_id, change_type,
			proposed_snapshot_json, base_live_version_no, status, requested_by, requested_at,
			reviewed_by, reviewed_at, reason, reject_reason
		FROM pending_admin_changes
		WHERE company_id = ?
	`
	args := []any{companyID}
	if s := strings.TrimSpace(status); s != "" {
		q += ` AND status = ?`
		args = append(args, s)
	}
	if at := strings.TrimSpace(aggregateType); at != "" {
		q += ` AND aggregate_type = ?`
		args = append(args, at)
	}
	q += ` ORDER BY requested_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]caapp.PendingAdminChange, 0)
	for rows.Next() {
		item, err := scanPendingAdminChangeRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *AdminRepository) HasPendingForAggregateStream(ctx context.Context, companyID, aggregateType, aggregateID string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pending_admin_changes
		WHERE company_id = ? AND aggregate_type = ? AND aggregate_id = ? AND status = ?
	`, companyID, aggregateType, aggregateID, configversion.ApprovalStatusPending).Scan(&n)
	return n > 0, err
}

func (r *AdminRepository) UpdatePendingAdminChangeDecision(ctx context.Context, companyID, approvalID, status, reviewedBy, rejectReason string) (*caapp.PendingAdminChange, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		UPDATE pending_admin_changes
		SET status = ?, reviewed_by = ?, reviewed_at = ?, reject_reason = ?
		WHERE id = ? AND company_id = ? AND status = ?
	`, status, reviewedBy, now, nullString(rejectReason), approvalID, companyID, configversion.ApprovalStatusPending)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		cur, getErr := r.GetPendingAdminChange(ctx, companyID, approvalID)
		if getErr != nil {
			return nil, getErr
		}
		if cur == nil {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "approval not found", nil)
		}
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeApprovalNotPending, "approval is not pending", nil)
	}
	return r.GetPendingAdminChange(ctx, companyID, approvalID)
}

func (r *AdminRepository) GetMaxNotificationRuleVersionNo(ctx context.Context, companyID, ruleID string) (int, error) {
	var n sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT MAX(version_no) FROM notification_rule_versions WHERE company_id = ? AND rule_id = ?
	`, companyID, ruleID).Scan(&n)
	if err != nil {
		return 0, err
	}
	if !n.Valid {
		return 0, nil
	}
	return int(n.Int64), nil
}

func (r *AdminRepository) GetMaxRBACMatrixVersionNo(ctx context.Context, companyID string) (int, error) {
	var n sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT MAX(version_no) FROM rbac_matrix_snapshots WHERE company_id = ?
	`, companyID).Scan(&n)
	if err != nil {
		return 0, err
	}
	if !n.Valid {
		return 0, nil
	}
	return int(n.Int64), nil
}

func (r *AdminRepository) ApplyPendingApprovalInTx(ctx context.Context, in caapp.ApplyPendingApprovalInput, row caapp.PendingAdminChange) (*caapp.ApplyPendingApprovalResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var status string
	err = tx.QueryRowContext(ctx, `
		SELECT status FROM pending_admin_changes WHERE id = ? AND company_id = ? FOR UPDATE
	`, in.ApprovalID, in.CompanyID).Scan(&status)
	if err == sql.ErrNoRows {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "approval not found", nil)
	}
	if err != nil {
		return nil, err
	}
	if status != configversion.ApprovalStatusPending {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeApprovalNotPending, "approval is not pending", nil)
	}

	raw := []byte(row.ProposedSnapshotJSON)
	switch row.AggregateType {
	case configversion.AggregateNotificationRule:
		if err := r.restoreNotificationRuleInTx(ctx, tx, in.CompanyID, raw); err != nil {
			return nil, err
		}
	case configversion.AggregateRBACMatrix:
		if err := r.restoreRBACMatrixInTx(ctx, tx, in.CompanyID, in.ActorUserID, raw); err != nil {
			return nil, err
		}
	default:
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "unsupported aggregate_type", nil)
	}

	var postVersionNo int
	switch row.AggregateType {
	case configversion.AggregateNotificationRule:
		err = tx.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(version_no), 0) + 1
			FROM notification_rule_versions
			WHERE company_id = ? AND rule_id = ?
			FOR UPDATE
		`, in.CompanyID, row.AggregateID).Scan(&postVersionNo)
		if err != nil {
			return nil, err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO notification_rule_versions (id, company_id, rule_id, version_no, snapshot_json, created_by, reason, source)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, in.VersionRowID, in.CompanyID, row.AggregateID, postVersionNo, raw, in.CreatedBy, "approval apply", configversion.SourceApprovalApply)
	case configversion.AggregateRBACMatrix:
		err = tx.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(version_no), 0) + 1
			FROM rbac_matrix_snapshots
			WHERE company_id = ?
			FOR UPDATE
		`, in.CompanyID).Scan(&postVersionNo)
		if err != nil {
			return nil, err
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO rbac_matrix_snapshots (id, company_id, version_no, snapshot_json, created_by, reason, source)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, in.VersionRowID, in.CompanyID, postVersionNo, raw, in.CreatedBy, "approval apply", configversion.SourceApprovalApply)
	}
	if err != nil {
		return nil, fmt.Errorf("insert post-apply version: %w", err)
	}

	now := time.Now().UTC()
	res, err := tx.ExecContext(ctx, `
		UPDATE pending_admin_changes
		SET status = ?, reviewed_by = ?, reviewed_at = ?
		WHERE id = ? AND company_id = ? AND status = ?
	`, configversion.ApprovalStatusApproved, in.ReviewedBy, now, in.ApprovalID, in.CompanyID, configversion.ApprovalStatusPending)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeApprovalNotPending, "approval is not pending", nil)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &caapp.ApplyPendingApprovalResult{PostApplyVersionNo: postVersionNo}, nil
}

func (r *AdminRepository) restoreNotificationRuleInTx(ctx context.Context, tx *sql.Tx, companyID string, raw []byte) error {
	var snap configversion.NotificationRuleSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid snapshot_json", nil)
	}
	payload := snap.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	status := strings.TrimSpace(snap.Status)
	if status == "" {
		status = "active"
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE notification_rules
		SET payload = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE notification_rule_id = ? AND company_id = ?
	`, payloadJSON, status, snap.NotificationRuleID, companyID)
	return err
}

func (r *AdminRepository) restoreRBACMatrixInTx(ctx context.Context, tx *sql.Tx, companyID, actorUserID string, raw []byte) error {
	var snap configversion.RBACMatrixSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid snapshot_json", nil)
	}
	roles, err := r.ListRoles(ctx, companyID)
	if err != nil {
		return err
	}
	roleSet := map[string]struct{}{}
	for _, role := range roles {
		roleSet[role.RoleID] = struct{}{}
	}
	targetRP := map[string]map[string]struct{}{}
	for _, e := range snap.RolePermissions {
		if _, ok := roleSet[e.RoleID]; !ok {
			continue
		}
		if targetRP[e.RoleID] == nil {
			targetRP[e.RoleID] = map[string]struct{}{}
		}
		targetRP[e.RoleID][e.PermissionID] = struct{}{}
	}
	for roleID := range roleSet {
		current, err := r.ListRolePermissions(ctx, companyID, roleID)
		if err != nil {
			return err
		}
		currentSet := map[string]struct{}{}
		for _, p := range current.Permissions {
			currentSet[p.PermissionID] = struct{}{}
		}
		want := targetRP[roleID]
		for pid := range currentSet {
			if _, ok := want[pid]; !ok {
				if err := r.removeRolePermissionTx(ctx, tx, roleID, pid); err != nil {
					return err
				}
			}
		}
		for pid := range want {
			if _, ok := currentSet[pid]; !ok {
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO role_permissions (role_id, permission_id, status)
					VALUES (?, ?, 'active')
					ON DUPLICATE KEY UPDATE status = 'active'
				`, roleID, pid); err != nil {
					return err
				}
			}
		}
	}
	currentDirect, err := r.ListActiveDirectPermissionsByCompany(ctx, companyID)
	if err != nil {
		return err
	}
	targetDirect := map[string]map[string]struct{}{}
	for _, d := range snap.DirectPermissions {
		if targetDirect[d.MembershipID] == nil {
			targetDirect[d.MembershipID] = map[string]struct{}{}
		}
		targetDirect[d.MembershipID][d.PermissionCode] = struct{}{}
	}
	for _, row := range currentDirect {
		if _, ok := targetDirect[row.MembershipID][row.PermissionCode]; ok {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE membership_direct_permissions
			SET revoked_at = CURRENT_TIMESTAMP, revoked_by = ?
			WHERE membership_id = ? AND permission_code = ? AND revoked_at IS NULL
		`, actorUserID, row.MembershipID, row.PermissionCode); err != nil {
			return err
		}
	}
	for _, d := range snap.DirectPermissions {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO membership_direct_permissions (membership_id, company_id, permission_code, granted_by)
			VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE revoked_at = NULL, revoked_by = NULL, granted_by = VALUES(granted_by), granted_at = CURRENT_TIMESTAMP
		`, d.MembershipID, companyID, d.PermissionCode, actorUserID); err != nil {
			return err
		}
	}
	return nil
}

func scanPendingAdminChange(row *sql.Row) (*caapp.PendingAdminChange, error) {
	var item caapp.PendingAdminChange
	var aggID, reviewedBy, reason, rejectReason sql.NullString
	var baseVer sql.NullInt64
	var reviewedAt sql.NullTime
	var raw []byte
	err := row.Scan(
		&item.ID, &item.CompanyID, &item.ApprovalSubjectType, &item.AggregateType, &aggID, &item.ChangeType,
		&raw, &baseVer, &item.Status, &item.RequestedBy, &item.RequestedAt,
		&reviewedBy, &reviewedAt, &reason, &rejectReason,
	)
	if err == sql.ErrNoRows {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "approval not found", nil)
	}
	if err != nil {
		return nil, err
	}
	item.AggregateID = aggID.String
	item.ProposedSnapshotJSON = raw
	if baseVer.Valid {
		v := int(baseVer.Int64)
		item.BaseLiveVersionNo = &v
	}
	item.ReviewedBy = reviewedBy.String
	if reviewedAt.Valid {
		t := reviewedAt.Time.UTC()
		item.ReviewedAt = &t
	}
	item.Reason = reason.String
	item.RejectReason = rejectReason.String
	item.RequestedAt = item.RequestedAt.UTC()
	return &item, nil
}

func scanPendingAdminChangeRows(rows *sql.Rows) (*caapp.PendingAdminChange, error) {
	var item caapp.PendingAdminChange
	var aggID, reviewedBy, reason, rejectReason sql.NullString
	var baseVer sql.NullInt64
	var reviewedAt sql.NullTime
	var raw []byte
	err := rows.Scan(
		&item.ID, &item.CompanyID, &item.ApprovalSubjectType, &item.AggregateType, &aggID, &item.ChangeType,
		&raw, &baseVer, &item.Status, &item.RequestedBy, &item.RequestedAt,
		&reviewedBy, &reviewedAt, &reason, &rejectReason,
	)
	if err != nil {
		return nil, err
	}
	item.AggregateID = aggID.String
	item.ProposedSnapshotJSON = raw
	if baseVer.Valid {
		v := int(baseVer.Int64)
		item.BaseLiveVersionNo = &v
	}
	item.ReviewedBy = reviewedBy.String
	if reviewedAt.Valid {
		t := reviewedAt.Time.UTC()
		item.ReviewedAt = &t
	}
	item.Reason = reason.String
	item.RejectReason = rejectReason.String
	item.RequestedAt = item.RequestedAt.UTC()
	return &item, nil
}

func nullInt(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}
