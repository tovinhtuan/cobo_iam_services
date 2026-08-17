package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// ─── Deadline Rule Catalog ─────────────────────────────────────────────────────

func (r *Repository) ListActiveDeadlineRuleCatalog(ctx context.Context) ([]disclosureapp.DeadlineRuleCatalogDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT code, label_vi, pattern, input_type
		FROM deadline_rule_catalog
		WHERE is_active = 1
		ORDER BY display_order ASC, code ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list active deadline rule catalog: %w", err)
	}
	defer rows.Close()
	var out []disclosureapp.DeadlineRuleCatalogDTO
	for rows.Next() {
		var d disclosureapp.DeadlineRuleCatalogDTO
		if err := rows.Scan(&d.Code, &d.LabelVI, &d.Pattern, &d.InputType); err != nil {
			return nil, fmt.Errorf("scan deadline rule: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repository) ListCmsDeadlineRules(ctx context.Context) ([]disclosureapp.CmsDeadlineRuleDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT rule_id, code, label_vi, pattern, input_type, is_active, display_order, created_at, updated_at
		FROM deadline_rule_catalog
		ORDER BY display_order ASC, code ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list cms deadline rules: %w", err)
	}
	defer rows.Close()
	var out []disclosureapp.CmsDeadlineRuleDTO
	for rows.Next() {
		var d disclosureapp.CmsDeadlineRuleDTO
		var isActive int
		if err := rows.Scan(&d.RuleID, &d.Code, &d.LabelVI, &d.Pattern, &d.InputType, &isActive, &d.DisplayOrder, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan cms deadline rule: %w", err)
		}
		d.IsActive = isActive == 1
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repository) CreateDeadlineRule(ctx context.Context, req disclosureapp.CmsDeadlineRuleCreateRequest, ruleID string) (*disclosureapp.CmsDeadlineRuleDTO, error) {
	inputType := req.InputType
	if inputType == "" {
		inputType = "text"
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO deadline_rule_catalog (rule_id, code, label_vi, pattern, input_type, is_active, display_order, created_by, updated_by)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?)
	`, ruleID, req.Code, req.LabelVI, req.Pattern, inputType, req.DisplayOrder, req.Subject.UserID, req.Subject.UserID)
	if err != nil {
		return nil, fmt.Errorf("create deadline rule: %w", err)
	}
	return r.getDeadlineRuleByID(ctx, ruleID)
}

func (r *Repository) UpdateDeadlineRule(ctx context.Context, req disclosureapp.CmsDeadlineRuleUpdateRequest) (*disclosureapp.CmsDeadlineRuleDTO, error) {
	var isActiveVal int = 1
	if req.IsActive != nil && !*req.IsActive {
		isActiveVal = 0
	} else if req.IsActive != nil && *req.IsActive {
		isActiveVal = 1
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE deadline_rule_catalog
		SET label_vi = ?, pattern = ?, input_type = ?, is_active = ?, display_order = ?, updated_by = ?
		WHERE rule_id = ?
	`, req.LabelVI, req.Pattern, req.InputType, isActiveVal, req.DisplayOrder, req.Subject.UserID, req.RuleID)
	if err != nil {
		return nil, fmt.Errorf("update deadline rule: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "deadline rule not found", nil)
	}
	return r.getDeadlineRuleByID(ctx, req.RuleID)
}

func (r *Repository) DeleteDeadlineRule(ctx context.Context, ruleID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM deadline_rule_catalog WHERE rule_id = ?`, ruleID)
	if err != nil {
		return fmt.Errorf("delete deadline rule: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "deadline rule not found", nil)
	}
	return nil
}

func (r *Repository) getDeadlineRuleByID(ctx context.Context, ruleID string) (*disclosureapp.CmsDeadlineRuleDTO, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT rule_id, code, label_vi, pattern, input_type, is_active, display_order, created_at, updated_at
		FROM deadline_rule_catalog WHERE rule_id = ?
	`, ruleID)
	var d disclosureapp.CmsDeadlineRuleDTO
	var isActive int
	if err := row.Scan(&d.RuleID, &d.Code, &d.LabelVI, &d.Pattern, &d.InputType, &isActive, &d.DisplayOrder, &d.CreatedAt, &d.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "deadline rule not found", nil)
		}
		return nil, fmt.Errorf("get deadline rule: %w", err)
	}
	d.IsActive = isActive == 1
	return &d, nil
}

// ─── Global Workflows ─────────────────────────────────────────────────────────

