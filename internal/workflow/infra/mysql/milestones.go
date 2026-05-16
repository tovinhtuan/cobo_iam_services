package mysql

import (
	"context"
	"database/sql"
	"fmt"

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
		SELECT milestone_id, workflow_instance_id, step_id, step_order, milestone_type, scheduled_date, reminder_sent
		FROM workflow_step_milestones
		WHERE company_id = ? AND workflow_instance_id = ?
		ORDER BY step_order ASC, milestone_type ASC
	`, companyID, workflowInstanceID)
	if err != nil {
		return nil, fmt.Errorf("list milestones: %w", err)
	}
	defer rows.Close()
	out := make([]workflowapp.InstanceReminderDTO, 0)
	for rows.Next() {
		var item workflowapp.InstanceReminderDTO
		var sent bool
		if err := rows.Scan(&item.ReminderID, &item.WorkflowInstanceID, &item.StepID, &item.StepIndex, &item.MilestoneType, &item.ReminderAt, &sent); err != nil {
			return nil, err
		}
		if sent {
			item.Status = "sent"
		} else {
			item.Status = "pending"
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
