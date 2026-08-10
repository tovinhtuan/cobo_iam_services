package app

import (
	"strings"
	"time"
)

// Mine semantics (locked):
// A disclosure record is "mine" iff current membership has:
//  1) an open workflow_tasks assignment match (v2 singular or v3 relation), OR
//  2) an active assignments row (membership) on that disclosure_record.
// NEVER expand via rbac.manage / company-wide / department-only visibility.

const (
	AccuracyExact       = "exact"
	AccuracyUnavailable = "unavailable"

	AlertUPCOMING       = "UPCOMING"
	AlertDUE_SOON       = "DUE_SOON"
	AlertOVERDUE        = "OVERDUE"
	AlertPENDINGConfirm = "PENDING_CONFIRM"
	AlertDONE           = "DONE"

	TaskStatusPending = "pending"
)

func IsTerminalRecordStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "done", "published":
		return true
	default:
		return false
	}
}

func IsDraftRecordStatus(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "draft")
}

func RemainingDays(dueDate string, now time.Time, loc *time.Location) int {
	dueDate = strings.TrimSpace(dueDate)
	if dueDate == "" {
		return 0
	}
	due, err := time.ParseInLocation("2006-01-02", dueDate, loc)
	if err != nil {
		return 0
	}
	now = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	due = time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, loc)
	return int(due.Sub(now).Hours() / 24)
}

// ClassifyMineAlert returns personal-ops alert status for a mine record.
// confirmed=true means deadline_alert_confirmations row exists → DONE.
func ClassifyMineAlert(recordStatus, dueDate string, confirmed bool, now time.Time, loc *time.Location) string {
	if confirmed {
		return AlertDONE
	}
	terminal := IsTerminalRecordStatus(recordStatus)
	if terminal {
		return AlertPENDINGConfirm
	}
	remaining := RemainingDays(dueDate, now, loc)
	if dueDate == "" {
		return AlertUPCOMING
	}
	if remaining < 0 {
		return AlertOVERDUE
	}
	if remaining == 0 {
		return AlertDUE_SOON
	}
	return AlertUPCOMING
}

func TaskStatusLabel(deadline string, now time.Time, loc *time.Location) (status, label string) {
	if strings.TrimSpace(deadline) == "" {
		return "open", "Đang mở"
	}
	rem := RemainingDays(deadline, now, loc)
	if rem < 0 {
		return "overdue", "Quá hạn"
	}
	if rem == 0 {
		return "due_soon", "Đến hạn"
	}
	return "open", "Đang mở"
}

func ptrInt(v int) *int { return &v }

func ptrString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func ExactMetric(v int) domainMetric {
	return domainMetric{Value: ptrInt(v), Accuracy: AccuracyExact}
}

func UnavailableMetric(reason string) domainMetric {
	return domainMetric{Value: nil, Accuracy: AccuracyUnavailable, Reason: ptrString(reason)}
}

// domainMetric is a local alias used by helpers before mapping to domain.Metric.
type domainMetric struct {
	Value    *int
	Accuracy string
	Reason   *string
}
