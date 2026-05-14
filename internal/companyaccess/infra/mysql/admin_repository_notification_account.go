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

func (r *AdminRepository) ListNotificationRules(ctx context.Context, companyID string) ([]caapp.NotificationRuleView, error) {
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company context required", nil)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT notification_rule_id, rule_code, status, payload_json, updated_at
		FROM notification_rules
		WHERE company_id = ?
		ORDER BY updated_at DESC, rule_code ASC
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list notification_rules: %w", err)
	}
	defer rows.Close()
	out := make([]caapp.NotificationRuleView, 0)
	for rows.Next() {
		var id, code, status string
		var raw []byte
		var updatedAt time.Time
		if err := rows.Scan(&id, &code, &status, &raw, &updatedAt); err != nil {
			return nil, err
		}
		var payload map[string]any
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &payload); err != nil {
				return nil, fmt.Errorf("decode payload_json: %w", err)
			}
		}
		if payload == nil {
			payload = map[string]any{}
		}
		for _, k := range []string{"notification_rule_id", "company_id", "rule_code", "status"} {
			delete(payload, k)
		}
		out = append(out, caapp.NotificationRuleView{
			NotificationRuleID: id,
			RuleCode:           code,
			Status:             status,
			Payload:            payload,
			UpdatedAt:          updatedAt.UTC(),
		})
	}
	return out, rows.Err()
}

func (r *AdminRepository) UpdateNotificationRuleMerged(ctx context.Context, companyID, ruleID string, payloadPatch map[string]any, status *string) error {
	companyID = strings.TrimSpace(companyID)
	ruleID = strings.TrimSpace(ruleID)
	if companyID == "" || ruleID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id and notification_rule_id required", nil)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var raw []byte
	var currentStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT payload_json, status FROM notification_rules
		WHERE notification_rule_id = ? AND company_id = ? FOR UPDATE
	`, ruleID, companyID).Scan(&raw, &currentStatus)
	if err == sql.ErrNoRows {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "notification rule not found", nil)
	}
	if err != nil {
		return fmt.Errorf("load notification_rule: %w", err)
	}
	base := map[string]any{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &base); err != nil {
			return fmt.Errorf("decode payload_json: %w", err)
		}
	}
	if len(payloadPatch) > 0 {
		mergeJSONObjects(base, payloadPatch)
	}
	merged, err := json.Marshal(base)
	if err != nil {
		return err
	}
	nextStatus := currentStatus
	if status != nil && strings.TrimSpace(*status) != "" {
		nextStatus = strings.TrimSpace(*status)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE notification_rules
		SET payload_json = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE notification_rule_id = ? AND company_id = ?
	`, merged, nextStatus, ruleID, companyID); err != nil {
		return fmt.Errorf("update notification_rule: %w", err)
	}
	return tx.Commit()
}

func (r *AdminRepository) DeleteNotificationRule(ctx context.Context, companyID, ruleID string) error {
	companyID = strings.TrimSpace(companyID)
	ruleID = strings.TrimSpace(ruleID)
	if companyID == "" || ruleID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id and notification_rule_id required", nil)
	}
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM notification_rules WHERE notification_rule_id = ? AND company_id = ?
	`, ruleID, companyID)
	if err != nil {
		return fmt.Errorf("delete notification_rule: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "notification rule not found", nil)
	}
	return nil
}

func mergeJSONObjects(dst map[string]any, src map[string]any) {
	for k, v := range src {
		dstMap, ok1 := dst[k].(map[string]any)
		srcMap, ok2 := v.(map[string]any)
		if ok1 && ok2 {
			mergeJSONObjects(dstMap, srcMap)
			continue
		}
		dst[k] = v
	}
}

func (r *AdminRepository) GetAdminAccountSettings(ctx context.Context, userID string) (*caapp.AdminAccountSettingsView, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "user_id required", nil)
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT user_id, login_id, full_name, COALESCE(email, ''), COALESCE(phone, ''), account_status
		FROM users WHERE user_id = ?
	`, userID)
	var v caapp.AdminAccountSettingsView
	if err := row.Scan(&v.UserID, &v.LoginID, &v.FullName, &v.Email, &v.Phone, &v.AccountStatus); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "user not found", nil)
		}
		return nil, err
	}
	return &v, nil
}

func (r *AdminRepository) PatchAdminAccountSettings(ctx context.Context, userID string, fullName, email, phone *string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "user_id required", nil)
	}
	if fullName == nil && email == nil && phone == nil {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "no fields to update", nil)
	}
	var sets []string
	var args []any
	if fullName != nil {
		sets = append(sets, "full_name = ?")
		args = append(args, strings.TrimSpace(*fullName))
	}
	if email != nil {
		sets = append(sets, "email = ?")
		em := strings.TrimSpace(strings.ToLower(*email))
		args = append(args, em)
	}
	if phone != nil {
		sets = append(sets, "phone = ?")
		args = append(args, strings.TrimSpace(*phone))
	}
	if len(sets) == 0 {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "no fields to update", nil)
	}
	q := "UPDATE users SET " + strings.Join(sets, ", ") + ", updated_at = CURRENT_TIMESTAMP WHERE user_id = ?"
	args = append(args, userID)
	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("patch user settings: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "user not found", nil)
	}
	return nil
}
