package app

import (
	"strings"
)

// BuildListRowsScopeSQL returns an additional AND (...) clause for scoped list queries.
// When CanViewAll, returns empty string (no extra filter).
func BuildListRowsScopeSQL(scope DeadlineAlertAccessScope) (clause string, args []any) {
	if scope.CanViewAll {
		return "", nil
	}
	parts := make([]string, 0, 4)
	if len(scope.DepartmentIDs) > 0 {
		placeholders := strings.Repeat("?,", len(scope.DepartmentIDs))
		placeholders = placeholders[:len(placeholders)-1]
		parts = append(parts, "dr.department_id IN ("+placeholders+")")
		for _, id := range scope.DepartmentIDs {
			args = append(args, id)
		}
	}
	if scope.MembershipID != "" {
		parts = append(parts, `EXISTS (
			SELECT 1 FROM assignments a
			WHERE a.company_id = dr.company_id
			  AND a.resource_type = 'disclosure_record'
			  AND a.resource_id = dr.record_id
			  AND a.assignee_type = 'membership'
			  AND a.assignee_ref_id = ?
			  AND a.status = 'active'
		)`)
		args = append(args, scope.MembershipID)

		parts = append(parts, `EXISTS (
			SELECT 1 FROM workflow_tasks wt
			INNER JOIN workflow_instances wi_task ON wi_task.workflow_instance_id = wt.workflow_instance_id
				AND wi_task.company_id = dr.company_id
			WHERE wi_task.record_id = dr.record_id
			  AND wt.company_id = dr.company_id
			  AND (
			    EXISTS (
			      SELECT 1 FROM workflow_task_assignees wta
			      WHERE wta.task_id = wt.task_id AND wta.membership_id = ?
			    )
			    OR (
			      NOT EXISTS (
			        SELECT 1 FROM workflow_task_assignees wta2
			        WHERE wta2.task_id = wt.task_id
			      )
			      AND wt.assignee_membership_id = ?
			    )
			  )
			  AND LOWER(TRIM(wt.status)) NOT IN ('completed', 'done', 'cancelled', 'skipped')
		)`)
		args = append(args, scope.MembershipID, scope.MembershipID)
	}
	if len(parts) == 0 {
		return " AND 1=0", nil
	}
	return " AND (" + strings.Join(parts, " OR ") + ")", args
}
