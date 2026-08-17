package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
)

// WorkflowStepTaskStateReader implements reminderapp.WorkflowStepTaskStateReader over workflow_tasks.
type WorkflowStepTaskStateReader struct {
	db *sql.DB
}

func NewWorkflowStepTaskStateReader(db *sql.DB) *WorkflowStepTaskStateReader {
	return &WorkflowStepTaskStateReader{db: db}
}

func (r *WorkflowStepTaskStateReader) StepTaskState(ctx context.Context, companyID, workflowInstanceID, stepID string) (reminderapp.WorkflowStepTaskState, error) {
	companyID = strings.TrimSpace(companyID)
	workflowInstanceID = strings.TrimSpace(workflowInstanceID)
	stepID = strings.TrimSpace(stepID)
	if r == nil || r.db == nil || companyID == "" || workflowInstanceID == "" || stepID == "" {
		return reminderapp.WorkflowStepTaskAbsent, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT status FROM workflow_tasks
		WHERE company_id = ? AND workflow_instance_id = ? AND step_code = ?
		ORDER BY created_at DESC, task_id DESC
	`, companyID, workflowInstanceID, stepID)
	if err != nil {
		return reminderapp.WorkflowStepTaskAbsent, fmt.Errorf("step task state: %w", err)
	}
	defer rows.Close()
	found := false
	hasPending := false
	hasCompleted := false
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			return reminderapp.WorkflowStepTaskAbsent, fmt.Errorf("scan step task state: %w", err)
		}
		found = true
		switch strings.ToLower(strings.TrimSpace(status)) {
		case "pending":
			hasPending = true
		case "approved", "reviewed", "confirmed", "rejected", "completed":
			hasCompleted = true
		}
	}
	if err := rows.Err(); err != nil {
		return reminderapp.WorkflowStepTaskAbsent, err
	}
	if hasPending {
		return reminderapp.WorkflowStepTaskPending, nil
	}
	if found && hasCompleted {
		return reminderapp.WorkflowStepTaskCompleted, nil
	}
	if found {
		return reminderapp.WorkflowStepTaskCompleted, nil
	}
	return reminderapp.WorkflowStepTaskAbsent, nil
}
