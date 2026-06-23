package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// Sprint 3 / Batch 4 — Workflow Override Conflict Detection.
//
// Every write in this file touches workflow_override_conflicts exclusively (Batch 4's locked
// scope — DB_WRITE_BOUNDARY_REPORT.md). UpsertWorkflowOverrideConflicts uses `id`/`conflict_key`
// (the two are always equal — id is itself derived from conflict_key, see
// workflow_override_conflict.go's BuildConflictKey) as the upsert key, implementing Batch 4's
// chosen idempotency strategy (Option B, PREFLIGHT_AUDIT.md §8): repeated preview calls against
// the same (company, type, base_version_no, target_version_no, step_key, field_path,
// conflict_type) tuple update the SAME row rather than inserting a duplicate.

// UpsertWorkflowOverrideConflicts persists each detected conflict, INSERT-ing a new row the
// first time a given conflict_key is seen, or UPDATE-ing the existing row's content (global
// values may shift between preview calls if the target version changes) WITHOUT touching its
// resolution_status/resolution/resolved_by/resolved_at — a conflict already resolved by an admin
// must not be silently reset to unresolved just because a later preview re-detects the same
// (step_key, field_path, conflict_type) divergence.
func (r *Repository) UpsertWorkflowOverrideConflicts(ctx context.Context, inputs []disclosureapp.PersistedConflictInput) ([]disclosureapp.PersistedConflictDTO, error) {
	if len(inputs) == 0 {
		return []disclosureapp.PersistedConflictDTO{}, nil
	}
	out := make([]disclosureapp.PersistedConflictDTO, 0, len(inputs))
	for _, in := range inputs {
		conflictKey := disclosureapp.BuildConflictKey(in.CompanyID, in.TypeID, in.BaseVersionNo, in.TargetVersionNo, in.StepKey, in.FieldPath, in.ConflictType)
		id := conflictKey

		globalOldJSON, err := json.Marshal(in.GlobalOld)
		if err != nil {
			return nil, fmt.Errorf("marshal global_old for conflict %s: %w", conflictKey, err)
		}
		globalNewJSON, err := json.Marshal(in.GlobalNew)
		if err != nil {
			return nil, fmt.Errorf("marshal global_new for conflict %s: %w", conflictKey, err)
		}
		companyValueJSON, err := json.Marshal(in.CompanyValue)
		if err != nil {
			return nil, fmt.Errorf("marshal company_value for conflict %s: %w", conflictKey, err)
		}

		_, err = r.db.ExecContext(ctx, `
			INSERT INTO workflow_override_conflicts (
				id, company_id, type_id, override_id, override_version_no, preview_id,
				base_version_no, target_version_no, conflict_key, step_key, field_path,
				severity, conflict_type, global_old_json, global_new_json, company_value_json,
				resolution_status, created_by
			) VALUES (?, ?, ?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'unresolved', ?)
			ON DUPLICATE KEY UPDATE
				severity = VALUES(severity),
				global_old_json = VALUES(global_old_json),
				global_new_json = VALUES(global_new_json),
				company_value_json = VALUES(company_value_json),
				override_version_no = VALUES(override_version_no)
		`, id, in.CompanyID, in.TypeID, nullIfEmpty(in.OverrideID), in.OverrideVersionNo,
			in.BaseVersionNo, in.TargetVersionNo, conflictKey, in.StepKey, in.FieldPath,
			in.Severity, in.ConflictType, globalOldJSON, globalNewJSON, companyValueJSON,
			in.CreatedBy)
		if err != nil {
			return nil, fmt.Errorf("upsert workflow override conflict %s: %w", conflictKey, err)
		}

		persisted, _, err := r.scanWorkflowOverrideConflictByID(ctx, in.CompanyID, in.TypeID, id)
		if err != nil {
			return nil, err
		}
		if persisted != nil {
			persisted.ResolutionOptions = in.ResolutionOptions
			out = append(out, *persisted)
		}
	}
	return out, nil
}

