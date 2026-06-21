package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	wfcapp "github.com/cobo/cobo_iam_services/internal/workflowconfig/app"
)

// VersionRepository implements workflowconfig/app.VersionRepository over global tables ONLY
// (global_workflows, global_workflow_steps, global_workflow_versions). It never references any
// company_template_workflow_override* (tenant) table — tenant isolation by construction.
type VersionRepository struct {
	db *sql.DB
}

func NewVersionRepository(db *sql.DB) *VersionRepository { return &VersionRepository{db: db} }

var _ wfcapp.VersionRepository = (*VersionRepository)(nil)

// BuildManifest reads the current editable workflow + steps for a type and returns a manifest.
func (r *VersionRepository) BuildManifest(ctx context.Context, typeID string) (wfcapp.Manifest, error) {
	var workflowID string
	err := r.db.QueryRowContext(ctx,
		`SELECT workflow_id FROM global_workflows WHERE type_id = ? AND status = 'active' LIMIT 1`, typeID).
		Scan(&workflowID)
	if err == sql.ErrNoRows {
		return wfcapp.Manifest{}, fmt.Errorf("no active global workflow for type %q", typeID)
	}
	if err != nil {
		return wfcapp.Manifest{}, fmt.Errorf("build manifest (workflow): %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT step_id, COALESCE(step_key,''), stage, COALESCE(assignee_role_ids, JSON_ARRAY()),
		       department_id, due_rule, processing_days, display_order
		FROM global_workflow_steps WHERE workflow_id = ?
		ORDER BY display_order ASC, step_key ASC
	`, workflowID)
	if err != nil {
		return wfcapp.Manifest{}, fmt.Errorf("build manifest (steps): %w", err)
	}
	defer rows.Close()
	m := wfcapp.Manifest{TypeID: typeID, WorkflowID: workflowID}
	for rows.Next() {
		var st wfcapp.ManifestStep
		var roleJSON []byte
		if err := rows.Scan(&st.StepID, &st.StepKey, &st.Stage, &roleJSON, &st.DepartmentID, &st.DueRule, &st.ProcessingDays, &st.DisplayOrder); err != nil {
			return wfcapp.Manifest{}, fmt.Errorf("scan step: %w", err)
		}
		st.Name = st.Stage
		st.Role = firstRole(roleJSON)
		m.Steps = append(m.Steps, st)
	}
	if err := rows.Err(); err != nil {
		return wfcapp.Manifest{}, err
	}
	return wfcapp.NormalizeManifest(m), nil
}

func (r *VersionRepository) ListVersions(ctx context.Context, typeID string) ([]wfcapp.VersionInfo, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT version_no, state, COALESCE(change_note,''), published_at, COALESCE(published_by,''),
		       activated_at, COALESCE(activated_by,'')
		FROM global_workflow_versions WHERE type_id = ? ORDER BY version_no ASC
	`, typeID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()
	var out []wfcapp.VersionInfo
	for rows.Next() {
		v := wfcapp.VersionInfo{TypeID: typeID}
		var pubAt, actAt sql.NullTime
		if err := rows.Scan(&v.VersionNo, &v.State, &v.ChangeNote, &pubAt, &v.PublishedBy, &actAt, &v.ActivatedBy); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		v.PublishedAt = nullTime(pubAt)
		v.ActivatedAt = nullTime(actAt)
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *VersionRepository) GetVersion(ctx context.Context, typeID string, versionNo int) (*wfcapp.VersionRecord, error) {
	return r.getVersionWhere(ctx, `type_id = ? AND version_no = ?`, typeID, versionNo)
}

func (r *VersionRepository) GetActiveVersion(ctx context.Context, typeID string) (*wfcapp.VersionRecord, error) {
	return r.getVersionWhere(ctx, `type_id = ? AND state = 'active'`, typeID)
}

func (r *VersionRepository) getVersionWhere(ctx context.Context, where string, args ...any) (*wfcapp.VersionRecord, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT type_id, version_no, state, COALESCE(change_note,''), steps_manifest_json,
		       published_at, COALESCE(published_by,''), activated_at, COALESCE(activated_by,'')
		FROM global_workflow_versions WHERE `+where+` LIMIT 1`, args...)
	var rec wfcapp.VersionRecord
	var manifestJSON []byte
	var pubAt, actAt sql.NullTime
	err := row.Scan(&rec.TypeID, &rec.VersionNo, &rec.State, &rec.ChangeNote, &manifestJSON,
		&pubAt, &rec.PublishedBy, &actAt, &rec.ActivatedBy)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get version: %w", err)
	}
	rec.PublishedAt = nullTime(pubAt)
	rec.ActivatedAt = nullTime(actAt)
	if len(manifestJSON) > 0 {
		_ = json.Unmarshal(manifestJSON, &rec.Manifest)
	}
	return &rec, nil
}

func (r *VersionRepository) Publish(ctx context.Context, m wfcapp.Manifest, changeNote, actor string, at time.Time) (wfcapp.VersionInfo, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return wfcapp.VersionInfo{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var next int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version_no),0)+1 FROM global_workflow_versions WHERE type_id = ?`, m.TypeID).
		Scan(&next); err != nil {
		return wfcapp.VersionInfo{}, fmt.Errorf("next version: %w", err)
	}
	m.VersionNo = next
	manifestJSON, err := wfcapp.ManifestJSON(m)
	if err != nil {
		return wfcapp.VersionInfo{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO global_workflow_versions
			(type_id, version_no, state, steps_manifest_json, change_note, published_at, published_by)
		VALUES (?, ?, 'published', ?, NULLIF(?, ''), ?, ?)
	`, m.TypeID, next, string(manifestJSON), changeNote, at.UTC(), actor); err != nil {
		return wfcapp.VersionInfo{}, fmt.Errorf("insert version: %w", err)
	}
	// Pointer on global_workflows (best-effort; source of truth is the version state).
	if _, err := tx.ExecContext(ctx,
		`UPDATE global_workflows SET published_version_no = ? WHERE type_id = ?`, next, m.TypeID); err != nil {
		return wfcapp.VersionInfo{}, fmt.Errorf("set published pointer: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return wfcapp.VersionInfo{}, err
	}
	pub := at.UTC()
	return wfcapp.VersionInfo{TypeID: m.TypeID, VersionNo: next, State: wfcapp.VersionStatePublished, ChangeNote: changeNote, PublishedAt: &pub, PublishedBy: actor}, nil
}

func (r *VersionRepository) Activate(ctx context.Context, typeID string, versionNo int, actor string, at time.Time) (wfcapp.VersionInfo, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return wfcapp.VersionInfo{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// Demote the currently-active version (if different) → 'published'. Ensures a single active.
	if _, err := tx.ExecContext(ctx,
		`UPDATE global_workflow_versions SET state = 'published'
		 WHERE type_id = ? AND state = 'active' AND version_no <> ?`, typeID, versionNo); err != nil {
		return wfcapp.VersionInfo{}, fmt.Errorf("demote active: %w", err)
	}
	// Activate the target version.
	res, err := tx.ExecContext(ctx,
		`UPDATE global_workflow_versions SET state = 'active', activated_at = ?, activated_by = ?
		 WHERE type_id = ? AND version_no = ? AND state IN ('published','active')`, at.UTC(), actor, typeID, versionNo)
	if err != nil {
		return wfcapp.VersionInfo{}, fmt.Errorf("activate version: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return wfcapp.VersionInfo{}, fmt.Errorf("version %d not activatable for type %q", versionNo, typeID)
	}
	// Pointer on global_workflows.
	if _, err := tx.ExecContext(ctx,
		`UPDATE global_workflows SET active_version_no = ? WHERE type_id = ?`, versionNo, typeID); err != nil {
		return wfcapp.VersionInfo{}, fmt.Errorf("set active pointer: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return wfcapp.VersionInfo{}, err
	}
	act := at.UTC()
	return wfcapp.VersionInfo{TypeID: typeID, VersionNo: versionNo, State: wfcapp.VersionStateActive, ActivatedAt: &act, ActivatedBy: actor}, nil
}

func firstRole(roleJSON []byte) string {
	var roles []string
	if err := json.Unmarshal(roleJSON, &roles); err != nil || len(roles) == 0 {
		return ""
	}
	return roles[0]
}

func nullTime(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	u := t.Time.UTC()
	return &u
}
