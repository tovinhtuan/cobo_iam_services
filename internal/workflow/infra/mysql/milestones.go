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
