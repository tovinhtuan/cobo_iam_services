package app

import (
	"strings"
	"time"
)

// Terminal completion statuses eligible for personal-ops on_time_rate outcome.
func IsTerminalCompletionStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "done", "published":
		return true
	default:
		return false
	}
}

// StampCompletedAtIfNeeded sets completed_at / completed_source once on terminal transition.
// Forward-only: never overwrites an existing completed_at. Never invents historical values.
func StampCompletedAtIfNeeded(rec *RecordDTO, source string, now time.Time) {
	if rec == nil {
		return
	}
	if !IsTerminalCompletionStatus(rec.Status) {
		return
	}
	if rec.CompletedAt != nil {
		return
	}
	t := now.UTC()
	rec.CompletedAt = &t
	if strings.TrimSpace(rec.CompletedSource) == "" {
		rec.CompletedSource = strings.TrimSpace(source)
	}
}
