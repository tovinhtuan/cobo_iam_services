package mysql

import (
	"context"
)

// proposalSchemaCaps describes optional ad_hoc_proposals columns for mixed-version deploy.
type proposalSchemaCaps struct {
	includeDeadlineDays bool
	includeDayType      bool
}

// hasProposedDeadlineDaysColumn detects (once, then caches) whether
// ad_hoc_proposals.proposed_deadline_days exists.
//
// CF-16: a sync.Once-based cache permanently poisons every future caller if the
// very first information_schema lookup hits a transient error (context
// cancellation, connection blip) — there is no self-healing. Instead, only a
// *successful* lookup is cached; a failed attempt is retried on the next call.
// Guarded by a mutex so concurrent callers neither race on the cached fields
// nor issue redundant queries once a result is cached.
func (r *Repository) hasProposedDeadlineDaysColumn(ctx context.Context) (bool, error) {
	r.deadlineDaysColMu.RLock()
	if r.deadlineDaysColCached {
		ok := r.deadlineDaysColOK
		r.deadlineDaysColMu.RUnlock()
		return ok, nil
	}
	r.deadlineDaysColMu.RUnlock()

	r.deadlineDaysColMu.Lock()
	defer r.deadlineDaysColMu.Unlock()
	if r.deadlineDaysColCached {
		return r.deadlineDaysColOK, nil
	}

	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
		  AND table_name = 'ad_hoc_proposals'
		  AND column_name = 'proposed_deadline_days'
	`).Scan(&count)
	if err != nil {
		// Transient failure: do not cache, allow the next caller to retry.
		return false, err
	}
	r.deadlineDaysColOK = count > 0
	r.deadlineDaysColCached = true
	return r.deadlineDaysColOK, nil
}

// hasProposedDeadlineDayTypeColumn mirrors CF-16 caching for proposed_deadline_day_type.
func (r *Repository) hasProposedDeadlineDayTypeColumn(ctx context.Context) (bool, error) {
	r.deadlineDayTypeColMu.RLock()
	if r.deadlineDayTypeColCached {
		ok := r.deadlineDayTypeColOK
		r.deadlineDayTypeColMu.RUnlock()
		return ok, nil
	}
	r.deadlineDayTypeColMu.RUnlock()

	r.deadlineDayTypeColMu.Lock()
	defer r.deadlineDayTypeColMu.Unlock()
	if r.deadlineDayTypeColCached {
		return r.deadlineDayTypeColOK, nil
	}

	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
		  AND table_name = 'ad_hoc_proposals'
		  AND column_name = 'proposed_deadline_day_type'
	`).Scan(&count)
	if err != nil {
		return false, err
	}
	r.deadlineDayTypeColOK = count > 0
	r.deadlineDayTypeColCached = true
	return r.deadlineDayTypeColOK, nil
}

func (r *Repository) loadProposalSchemaCaps(ctx context.Context) (proposalSchemaCaps, error) {
	days, err := r.hasProposedDeadlineDaysColumn(ctx)
	if err != nil {
		return proposalSchemaCaps{}, err
	}
	dayType, err := r.hasProposedDeadlineDayTypeColumn(ctx)
	if err != nil {
		return proposalSchemaCaps{}, err
	}
	return proposalSchemaCaps{includeDeadlineDays: days, includeDayType: dayType}, nil
}

func (r *Repository) proposalDetailColumns(ctx context.Context) (cols string, caps proposalSchemaCaps, err error) {
	caps, err = r.loadProposalSchemaCaps(ctx)
	if err != nil {
		return "", proposalSchemaCaps{}, err
	}
	cols = `proposal_id, company_id, type_id, status, proposed_workflow_json,
		       proposed_t0_date, proposed_deadline_date`
	if caps.includeDeadlineDays {
		cols += `, proposed_deadline_days`
	}
	if caps.includeDayType {
		cols += `, proposed_deadline_day_type`
	}
	cols += `, change_note,
		       final_t0_date, final_deadline_date, adjustment_note,
		       focal_approved_by, focal_approved_at, admin_approved_by, admin_approved_at,
		       rejected_by, rejected_at, reject_reason,
		       record_id, workflow_instance_id, created_by, process_controller_id, created_at, updated_at`
	return cols, caps, nil
}
