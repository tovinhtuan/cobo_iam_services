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

// BuildManifest reads the template-owned draft candidate (or active publication
// when no draft exists). global_workflows is no longer editable authority.
func (r *VersionRepository) BuildManifest(ctx context.Context, typeID string) (wfcapp.Manifest, error) {
	var templateVersionNo int
	var raw []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT v.version_no, v.workflow_manifest_json
		FROM disclosure_types t
		INNER JOIN disclosure_type_versions v ON v.type_id = t.type_id
		WHERE t.type_id = ?
		  AND v.workflow_authority_mode = 'TEMPLATE_PINNED'
		  AND v.workflow_manifest_json IS NOT NULL
		  AND (v.version_no = t.active_version_no OR COALESCE(v.is_released, 0) = 0)
		ORDER BY
		  CASE WHEN v.version_no <> t.active_version_no AND COALESCE(v.is_released, 0) = 0 THEN 0 ELSE 1 END,
		  v.version_no DESC
		LIMIT 1
	`, typeID).Scan(&templateVersionNo, &raw)
	if err == sql.ErrNoRows {
		return wfcapp.Manifest{}, fmt.Errorf("no pinned template workflow candidate for type %q", typeID)
	}
	if err != nil {
		return wfcapp.Manifest{}, fmt.Errorf("build template manifest: %w", err)
	}
	var publication struct {
		Steps []struct {
			StepID          string                       `json:"step_id"`
			StepKey         string                       `json:"step_key"`
			Stage           string                       `json:"stage"`
			Description     string                       `json:"description"`
			Instructions    string                       `json:"instructions"`
			DepartmentID    string                       `json:"department_id"`
			AssigneeRoleIDs []string                     `json:"assignee_role_ids"`
			DueRule         string                       `json:"due_rule"`
			ProcessingDays  int                          `json:"processing_days"`
			DisplayOrder    int                          `json:"display_order"`
			ReminderConfig  *wfcapp.ManifestReminderConfig `json:"reminder_config"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(raw, &publication); err != nil {
		return wfcapp.Manifest{}, fmt.Errorf("decode template workflow manifest: %w", err)
	}
	m := wfcapp.Manifest{
		TypeID: typeID, WorkflowID: fmt.Sprintf("template:%s:%d", typeID, templateVersionNo),
		TemplateVersionNo: templateVersionNo, Steps: make([]wfcapp.ManifestStep, 0, len(publication.Steps)),
	}
	for _, step := range publication.Steps {
		role := ""
		if len(step.AssigneeRoleIDs) > 0 {
			role = step.AssigneeRoleIDs[0]
		}
		m.Steps = append(m.Steps, wfcapp.ManifestStep{
			StepID: step.StepID, StepKey: step.StepKey, Stage: step.Stage, Name: step.Stage,
			Description: step.Description, Instructions: step.Instructions, Role: role,
			DepartmentID: step.DepartmentID, DueRule: step.DueRule,
			ProcessingDays: step.ProcessingDays, DisplayOrder: step.DisplayOrder,
			ReminderConfig: step.ReminderConfig,
		})
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
			(type_id, version_no, template_version_no, state, steps_manifest_json, change_note, published_at, published_by)
		VALUES (?, ?, NULLIF(?, 0), 'published', ?, NULLIF(?, ''), ?, ?)
	`, m.TypeID, next, m.TemplateVersionNo, string(manifestJSON), changeNote, at.UTC(), actor); err != nil {
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

	var templateVersionNo int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(template_version_no, 0)
		FROM global_workflow_versions
		WHERE type_id = ? AND version_no = ? AND state IN ('published','active')
		FOR UPDATE
	`, typeID, versionNo).Scan(&templateVersionNo); err != nil {
		if err == sql.ErrNoRows {
			return wfcapp.VersionInfo{}, fmt.Errorf("version %d not activatable for type %q", versionNo, typeID)
		}
		return wfcapp.VersionInfo{}, err
	}
	if templateVersionNo <= 0 {
		return wfcapp.VersionInfo{}, fmt.Errorf("version %d is not mapped to a template publication", versionNo)
	}
	var authorityMode, candidateHash string
	var publicationRaw []byte
	if err := tx.QueryRowContext(ctx, `
		SELECT workflow_authority_mode, workflow_manifest_json, COALESCE(publication_candidate_hash, '')
		FROM disclosure_type_versions
		WHERE type_id = ? AND version_no = ?
		FOR UPDATE
	`, typeID, templateVersionNo).Scan(&authorityMode, &publicationRaw, &candidateHash); err != nil {
		return wfcapp.VersionInfo{}, fmt.Errorf("lock template publication: %w", err)
	}
	if authorityMode != "TEMPLATE_PINNED" || len(publicationRaw) == 0 || candidateHash == "" {
		return wfcapp.VersionInfo{}, fmt.Errorf("mapped template publication is not activation-ready")
	}

	// Compatibility history retains one active marker; runtime ignores it.
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
	// Canonical activation: release and point the template in this transaction.
	if _, err := tx.ExecContext(ctx,
		`UPDATE disclosure_type_versions
		 SET is_released = 1, activated_at = ?, updated_by = ?
		 WHERE type_id = ? AND version_no = ?`,
		at.UTC(), actor, typeID, templateVersionNo); err != nil {
		return wfcapp.VersionInfo{}, fmt.Errorf("release template publication: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE disclosure_types
		 SET active_version_no = ?, status = 'active', updated_at = CURRENT_TIMESTAMP
		 WHERE type_id = ?`,
		templateVersionNo, typeID); err != nil {
		return wfcapp.VersionInfo{}, fmt.Errorf("set template active pointer: %w", err)
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

func decodeManifestReminderConfig(raw []byte) *wfcapp.ManifestReminderConfig {
	if len(raw) == 0 {
		return nil
	}
	if raw[0] == '[' {
		return nil
	}
	var envelope struct {
		ReminderConfig *wfcapp.ManifestReminderConfig `json:"reminder_config"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.ReminderConfig != nil {
		return envelope.ReminderConfig
	}
	var cfg wfcapp.ManifestReminderConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil
	}
	if !cfg.Enabled && cfg.Mode == "" && len(cfg.DaysBefore) == 0 {
		return nil
	}
	return &cfg
}

func nullTime(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	u := t.Time.UTC()
	return &u
}
