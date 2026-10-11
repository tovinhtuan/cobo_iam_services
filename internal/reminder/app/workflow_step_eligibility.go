package app

import (
	"strings"
	"time"
)

// stepReminderDecision is the outcome of evaluating one due-minus milestone.
type stepReminderDecision int

const (
	// decisionEligible: seed the occurrence now.
	decisionEligible stepReminderDecision = iota
	// decisionWait: keep the milestone; it may become due later (step not current yet).
	decisionWait
	// decisionObsolete: the reminder can never be due again; take the milestone out of the queue.
	decisionObsolete
)

const (
	reasonInstanceMissing   = "INSTANCE_MISSING"
	reasonInstanceClosed    = "INSTANCE_CLOSED"
	reasonStepNotInSnapshot = "STEP_NOT_IN_SNAPSHOT"
	reasonStepCompleted     = "STEP_COMPLETED"
	reasonStepExpired       = "STEP_EXPIRED"
	reasonWorkflowExpired   = "WORKFLOW_PLAN_EXPIRED"
	reasonStepNotCurrent    = "STEP_NOT_CURRENT"
	reasonDelayShifted      = "DELAY_SHIFTED"
)

// reminderStepGraceDays: a step reminder stays possible up to this many days after the
// step's planned end (contract v1.2 §2.2); after that the milestone is obsolete.
const reminderStepGraceDays = 2

func isProposalWorkflowSource(source string) bool {
	return strings.HasPrefix(strings.TrimSpace(source), "proposal_snapshot")
}

func isClosedInstanceStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "approved", "rejected", "cancelled", "canceled", "completed", "done":
		return true
	}
	return false
}

func findRuntimeStep(rt *WorkflowStepRuntime, stepID string) (WorkflowStepRuntimeStep, bool) {
	stepID = strings.TrimSpace(stepID)
	for _, st := range rt.Steps {
		if stepID != "" && (strings.TrimSpace(st.StepID) == stepID || strings.TrimSpace(st.StepCode) == stepID) {
			return st, true
		}
	}
	return WorkflowStepRuntimeStep{}, false
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// decideWorkflowStepReminder decides whether the due_minus_<offset>d reminder of stepID is
// due on today (a company-timezone date encoded at 00:00 UTC).
//
// Proposal workflows (v2/v3) advance one task per step, so the task state is authoritative.
// Other workflows (global_template / company_override) only ever create the first task;
// their progress lives in workflow_instance_step_states, exposed through rt (RMC-01).
func decideWorkflowStepReminder(rt *WorkflowStepRuntime, task WorkflowStepTaskState, stepID string, offset int, today time.Time) (stepReminderDecision, string) {
	if rt == nil {
		return decisionObsolete, reasonInstanceMissing
	}
	if isClosedInstanceStatus(rt.InstanceStatus) {
		return decisionObsolete, reasonInstanceClosed
	}
	st, ok := findRuntimeStep(rt, stepID)
	if !ok {
		return decisionObsolete, reasonStepNotInSnapshot
	}
	today = dateOnly(today)
	if st.Completed {
		return decisionObsolete, reasonStepCompleted
	}
	active := st.Current
	if isProposalWorkflowSource(rt.WorkflowSource) {
		switch task {
		case WorkflowStepTaskPending:
			active = true
		case WorkflowStepTaskCompleted:
			return decisionObsolete, reasonStepCompleted
		default:
			active = false
		}
	}
	if !active {
		// A later step must not lose its reminders because an earlier step runs late (PERF-48).
		// It waits until it becomes active, unless the whole workflow plan is long past.
		if last, ok := lastStepEnd(rt); ok && today.After(last.AddDate(0, 0, reminderStepGraceDays)) {
			return decisionObsolete, reasonWorkflowExpired
		}
		return decisionWait, reasonStepNotCurrent
	}
	if st.HasEnd && today.After(dateOnly(st.EndDate).AddDate(0, 0, reminderStepGraceDays)) {
		return decisionObsolete, reasonStepExpired
	}
	// RMC-04: a predecessor delay shifted this step's end; the reminder moves with it.
	if st.HasEnd && offset > 0 && dateOnly(st.EndDate).AddDate(0, 0, -offset).After(today) {
		return decisionWait, reasonDelayShifted
	}
	return decisionEligible, ""
}

func lastStepEnd(rt *WorkflowStepRuntime) (time.Time, bool) {
	var last time.Time
	found := false
	for _, st := range rt.Steps {
		if st.HasEnd && (!found || st.EndDate.After(last)) {
			last = dateOnly(st.EndDate)
			found = true
		}
	}
	return last, found
}

// reminderScheduledAt returns when the occurrence is due: the milestone date, moved later
// when the step end was delay-shifted (RMC-04).
func reminderScheduledAt(m DueMilestone, rt *WorkflowStepRuntime, offset int) time.Time {
	at := m.ScheduledDate.UTC()
	if rt == nil || offset <= 0 {
		return at
	}
	if st, ok := findRuntimeStep(rt, m.StepID); ok && st.HasEnd {
		if shifted := dateOnly(st.EndDate).AddDate(0, 0, -offset); shifted.After(at) {
			return shifted
		}
	}
	return at
}
