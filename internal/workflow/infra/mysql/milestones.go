package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	workflowapp "github.com/cobo/cobo_iam_services/internal/workflow/app"
)

type MilestoneRepository struct {
	db *sql.DB
}

func NewMilestoneRepository(db *sql.DB) *MilestoneRepository {
	return &MilestoneRepository{db: db}
}

// InsertStepMilestones bulk-inserts milestone rows; duplicate milestone_id is silently ignored
// (INSERT IGNORE) so the call is safe to retry.
func (r *MilestoneRepository) InsertStepMilestones(ctx context.Context, rows []workflowapp.StepMilestoneRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("milestone tx begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT IGNORE INTO workflow_step_milestones
			(milestone_id, company_id, workflow_instance_id, step_id, step_order, milestone_type, scheduled_date)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("milestone prepare: %w", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		dateStr := row.ScheduledDate.Format("2006-01-02")
		if _, err := stmt.ExecContext(ctx,
			row.MilestoneID, row.CompanyID, row.InstanceID, row.StepID,
			row.StepOrder, row.MilestoneType, dateStr,
		); err != nil {
			return fmt.Errorf("milestone insert %s: %w", row.MilestoneID, err)
		}
	}
	return tx.Commit()
}

func (r *MilestoneRepository) ListByInstance(ctx context.Context, companyID, workflowInstanceID string) ([]workflowapp.InstanceReminderDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.milestone_id, m.workflow_instance_id, m.step_id, m.step_order, m.milestone_type, m.scheduled_date,
		       m.reminder_sent, m.reminder_occurrence_id, o.status, o.last_error_code
		FROM workflow_step_milestones m
		LEFT JOIN reminder_occurrences o ON o.occurrence_id = m.reminder_occurrence_id
			AND o.scope_type = 'WORKFLOW_STEP' AND o.disclosure_id = m.workflow_instance_id
		WHERE m.company_id = ? AND m.workflow_instance_id = ?
		ORDER BY m.step_order ASC, m.milestone_type ASC
	`, companyID, workflowInstanceID)
	if err != nil {
		return nil, fmt.Errorf("list milestones: %w", err)
	}
	defer rows.Close()
	out := make([]workflowapp.InstanceReminderDTO, 0)
	for rows.Next() {
		var item workflowapp.InstanceReminderDTO
		var sent int64 // scanned as int: tolerant of any non-boolean legacy value
		var occID, occStatus, occCode sql.NullString
		if err := rows.Scan(&item.ReminderID, &item.WorkflowInstanceID, &item.StepID, &item.StepIndex, &item.MilestoneType, &item.ReminderAt,
			&sent, &occID, &occStatus, &occCode); err != nil {
			return nil, err
		}
		item.Status = reminderStatusFromRow(sent, occID, occStatus, occCode)
		out = append(out, item)
	}
	return out, rows.Err()
}

// reminderStatusFromRow maps a milestone and its reminder occurrence to the API status
// (pending | sent | failed | cancelled). reminder_sent=1 only means "handled by the
// seeder"; the delivery outcome lives on the occurrence (RMC-06). A handled milestone
// without an occurrence was taken out of the queue as obsolete (REM-06).
func reminderStatusFromRow(sent int64, occurrenceID, occurrenceStatus, lastErrorCode sql.NullString) string {
	if sent == 0 {
		return "pending"
	}
	if !occurrenceID.Valid || strings.TrimSpace(occurrenceID.String) == "" {
		return "cancelled"
	}
	switch strings.ToUpper(strings.TrimSpace(occurrenceStatus.String)) {
	case "SENT":
		return "sent"
	case "FAILED":
		switch strings.ToUpper(strings.TrimSpace(lastErrorCode.String)) {
		case "REMINDER_EXPIRED", "REMINDER_OBSOLETE":
			return "cancelled"
		}
		return "failed"
	default:
		return "pending"
	}
}