// GetWorkflowOverrideConflict reads a single conflict row, scoped to (companyID, typeID) —
// returns nil (no error) if not found OR if it belongs to a different company/type, so the
// resolve handler's "404 if not found or not own company" requirement is satisfied by
// construction (the caller cannot distinguish "doesn't exist" from "exists but isn't yours",
// which is the correct, tenant-safe behavior — never leak existence of another company's row).
func (r *Repository) GetWorkflowOverrideConflict(ctx context.Context, companyID, typeID, conflictID string) (*disclosureapp.PersistedConflictDTO, error) {
	dto, _, err := r.scanWorkflowOverrideConflictByID(ctx, companyID, typeID, conflictID)
	return dto, err
}

// ResolveWorkflowOverrideConflict writes ONLY resolution_status, resolution, resolution_json,
// resolved_by, resolved_at, updated_at — never any other column, never any other table.
func (r *Repository) ResolveWorkflowOverrideConflict(ctx context.Context, companyID, typeID, conflictID, resolution string, resolutionValue any, resolvedBy string, resolvedAt time.Time) (*disclosureapp.PersistedConflictDTO, error) {
	resolutionJSON, err := json.Marshal(resolutionValue)
	if err != nil {
		return nil, fmt.Errorf("marshal resolution_value for conflict %s: %w", conflictID, err)
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE workflow_override_conflicts
		SET resolution_status = 'resolved', resolution = ?, resolution_json = ?,
		    resolved_by = ?, resolved_at = ?
		WHERE id = ? AND company_id = ? AND type_id = ?
	`, resolution, resolutionJSON, resolvedBy, resolvedAt, conflictID, companyID, typeID)
	if err != nil {
		return nil, fmt.Errorf("resolve workflow override conflict %s: %w", conflictID, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("resolve workflow override conflict %s: rows affected: %w", conflictID, err)
	}
	if affected == 0 {
		return nil, nil
	}
	dto, _, err := r.scanWorkflowOverrideConflictByID(ctx, companyID, typeID, conflictID)
	return dto, err
}

func (r *Repository) scanWorkflowOverrideConflictByID(ctx context.Context, companyID, typeID, conflictID string) (*disclosureapp.PersistedConflictDTO, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, company_id, type_id, COALESCE(override_id, ''), COALESCE(override_version_no, 0),
		       preview_id, base_version_no, target_version_no, step_key, field_path, severity,
		       conflict_type, global_old_json, global_new_json, company_value_json,
		       resolution_status, resolution, created_at, resolved_by, resolved_at
		FROM workflow_override_conflicts
		WHERE id = ? AND company_id = ? AND type_id = ?
	`, conflictID, companyID, typeID)

	var dto disclosureapp.PersistedConflictDTO
	var previewID, resolution, resolvedBy sql.NullString
	var globalOldRaw, globalNewRaw, companyValueRaw []byte
	var resolvedAt sql.NullTime

	err := row.Scan(&dto.ID, &dto.CompanyID, &dto.TypeID, &dto.OverrideID, &dto.OverrideVersionNo,
		&previewID, &dto.BaseVersionNo, &dto.TargetVersionNo, &dto.StepKey, &dto.FieldPath,
		&dto.Severity, &dto.ConflictType, &globalOldRaw, &globalNewRaw, &companyValueRaw,
		&dto.ResolutionStatus, &resolution, &dto.CreatedAt, &resolvedBy, &resolvedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("scan workflow override conflict %s: %w", conflictID, err)
	}

	if previewID.Valid {
		dto.PreviewID = &previewID.String
	}
	if resolution.Valid {
		dto.Resolution = &resolution.String
	}
	if resolvedBy.Valid {
		dto.ResolvedBy = &resolvedBy.String
	}
	if resolvedAt.Valid {
		t := resolvedAt.Time
		dto.ResolvedAt = &t
	}
	_ = json.Unmarshal(globalOldRaw, &dto.GlobalOld)
	_ = json.Unmarshal(globalNewRaw, &dto.GlobalNew)
	_ = json.Unmarshal(companyValueRaw, &dto.CompanyValue)

	return &dto, true, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
