package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// Sprint 3 / Batch 2 — Workflow Override Staleness Detection.
//
// Every function in this file is NEW and narrow. None of them is called by, or calls,
// GetEffectiveWorkflow / GetCompanyWorkflowOverride (internal/disclosure/infra/mysql/repository.go)
// — see docs/ai-cache/workflow-override-foundation-batch2/PREFLIGHT_AUDIT.md §6. The only WRITE
// in this file touches exactly stale_status and last_rebase_check_at, both added by Batch 1
// specifically for this purpose — never workflow_json, active_version_no, or status.

// TypeExists is the minimal existence check backing the task's required "404 only if typeId does
// not exist" rule — deliberately independent of GetTypeDetail (which does much more work and
// also enforces company-scoping semantics this status endpoint doesn't need: a type's existence
// is a global fact, not a per-company one).
func (r *Repository) TypeExists(ctx context.Context, typeID string) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM disclosure_types WHERE type_id = ? LIMIT 1`, typeID).Scan(&one)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("type exists check: %w", err)
	}
	return true, nil
}

// GetOverrideStalenessMetadata reads exactly the Batch-1 columns the comparator needs, for one
// (company_id, type_id) pair. Returns ok=false (no error) when no row exists at all — the
// caller's Rule 1 ("no override") path, not an error condition.
func (r *Repository) GetOverrideStalenessMetadata(ctx context.Context, companyID, typeID string) (*disclosureapp.OverrideStalenessRow, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT active_version_no, base_source, base_workflow_id, base_version_no, base_hash, stale_status, last_rebase_check_at
		FROM company_template_workflow_overrides
		WHERE company_id = ? AND type_id = ?
	`, companyID, typeID)

	var activeVersionNo int
	var baseSource, staleStatus string
	var baseWorkflowID, baseHash sql.NullString
	var baseVersionNo sql.NullInt64
	var lastRebaseCheckAt sql.NullTime

	if err := row.Scan(&activeVersionNo, &baseSource, &baseWorkflowID, &baseVersionNo, &baseHash, &staleStatus, &lastRebaseCheckAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("get override staleness metadata: %w", err)
	}

	out := &disclosureapp.OverrideStalenessRow{
		// HasOverride mirrors the resolver's own definition of "is this override actually
		// winning" (repository.go:1183's `view.ActiveVersion != nil` check, which is reachable
		// only when active_version_no > 0 — see GetCompanyWorkflowOverride). A draft-only row
		// (active_version_no == 0) is NOT a "has_override=true" case for this endpoint.
		HasOverride:       activeVersionNo > 0,
		BaseSource:        baseSource,
		StaleStatus:        staleStatus,
		LastRebaseCheckAt:  nullTimePtr(lastRebaseCheckAt),
	}
	if baseWorkflowID.Valid {
		out.BaseWorkflowID = &baseWorkflowID.String
	}
	if baseVersionNo.Valid {
		v := int(baseVersionNo.Int64)
		out.BaseVersionNo = &v
	}
	if baseHash.Valid {
		out.BaseHash = &baseHash.String
	}
	return out, true, nil
}

// GetCurrentGlobalActiveVersionNo reads the type's CURRENT active global_workflow_versions
// number, independent of any company override — i.e. exactly what loadActiveGlobalWorkflow
// (repository.go / global_workflow_read.go) would resolve to for any company with NO override.
// Deliberately a separate, narrower query — does not decode steps_manifest_json at all, since
// the comparator only needs the version NUMBER, never the step content.
func (r *Repository) GetCurrentGlobalActiveVersionNo(ctx context.Context, typeID string) (*int, error) {
	var versionNo sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT w.active_version_no
		FROM global_workflows w
		WHERE w.type_id = ? AND w.status = 'active' AND w.active_version_no IS NOT NULL
		LIMIT 1
	`, typeID).Scan(&versionNo)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get current global active version: %w", err)
	}
	return nullInt(versionNo), nil
}

// UpdateOverrideStaleness writes ONLY stale_status and last_rebase_check_at. Per the task's
// explicit forbidden-writes list, this function must never be extended to also write
// workflow_json, active_version_no, status, base_source, base_workflow_id, base_version_no, or
// base_hash — those require a separate, explicitly-approved change, not a Batch 2 default. A
// no-op (zero rows affected) when no override row exists is not an error — it's Rule 1's "no DB
// write required" case naturally falling out of an unconditional WHERE match on zero rows.
func (r *Repository) UpdateOverrideStaleness(ctx context.Context, companyID, typeID, staleStatus string, checkedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE company_template_workflow_overrides
		SET stale_status = ?, last_rebase_check_at = ?
		WHERE company_id = ? AND type_id = ?
	`, staleStatus, checkedAt, companyID, typeID)
	if err != nil {
		return fmt.Errorf("update override staleness: %w", err)
	}
	return nil
}

func nullTimePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}