func (r *Repository) CountGlobalWorkflowsByTypeId(ctx context.Context, typeID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM global_workflows WHERE type_id = ? AND status = 'active'`,
		typeID).Scan(&count)
	return count, err
}

// nullInt converts a nullable SQL int into *int (nil when NULL).
func nullInt(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

func (r *Repository) GetGlobalWorkflow(ctx context.Context, typeID string) (*disclosureapp.GlobalWorkflowDTO, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT workflow_id, type_id, status, COALESCE(change_note,''), created_by, updated_by, created_at, updated_at,
		       published_version_no, active_version_no
		FROM global_workflows WHERE type_id = ? AND status = 'active'
		LIMIT 1
	`, typeID)
	var wf disclosureapp.GlobalWorkflowDTO
	var pubNo, actNo sql.NullInt64
	if err := row.Scan(&wf.WorkflowID, &wf.TypeID, &wf.Status, &wf.ChangeNote, &wf.CreatedBy, &wf.UpdatedBy, &wf.CreatedAt, &wf.UpdatedAt, &pubNo, &actNo); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get global workflow: %w", err)
	}
	wf.PublishedVersionNo = nullInt(pubNo)
	wf.ActiveVersionNo = nullInt(actNo)
	steps, err := r.listGlobalWorkflowSteps(ctx, wf.WorkflowID)
	if err != nil {
		return nil, err
	}
	wf.Steps = steps
	return &wf, nil
}

