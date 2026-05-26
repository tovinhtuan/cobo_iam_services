package app

import (
	"strings"
	"time"
)

func normalizeStatusFilter(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "", "ALL":
		return ""
	case "UPCOMING":
		return "UPCOMING"
	case "DUE_SOON", "DUE SOON":
		return "DUE_SOON"
	case "OVERDUE":
		return "OVERDUE"
	case "PENDING_CONFIRM", "PENDING CONFIRM":
		return "PENDING_CONFIRM"
	case "DONE":
		return "DONE"
	default:
		return strings.ToUpper(strings.TrimSpace(raw))
	}
}

func isTerminalRecordStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "done", "published":
		return true
	default:
		return false
	}
}

func isDraftRecordStatus(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "draft")
}

func alertStatusFromRemainingDays(remainingDays int, terminal bool) string {
	if terminal {
		return "DONE"
	}
	if remainingDays < 0 {
		return "OVERDUE"
	}
	if remainingDays == 0 {
		return "DUE_SOON"
	}
	return "UPCOMING"
}

func remainingDaysFromDue(dueDate string, now time.Time, loc *time.Location) int {
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

func matchesDateRange(dueDate, startDate, endDate string) bool {
	dueDate = strings.TrimSpace(dueDate)
	if dueDate == "" {
		return startDate == "" && endDate == ""
	}
	if startDate != "" && dueDate < startDate {
		return false
	}
	if endDate != "" && dueDate > endDate {
		return false
	}
	return true
}
