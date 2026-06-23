package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// Sprint 3 / Batch 5 — Workflow Override Rebase Apply.
//
// ApplyWorkflowOverrideRebase mirrors ApproveCompanyWorkflowOverride's exact transactional shape
// (repository.go:1004) — lock the override row, insert one new version row, flip
// active_version_no — with two additions specific to a REBASE apply rather than a manual edit
// approval: (1) a race guard comparing the locked row's CURRENT active_version_no against what
// the service layer computed its preview against, so a concurrent edit/approve/apply between
// preview and this transaction can never be silently overwritten; (2) resetting the staleness
// metadata columns (stale_status, base_version_no, base_hash, last_rebase_check_at) Batch 1/2
// introduced, since a successful rebase is, by definition, what makes an override "current"
// again. No write touches any table besides company_template_workflow_override_versions and
// company_template_workflow_overrides (DB_WRITE_BOUNDARY_REPORT.md).
func (r *Repository) ApplyWorkflowOverrideRebase(ctx context.Context, params disclosureapp.ApplyWorkflowOverrideRebaseParams) (*disclosureapp.ApplyWorkflowOverrideRebaseResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var overrideID string
	var currentActiveVersionNo int
	if err := tx.QueryRowContext(ctx, `
		SELECT override_id, active_version_no
		FROM company_template_workflow_overrides
		WHERE company_id = ? AND type_id = ?
		FOR UPDATE
	`, params.CompanyID, params.TypeID).Scan(&overrideID, &currentActiveVersionNo); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeOverrideNotFound, "workflow override not found", nil)
		}
		return nil, err
	}

	// Race guard: the service layer's preview/conflict computation ran against
	// ExpectedActiveVersionNo. If another request (edit, approve, or a concurrent apply) changed
	// active_version_no since then, this apply's patch was computed against state that is no
	// longer current — reject rather than risk silently clobbering that other change.
	if currentActiveVersionNo != params.ExpectedActiveVersionNo {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "override was modified concurrently; re-run rebase-preview and retry", nil)
	}

	var nextVersion int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version_no), 0) + 1
		FROM company_template_workflow_override_versions
		WHERE override_id = ?
	`, overrideID).Scan(&nextVersion); err != nil {
		return nil, err
	}

	workflowJSON, err := json.Marshal(params.NewSnapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal new snapshot: %w", err)
	}
	changeNote := "rebase-apply"
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO company_template_workflow_override_versions (
			override_id, version_no, workflow_json, change_note, state,
			created_by, approved_by, approved_at, created_at, updated_at
		) VALUES (?, ?, CAST(? AS JSON), ?, 'approved', ?, ?, ?, ?, ?)
	`, overrideID, nextVersion, workflowJSON, changeNote,
		params.UserID, params.UserID, params.Now, params.Now, params.Now); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE company_template_workflow_overrides
		SET active_version_no = ?, status = 'approved',
		    base_source = 'global_workflow', base_version_no = ?, base_hash = NULL,
		    stale_status = 'current', last_rebase_check_at = ?,
		    approved_by = ?, approved_at = ?, updated_by = ?, updated_at = ?
		WHERE override_id = ?
	`, nextVersion, params.NewBaseVersionNo, params.Now,
		params.UserID, params.Now, params.UserID, params.Now, overrideID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &disclosureapp.ApplyWorkflowOverrideRebaseResult{
		OverrideID:   overrideID,
		NewVersionNo: nextVersion,
		AppliedAt:    params.Now,
	}, nil
}
