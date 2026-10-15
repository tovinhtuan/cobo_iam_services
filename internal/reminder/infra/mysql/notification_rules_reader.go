package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
)

var _ reminderapp.NotificationRulesReader = (*NotificationRulesReader)(nil)

// NotificationRulesReader loads notification_rules rows for reminder consumer (read-only).
type NotificationRulesReader struct {
	db *sql.DB
}

// NewNotificationRulesReader returns a storage-backed NotificationRulesReader.
func NewNotificationRulesReader(db *sql.DB) *NotificationRulesReader {
	return &NotificationRulesReader{db: db}
}

// GetCompanyAlertPrefs loads company.alert_channel_prefs.v1 for companyID, whatever its status:
// the evaluator turns a non-active row into RULE_DISABLED (admin choice) and a missing row into the
// defaults (Q-23). The row is unique per (company_id, rule_code). Returns (nil, nil) when none exists.
func (r *NotificationRulesReader) GetCompanyAlertPrefs(ctx context.Context, companyID string) (*reminderapp.AlertChannelPrefsDocument, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("notification rules reader db is nil")
	}
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, fmt.Errorf("company_id is required")
	}
	var raw []byte
	var status, ruleCode string
	err := r.db.QueryRowContext(ctx, `
		SELECT rule_code, status, payload_json
		FROM notification_rules
		WHERE company_id = ? AND rule_code = ?
		LIMIT 1
	`, companyID, reminderapp.AlertChannelPrefsRuleCode).Scan(&ruleCode, &status, &raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get notification_rules: %w", err)
	}
	if st := strings.TrimSpace(status); st != "" && !strings.EqualFold(st, "active") {
		// Disabled by an admin: report the status only, so an old or invalid payload on a disabled
		// row still yields RULE_DISABLED instead of an evaluator error (BES-01).
		return &reminderapp.AlertChannelPrefsDocument{RuleCode: ruleCode, CompanyID: companyID, Status: st}, nil
	}
	payload := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, fmt.Errorf("decode notification_rules payload: %w", err)
		}
	}
	doc, err := reminderapp.ParseAlertChannelPrefsDocument(companyID, ruleCode, status, payload)
	if err != nil {
		return nil, err
	}
	return doc, nil
}
