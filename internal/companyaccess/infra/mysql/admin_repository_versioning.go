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

func (r *AdminRepository) BuildNotificationRuleSnapshotJSON(ctx context.Context, companyID, ruleID string) ([]byte, error) {
	rules, err := r.ListNotificationRules(ctx, companyID)
	if err != nil {
		return nil, err
	}
	var rule *caapp.NotificationRuleView
	for i := range rules {
		if rules[i].NotificationRuleID == ruleID {
			rule = &rules[i]
			break
		}
	}
	if rule == nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "notification rule not found", nil)
	}
	snap := configversion.NotificationRuleSnapshot{
		SchemaVersion:      configversion.NotificationSnapshotSchema,
		NotificationRuleID: rule.NotificationRuleID,
		RuleCode:           rule.RuleCode,
		Status:             rule.Status,
		Payload:            rule.Payload,
	}
	return json.Marshal(snap)
}

func (r *AdminRepository) BuildRBACMatrixSnapshotJSON(ctx context.Context, companyID string) ([]byte, error) {
	return buildRBACMatrixSnapshotQ(ctx, r.db, companyID)
}

func buildRBACMatrixSnapshotQ(ctx context.Context, q queryer, companyID string) ([]byte, error) {
	roles, err := listRolesQ(ctx, q, companyID)
	if err != nil {
		return nil, err
	}
	snap := configversion.RBACMatrixSnapshot{
		SchemaVersion:     configversion.RBACMatrixSnapshotSchema,
		RolePermissions:   []configversion.RolePermissionEntry{},
		DirectPermissions: []configversion.DirectPermissionEntry{},
	}
	for _, role := range roles {
		perms, err := listRolePermissionsQ(ctx, q, companyID, role.RoleID)
		if err != nil {
			return nil, err
		}
		for _, p := range perms.Permissions {
			snap.RolePermissions = append(snap.RolePermissions, configversion.RolePermissionEntry{
				RoleID:       role.RoleID,
				PermissionID: p.PermissionID,
			})
		}
	}
	directRows, err := listActiveDirectPermissionsByCompanyQ(ctx, q, companyID)
	if err != nil {
		return nil, err
	}
	for _, row := range directRows {
		snap.DirectPermissions = append(snap.DirectPermissions, configversion.DirectPermissionEntry{
			MembershipID:   row.MembershipID,
			PermissionCode: row.PermissionCode,
		})
	}
	return json.Marshal(snap)
}

func (r *AdminRepository) InsertNotificationRuleVersion(ctx context.Context, in caapp.InsertNotificationRuleVersionInput) (*caapp.ConfigVersionRow, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var next int
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version_no), 0) + 1
		FROM notification_rule_versions
		WHERE company_id = ? AND rule_id = ?
		FOR UPDATE
	`, in.CompanyID, in.RuleID).Scan(&next)
	if err != nil {
		return nil, fmt.Errorf("next notification version_no: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO notification_rule_versions (id, company_id, rule_id, version_no, snapshot_json, created_by, reason, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, in.ID, in.CompanyID, in.RuleID, next, in.SnapshotJSON, in.CreatedBy, nullString(in.Reason), in.Source)
	if err != nil {
		return nil, fmt.Errorf("insert notification_rule_versions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &caapp.ConfigVersionRow{
		ID:           in.ID,
		CompanyID:    in.CompanyID,
		AggregateType: configversion.AggregateNotificationRule,
		AggregateID:  in.RuleID,
		VersionNo:    next,
		CreatedBy:    in.CreatedBy,
		CreatedAt:    time.Now().UTC(),
		Reason:       in.Reason,
		Source:       in.Source,
	}, nil
}

func (r *AdminRepository) InsertRBACMatrixSnapshot(ctx context.Context, in caapp.InsertRBACMatrixSnapshotInput) (*caapp.ConfigVersionRow, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var next int
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version_no), 0) + 1
		FROM rbac_matrix_snapshots
		WHERE company_id = ?
		FOR UPDATE
	`, in.CompanyID).Scan(&next)
	if err != nil {
		return nil, fmt.Errorf("next rbac version_no: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO rbac_matrix_snapshots (id, company_id, version_no, snapshot_json, created_by, reason, source)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, in.ID, in.CompanyID, next, in.SnapshotJSON, in.CreatedBy, nullString(in.Reason), in.Source)
	if err != nil {
		return nil, fmt.Errorf("insert rbac_matrix_snapshots: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &caapp.ConfigVersionRow{
		ID:            in.ID,
		CompanyID:     in.CompanyID,
		AggregateType: configversion.AggregateRBACMatrix,
		VersionNo:     next,
		CreatedBy:     in.CreatedBy,
		CreatedAt:     time.Now().UTC(),
		Reason:        in.Reason,
		Source:        in.Source,
	}, nil
}

func (r *AdminRepository) ListNotificationRuleVersions(ctx context.Context, companyID, ruleID string, limit int) ([]caapp.ConfigVersionRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := `
		SELECT id, company_id, rule_id, version_no, created_by, created_at, reason, source
		FROM notification_rule_versions
		WHERE company_id = ?
	`
	args := []any{companyID}
	if strings.TrimSpace(ruleID) != "" {
		q += ` AND rule_id = ?`
		args = append(args, ruleID)
	}
	q += ` ORDER BY version_no DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanConfigVersionRows(rows, configversion.AggregateNotificationRule, true)
}

func (r *AdminRepository) GetNotificationRuleVersion(ctx context.Context, companyID, ruleID string, versionNo int) (*caapp.ConfigVersionDetail, error) {
	var id, createdBy, source string
	var createdAt time.Time
	var reason sql.NullString
	var raw []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, version_no, snapshot_json, created_by, created_at, reason, source
		FROM notification_rule_versions
		WHERE company_id = ? AND rule_id = ? AND version_no = ?
	`, companyID, ruleID, versionNo).Scan(&id, &versionNo, &raw, &createdBy, &createdAt, &reason, &source)
	if err == sql.ErrNoRows {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "version not found", nil)
	}
	if err != nil {
		return nil, err
	}
	return &caapp.ConfigVersionDetail{
		ConfigVersionRow: caapp.ConfigVersionRow{
			ID:            id,
			CompanyID:     companyID,
			AggregateType: configversion.AggregateNotificationRule,
			AggregateID:   ruleID,
			VersionNo:     versionNo,
			CreatedBy:     createdBy,
			CreatedAt:     createdAt.UTC(),
			Reason:        reason.String,
			Source:        source,
		},
		SnapshotJSON: raw,
	}, nil
}

