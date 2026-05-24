package mysql

import (
	"context"
)

func (r *Repository) hasProposedDeadlineDaysColumn(ctx context.Context) (bool, error) {
	r.deadlineDaysColOnce.Do(func() {
		var count int
		err := r.db.QueryRowContext(ctx, `
			SELECT COUNT(1)
			FROM information_schema.columns
			WHERE table_schema = DATABASE()
			  AND table_name = 'ad_hoc_proposals'
			  AND column_name = 'proposed_deadline_days'
		`).Scan(&count)
		r.deadlineDaysColErr = err
		r.deadlineDaysColOK = count > 0
	})
	return r.deadlineDaysColOK, r.deadlineDaysColErr
}

func (r *Repository) proposalDetailColumns(ctx context.Context) (cols string, includeDeadlineDays bool, err error) {
	includeDeadlineDays, err = r.hasProposedDeadlineDaysColumn(ctx)
	if err != nil {
		return "", false, err
	}
	cols = `proposal_id, company_id, type_id, status, proposed_workflow_json,
		       proposed_t0_date, proposed_deadline_date`
	if includeDeadlineDays {
		cols += `, proposed_deadline_days`
	}
	cols += `, change_note,
		       final_t0_date, final_deadline_date, adjustment_note,
		       focal_approved_by, focal_approved_at, admin_approved_by, admin_approved_at,
		       rejected_by, rejected_at, reject_reason,
		       record_id, workflow_instance_id, created_by, process_controller_id, created_at, updated_at`
	return cols, includeDeadlineDays, nil
}
