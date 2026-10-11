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

// ListDueMilestones returns the first page of due milestones (see ListDueMilestonesPage).
func (s *MilestoneScanner) ListDueMilestones(ctx context.Context, asOf time.Time, limit int) ([]reminderapp.DueMilestone, error) {
	return s.ListDueMilestonesPage(ctx, asOf, nil, limit)
}

// ListDueMilestonesPage returns milestone rows whose scheduled_date <= DATE(asOf) and
// reminder_sent=0, ordered by (scheduled_date, milestone_id) and starting strictly after the
// keyset cursor. Paging keeps rows that must wait from hiding later eligible rows (REM-06).
// Duplicate seeding across workers is prevented by the occurrence idempotency_key, not by
// row locks (this query runs in autocommit).
func (s *MilestoneScanner) ListDueMilestonesPage(ctx context.Context, asOf time.Time, after *reminderapp.MilestoneCursor, limit int) ([]reminderapp.DueMilestone, error) {
	asOfDate := asOf.UTC().Format("2006-01-02")
	afterDate, afterID := "0001-01-01", ""
	if after != nil {
		afterDate = after.ScheduledDate.UTC().Format("2006-01-02")
		afterID = after.MilestoneID
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT milestone_id, company_id, workflow_instance_id, step_id, milestone_type, scheduled_date
		FROM workflow_step_milestones
		WHERE reminder_sent = 0 AND scheduled_date <= ?
		  AND milestone_type LIKE 'due_minus_%'
		  AND (scheduled_date > ? OR (scheduled_date = ? AND milestone_id > ?))
		ORDER BY scheduled_date ASC, milestone_id ASC
		LIMIT ?
	`, asOfDate, afterDate, afterDate, afterID, limit)
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

// MarkMilestoneObsolete takes a milestone out of the due queue when its reminder can never
// be due again. It reuses reminder_sent=1 with a NULL occurrence id: reminder_sent is read as
// a bool by the workflow reminders API, so a new value (e.g. 2) would break that endpoint.
func (s *MilestoneScanner) MarkMilestoneObsolete(ctx context.Context, milestoneID string) error {
	if _, err := s.db.ExecContext(ctx, `
		UPDATE workflow_step_milestones
		SET reminder_sent = 1, reminder_occurrence_id = NULL
		WHERE milestone_id = ? AND reminder_sent = 0
	`, milestoneID); err != nil {
		return fmt.Errorf("mark milestone obsolete %s: %w", milestoneID, err)
	}
	return nil
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
