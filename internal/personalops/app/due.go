package app

import "strings"

// Deadline source enum (real schema). workflow_task_due_date / milestone / deadline_alert
// columns do NOT exist on workflow_tasks — documented as absent, never invented.
const (
	DeadlineSourceAdHocApproved   = "ad_hoc_approved_deadline"
	DeadlineSourcePlannedFallback = "planned_date_fallback"
	DeadlineSourceUnavailable     = "unavailable"
	// Documented absent sources (never emitted as exact):
	DeadlineSourceWorkflowTask     = "workflow_task_due_date"      // column absent
	DeadlineSourceWorkflowMilestone = "workflow_milestone_due_date" // not record due
	DeadlineSourceDeadlineAlert    = "deadline_alert_due_date"     // derived model, not a column
)

type ResolvedDeadline struct {
	Date     string
	Source   string
	Accuracy string // exact | unavailable
}

// ResolveDeadline prefers approved ad-hoc deadline over planned_date (product priority).
// planned_date is always labeled planned_date_fallback when used.
func ResolveDeadline(plannedDate, adHocDue string) ResolvedDeadline {
	adHocDue = strings.TrimSpace(adHocDue)
	plannedDate = strings.TrimSpace(plannedDate)
	if adHocDue != "" {
		return ResolvedDeadline{Date: adHocDue, Source: DeadlineSourceAdHocApproved, Accuracy: AccuracyExact}
	}
	if plannedDate != "" {
		return ResolvedDeadline{Date: plannedDate, Source: DeadlineSourcePlannedFallback, Accuracy: AccuracyExact}
	}
	return ResolvedDeadline{Date: "", Source: DeadlineSourceUnavailable, Accuracy: AccuracyUnavailable}
}
