package workflowassign

import (
	"fmt"
	"strings"
)

// TaskAssignedToMembershipSQL returns a SQL boolean expression for task alias matching one membership.
// Authority:
//   - relation rows present → relation only
//   - otherwise → singular assignee_membership_id
//
// The expression binds the membership ID twice (? , ?).
func TaskAssignedToMembershipSQL(taskAlias string) string {
	a := strings.TrimSpace(taskAlias)
	if a == "" {
		a = "wt"
	}
	return fmt.Sprintf(`(
  EXISTS (
    SELECT 1 FROM workflow_task_assignees wta
    WHERE wta.task_id = %s.task_id AND wta.membership_id = ?
  )
  OR (
    NOT EXISTS (
      SELECT 1 FROM workflow_task_assignees wta2
      WHERE wta2.task_id = %s.task_id
    )
    AND %s.assignee_membership_id = ?
  )
)`, a, a, a)
}

// TaskAssignedToMembershipsINSQL matches any of N membership IDs (relation-first authority).
// Binds membership placeholders twice (2*n args in the same order).
func TaskAssignedToMembershipsINSQL(taskAlias string, n int) string {
	if n <= 0 {
		return "1=0"
	}
	a := strings.TrimSpace(taskAlias)
	if a == "" {
		a = "wt"
	}
	ph := placeholders(n)
	return fmt.Sprintf(`(
  EXISTS (
    SELECT 1 FROM workflow_task_assignees wta
    WHERE wta.task_id = %s.task_id AND wta.membership_id IN (%s)
  )
  OR (
    NOT EXISTS (
      SELECT 1 FROM workflow_task_assignees wta2
      WHERE wta2.task_id = %s.task_id
    )
    AND %s.assignee_membership_id IN (%s)
  )
)`, a, ph, a, a, ph)
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	b := strings.Builder{}
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('?')
	}
	return b.String()
}