func (r *AdminRepository) ListRBACMatrixVersions(ctx context.Context, companyID string, limit int) ([]caapp.ConfigVersionRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, company_id, version_no, created_by, created_at, reason, source
		FROM rbac_matrix_snapshots
		WHERE company_id = ?
		ORDER BY version_no DESC
		LIMIT ?
	`, companyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanConfigVersionRows(rows, configversion.AggregateRBACMatrix, false)
}

func (r *AdminRepository) GetRBACMatrixVersion(ctx context.Context, companyID string, versionNo int) (*caapp.ConfigVersionDetail, error) {
	var id, createdBy, source string
	var createdAt time.Time
	var reason sql.NullString
	var raw []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, version_no, snapshot_json, created_by, created_at, reason, source
		FROM rbac_matrix_snapshots
		WHERE company_id = ? AND version_no = ?
	`, companyID, versionNo).Scan(&id, &versionNo, &raw, &createdBy, &createdAt, &reason, &source)
	if err == sql.ErrNoRows {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "version not found", nil)
	}
	if err != nil {
		return nil, err
	}
	return &caapp.ConfigVersionDetail{
		ConfigVersionRow: caapp.ConfigVersionRow{
			ID:            id,
			CompanyID:     companyID,
			AggregateType: configversion.AggregateRBACMatrix,
			VersionNo:     versionNo,
			CreatedBy:     createdBy,
			CreatedAt:     createdAt.UTC(),
			Reason:        reason.String,
			Source:        source,
		},
		SnapshotJSON: raw,
	}, nil
}

func (r *AdminRepository) RestoreNotificationRuleFromSnapshot(ctx context.Context, companyID string, raw []byte) error {
	var snap configversion.NotificationRuleSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid snapshot_json", nil)
	}
	if strings.TrimSpace(snap.NotificationRuleID) == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "snapshot missing notification_rule_id", nil)
	}
	payload := snap.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	status := strings.TrimSpace(snap.Status)
	if status == "" {
		status = "active"
	}
	return r.UpdateNotificationRuleMerged(ctx, companyID, snap.NotificationRuleID, payload, &status)
}

func (r *AdminRepository) RestoreRBACMatrixFromSnapshot(ctx context.Context, companyID, actorUserID string, raw []byte, opts caapp.RBACRestoreOptions) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := r.restoreRBACMatrixTx(ctx, tx, companyID, actorUserID, raw, opts); err != nil {
		return err
	}
	return tx.Commit()
}

func scanConfigVersionRows(rows *sql.Rows, aggregateType string, withRuleID bool) ([]caapp.ConfigVersionRow, error) {
	out := make([]caapp.ConfigVersionRow, 0)
	for rows.Next() {
		var row caapp.ConfigVersionRow
		var reason sql.NullString
		var ruleID string
		var err error
		if withRuleID {
			err = rows.Scan(&row.ID, &row.CompanyID, &ruleID, &row.VersionNo, &row.CreatedBy, &row.CreatedAt, &reason, &row.Source)
			row.AggregateID = ruleID
		} else {
			err = rows.Scan(&row.ID, &row.CompanyID, &row.VersionNo, &row.CreatedBy, &row.CreatedAt, &reason, &row.Source)
		}
		if err != nil {
			return nil, err
		}
		row.AggregateType = aggregateType
		row.Reason = reason.String
		row.CreatedAt = row.CreatedAt.UTC()
		out = append(out, row)
	}
	return out, rows.Err()
}

func nullString(s string) sql.NullString {
	s = strings.TrimSpace(s)
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