func (r *Repository) listGlobalWorkflowSteps(ctx context.Context, workflowID string) ([]disclosureapp.GlobalWorkflowStepInput, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT step_id, COALESCE(step_key, ''), stage, description, instructions, department_id, assignee_role_ids, due_rule, processing_days, display_order, documents_json
		FROM global_workflow_steps WHERE workflow_id = ?
		ORDER BY display_order ASC, step_id ASC
	`, workflowID)
	if err != nil {
		return nil, fmt.Errorf("list global workflow steps: %w", err)
	}
	defer rows.Close()
	var out []disclosureapp.GlobalWorkflowStepInput
	for rows.Next() {
		var step disclosureapp.GlobalWorkflowStepInput
		var roleIDsJSON []byte
		var description sql.NullString
		var instructions sql.NullString
		var documentsJSON []byte
		if err := rows.Scan(&step.StepID, &step.StepKey, &step.Stage, &description, &instructions, &step.DepartmentID, &roleIDsJSON, &step.DueRule, &step.ProcessingDays, &step.DisplayOrder, &documentsJSON); err != nil {
			return nil, fmt.Errorf("scan global workflow step: %w", err)
		}
		step.Description = description.String
		step.Instructions = instructions.String
		if err := json.Unmarshal(roleIDsJSON, &step.AssigneeRoleIds); err != nil {
			step.AssigneeRoleIds = []string{}
		}
		step.ReminderConfig = disclosureapp.DecodeGlobalWorkflowStepReminderDocumentsJSON(documentsJSON)
		out = append(out, step)
	}
	return out, rows.Err()
}

func (r *Repository) UpsertGlobalWorkflow(ctx context.Context, req disclosureapp.CmsUpsertGlobalWorkflowRequest, workflowID string) (*disclosureapp.GlobalWorkflowDTO, error) {
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// mig-S1: read the existing step identity map BEFORE delete, so step_key is preserved
	// across the DELETE+INSERT upsert. Match incoming steps by step_key, then by step_id.
	existingKeyByStepID := map[string]string{} // old step_id -> step_key
	existingKeys := map[string]bool{}          // valid existing step_keys (server-owned)
	existingDocsByStepID := map[string][]byte{}
	existingDocsByKey := map[string][]byte{}
	stepRows, err := tx.QueryContext(ctx, `
		SELECT s.step_id, COALESCE(s.step_key, ''), s.documents_json
		FROM global_workflow_steps s
		JOIN global_workflows w ON w.workflow_id = s.workflow_id
		WHERE w.type_id = ?
	`, req.TypeID)
	if err != nil {
		return nil, fmt.Errorf("read existing step keys: %w", err)
	}
	for stepRows.Next() {
		var oldStepID, oldStepKey string
		var oldDocs []byte
		if err := stepRows.Scan(&oldStepID, &oldStepKey, &oldDocs); err != nil {
			_ = stepRows.Close()
			return nil, fmt.Errorf("scan existing step key: %w", err)
		}
		docsCopy := append([]byte(nil), oldDocs...)
		existingDocsByStepID[oldStepID] = docsCopy
		if oldStepKey != "" {
			existingKeyByStepID[oldStepID] = oldStepKey
			existingKeys[oldStepKey] = true
			existingDocsByKey[oldStepKey] = docsCopy
		}
	}
	if err := stepRows.Err(); err != nil {
		_ = stepRows.Close()
		return nil, fmt.Errorf("iterate existing step keys: %w", err)
	}
	_ = stepRows.Close()

	// Batch 3: read the version pointers BEFORE delete so save-draft does not reset them.
	// (DELETE+INSERT would otherwise null published_version_no/active_version_no.)
	var prevPubNo, prevActNo sql.NullInt64
	if err := tx.QueryRowContext(ctx,
		`SELECT published_version_no, active_version_no FROM global_workflows WHERE type_id = ? LIMIT 1`, req.TypeID).
		Scan(&prevPubNo, &prevActNo); err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("read version pointers: %w", err)
	}

	// Delete any existing workflow for this type (all statuses).
	// FK CASCADE deletes associated steps, preventing step_id primary key collisions
	// on the next upsert. "Upsert" semantics: exactly one workflow per type at all times.
	_, err = tx.ExecContext(ctx, `DELETE FROM global_workflows WHERE type_id = ?`, req.TypeID)
	if err != nil {
		return nil, fmt.Errorf("delete existing workflow: %w", err)
	}

	// Insert new active workflow, carrying the preserved version pointers (Batch 3).
	changeNote := req.ChangeNote
	_, err = tx.ExecContext(ctx, `
		INSERT INTO global_workflows (workflow_id, type_id, status, change_note, created_by, updated_by, created_at, updated_at, published_version_no, active_version_no)
		VALUES (?, ?, 'active', NULLIF(?, ''), ?, ?, ?, ?, ?, ?)
	`, workflowID, req.TypeID, changeNote, req.Subject.UserID, req.Subject.UserID, now, now,
		prevPubNo, prevActNo)
	if err != nil {
		return nil, fmt.Errorf("insert global workflow: %w", err)
	}

	// Insert steps with server-generated IDs to prevent caller-supplied step_id collisions.
	// step_key is the stable identity (mig-S1): preserved for existing steps, minted for new ones.
	usedKeys := map[string]bool{}
	for i, step := range req.Steps {
		roleIDsJSON, _ := json.Marshal(step.AssigneeRoleIds)
		displayOrder := step.DisplayOrder
		if displayOrder <= 0 {
			displayOrder = i + 1
		}
		stepID := fmt.Sprintf("%s-step-%d", workflowID, i+1)
		stepKey := disclosureapp.ResolveStepKey(step, existingKeys, existingKeyByStepID, usedKeys)
		usedKeys[stepKey] = true
		existingDocs := existingDocsByKey[stepKey]
		if len(existingDocs) == 0 {
			existingDocs = existingDocsByStepID[strings.TrimSpace(step.StepID)]
		}
		documentsJSON, encErr := disclosureapp.MergeGlobalWorkflowStepReminderDocumentsJSON(existingDocs, step.ReminderConfig)
		if encErr != nil {
			return nil, fmt.Errorf("encode reminder_config for step %d: %w", i, encErr)
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO global_workflow_steps (step_id, step_key, workflow_id, stage, description, instructions, department_id, assignee_role_ids, due_rule, processing_days, display_order, documents_json, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, stepID, stepKey, workflowID, step.Stage, step.Description, step.Instructions, step.DepartmentID, string(roleIDsJSON), step.DueRule, step.ProcessingDays, displayOrder, documentsJSON, now)
		if err != nil {
			return nil, fmt.Errorf("insert global workflow step %d: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return r.GetGlobalWorkflow(ctx, req.TypeID)
}

func (r *Repository) DeleteGlobalWorkflow(ctx context.Context, typeID string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM global_workflows WHERE type_id = ?
	`, typeID)
	if err != nil {
		return fmt.Errorf("delete global workflow: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "global workflow not found", nil)
	}
	return nil
}

// ─── System Template Archive ──────────────────────────────────────────────────

