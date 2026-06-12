package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
)

// MilestoneScanner implements reminderapp.MilestoneScanner by querying workflow_step_milestones.
type MilestoneScanner struct {
	db *sql.DB
}

func NewMilestoneScanner(db *sql.DB) *MilestoneScanner {
	return &MilestoneScanner{db: db}
}

// ListDueMilestones returns milestone rows whose scheduled_date <= DATE(asOf) and reminder_sent=0.
// Uses FOR UPDATE SKIP LOCKED to allow concurrent worker instances without double-sending.
func (s *MilestoneScanner) ListDueMilestones(ctx context.Context, asOf time.Time, limit int) ([]reminderapp.DueMilestone, error) {
	asOfDate := asOf.UTC().Format("2006-01-02")
	rows, err := s.db.QueryContext(ctx, `
		SELECT milestone_id, company_id, workflow_instance_id, step_id, milestone_type, scheduled_date
		FROM workflow_step_milestones
		WHERE reminder_sent = 0 AND scheduled_date <= ?
		ORDER BY scheduled_date ASC
		LIMIT ?
		FOR UPDATE SKIP LOCKED
	`, asOfDate, limit)
	if err != nil {
		return nil, fmt.Errorf("list due milestones: %w", err)
	}
	defer rows.Close()

	out := make([]reminderapp.DueMilestone, 0, limit)
	for rows.Next() {
		var m reminderapp.DueMilestone
		var scheduledDate time.Time
		if err := rows.Scan(&m.MilestoneID, &m.CompanyID, &m.WorkflowInstanceID, &m.StepID, &m.MilestoneType, &scheduledDate); err != nil {
			return nil, fmt.Errorf("scan due milestone: %w", err)
		}
		m.ScheduledDate = time.Date(scheduledDate.Year(), scheduledDate.Month(), scheduledDate.Day(), 0, 0, 0, 0, time.UTC)
		out = append(out, m)
	}
	return out, rows.Err()
}

// MarkMilestoneSent sets reminder_sent=1 and records the occurrence_id on the milestone row.
func (s *MilestoneScanner) MarkMilestoneSent(ctx context.Context, milestoneID, occurrenceID string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE workflow_step_milestones
		SET reminder_sent = 1, reminder_occurrence_id = ?
		WHERE milestone_id = ? AND reminder_sent = 0
	`, occurrenceID, milestoneID)
	if err != nil {
		return fmt.Errorf("mark milestone sent %s: %w", milestoneID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// Already marked by another worker — not an error (idempotent).
		return nil
	}
	return nil
}
