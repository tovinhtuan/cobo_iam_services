package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

const workflowStepReminderIdempotencyPrefix = "wsm"

// WorkflowStepReminderIdempotencyKey uniquely identifies a reminder occurrence by
// instance + step + offset (encoded in milestone_type due_minus_Nd).
func WorkflowStepReminderIdempotencyKey(instanceID, stepID, milestoneType string) string {
	return fmt.Sprintf("%s:%s:%s:%s", workflowStepReminderIdempotencyPrefix, instanceID, stepID, milestoneType)
}

func workflowStepReminderMilestoneType(idempotencyKey string) (string, bool) {
	parts := strings.Split(idempotencyKey, ":")
	if len(parts) < 4 || parts[0] != workflowStepReminderIdempotencyPrefix {
		return "", false
	}
	return parts[len(parts)-1], true
}

// ParseDueMinusReminderOffset extracts N from due_minus_Nd.
func ParseDueMinusReminderOffset(milestoneType string) (int, bool) {
	const prefix = "due_minus_"
	const suffix = "d"
	s := strings.TrimSpace(milestoneType)
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) {
		return 0, false
	}
	mid := strings.TrimSuffix(strings.TrimPrefix(s, prefix), suffix)
	n, err := strconv.Atoi(mid)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// IsDueMinusReminderMilestone reports whether the milestone is a configurable due-minus reminder.
func IsDueMinusReminderMilestone(milestoneType string) bool {
	_, ok := ParseDueMinusReminderOffset(milestoneType)
	return ok
}

func (s *service) workflowStepReminderEligible(ctx context.Context, m DueMilestone) bool {
	if s.stepTaskStateReader == nil {
		return false
	}
	state, err := s.stepTaskStateReader.StepTaskState(ctx, m.CompanyID, m.WorkflowInstanceID, m.StepID)
	if err != nil {
		return false
	}
	return state == WorkflowStepTaskPending
}