func (r *Repository) ArchiveGlobalTemplate(ctx context.Context, typeID, _ string) error {
	// company_id IS NULL identifies global (platform-managed) templates.
	// active_version_no = 0 ensures the template is hidden from Portal's ListTypes JOIN.
	res, err := r.db.ExecContext(ctx, `
		UPDATE disclosure_types
		SET status = 'archived', active_version_no = 0, updated_at = NOW()
		WHERE type_id = ? AND company_id IS NULL
	`, typeID)
	if err != nil {
		return fmt.Errorf("archive global template: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "global template not found", nil)
	}
	return nil
}

// ─── Display Group CRUD ────────────────────────────────────────────────────────

func (r *Repository) CreateDisplayGroup(ctx context.Context, req disclosureapp.CmsDisplayGroupCreateRequest) (*disclosureapp.DisplayGroupDTO, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO disclosure_display_groups (display_group_code, name_vi, name_en, description, icon, display_order, is_active, is_system)
		VALUES (?, ?, ?, ?, ?, ?, 1, 0)
	`, req.Code, req.NameVI, req.NameEN, req.Description, req.Icon, req.DisplayOrder)
	if err != nil {
		return nil, fmt.Errorf("create display group: %w", err)
	}
	return r.getDisplayGroupByCode(ctx, req.Code)
}

func (r *Repository) UpdateDisplayGroup(ctx context.Context, req disclosureapp.CmsDisplayGroupUpdateRequest) (*disclosureapp.DisplayGroupDTO, error) {
	isActive := 1
	if req.IsActive != nil && !*req.IsActive {
		isActive = 0
	} else if req.IsActive != nil && *req.IsActive {
		isActive = 1
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE disclosure_display_groups
		SET name_vi = ?, name_en = ?, description = ?, icon = ?, display_order = ?, is_active = ?
		WHERE display_group_code = ?
	`, req.NameVI, req.NameEN, req.Description, req.Icon, req.DisplayOrder, isActive, req.Code)
	if err != nil {
		return nil, fmt.Errorf("update display group: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "display group not found", nil)
	}
	return r.getDisplayGroupByCode(ctx, req.Code)
}

func (r *Repository) DeleteDisplayGroup(ctx context.Context, code string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM disclosure_display_groups WHERE display_group_code = ? AND is_system = 0`, code)
	if err != nil {
		return fmt.Errorf("delete display group: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "display group not found or is system-protected", nil)
	}
	return nil
}

func (r *Repository) getDisplayGroupByCode(ctx context.Context, code string) (*disclosureapp.DisplayGroupDTO, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT display_group_code, COALESCE(name_vi,''), COALESCE(name_en,''), COALESCE(description,''), COALESCE(icon,''), display_order, is_active, is_system
		FROM disclosure_display_groups WHERE display_group_code = ?
	`, code)
	var d disclosureapp.DisplayGroupDTO
	var isActive, isSystem int
	if err := row.Scan(&d.DisplayGroupCode, &d.NameVI, &d.NameEN, &d.Description, &d.Icon, &d.DisplayOrder, &isActive, &isSystem); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "display group not found", nil)
		}
		return nil, fmt.Errorf("get display group: %w", err)
	}
	d.IsActive = isActive == 1
	d.IsSystem = isSystem == 1
	return &d, nil
}

// ─── Template default department catalog ───────────────────────────────────────

func (r *Repository) ListTemplateDepartments(ctx context.Context) ([]disclosureapp.TemplateDepartmentDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT department_code, COALESCE(department_name,''), COALESCE(description,''), display_order, is_system
		FROM workflow_template_departments
		ORDER BY display_order ASC, department_name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list template departments: %w", err)
	}
	defer rows.Close()
	out := make([]disclosureapp.TemplateDepartmentDTO, 0)
	for rows.Next() {
		var d disclosureapp.TemplateDepartmentDTO
		var isSystem int
		if err := rows.Scan(&d.DepartmentCode, &d.DepartmentName, &d.Description, &d.DisplayOrder, &isSystem); err != nil {
			return nil, fmt.Errorf("scan template department: %w", err)
		}
		d.IsSystem = isSystem == 1
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repository) CreateTemplateDepartment(ctx context.Context, req disclosureapp.CmsTemplateDepartmentCreateRequest) (*disclosureapp.TemplateDepartmentDTO, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO workflow_template_departments (department_code, department_name, description, display_order, is_system)
		VALUES (?, ?, ?, ?, 0)
	`, req.Code, req.Name, req.Description, req.DisplayOrder)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "department code already exists", nil)
		}
		return nil, fmt.Errorf("create template department: %w", err)
	}
	return r.getTemplateDepartmentByCode(ctx, req.Code)
}

func (r *Repository) getTemplateDepartmentByCode(ctx context.Context, code string) (*disclosureapp.TemplateDepartmentDTO, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT department_code, COALESCE(department_name,''), COALESCE(description,''), display_order, is_system
		FROM workflow_template_departments WHERE department_code = ?
	`, code)
	var d disclosureapp.TemplateDepartmentDTO
	var isSystem int
	if err := row.Scan(&d.DepartmentCode, &d.DepartmentName, &d.Description, &d.DisplayOrder, &isSystem); err != nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "template department not found", nil)
	}
	d.IsSystem = isSystem == 1
	return &d, nil
}
