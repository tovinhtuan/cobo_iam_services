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

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, rec disclosureapp.RecordDTO) (*disclosureapp.RecordDTO, error) {
	attachmentsJSON, err := json.Marshal(rec.Attachments)
	if err != nil {
		return nil, fmt.Errorf("marshal attachments: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO disclosure_records (
			record_id, company_id, type_id, department_id, title, summary, content, planned_date, published_date, status, attachments_json, evidence_link, created_by, updated_by
		) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, CAST(? AS JSON), NULLIF(?, ''), ?, ?)
	`, rec.RecordID, rec.CompanyID, rec.TypeID, rec.DepartmentID, rec.Title, rec.Summary, rec.Content, rec.PlannedDate, rec.PublishedDate, rec.Status, string(attachmentsJSON), rec.EvidenceLink, rec.CreatedBy, rec.UpdatedBy)
	if err != nil {
		return nil, fmt.Errorf("disclosure insert: %w", err)
	}
	return r.FindByID(ctx, rec.CompanyID, rec.RecordID)
}

func (r *Repository) Update(ctx context.Context, rec disclosureapp.RecordDTO) (*disclosureapp.RecordDTO, error) {
	attachmentsJSON, err := json.Marshal(rec.Attachments)
	if err != nil {
		return nil, fmt.Errorf("marshal attachments: %w", err)
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE disclosure_records
		SET type_id = NULLIF(?, ''), department_id = ?, title = ?, summary = ?, content = ?, planned_date = NULLIF(?, ''), published_date = NULLIF(?, ''), status = ?, attachments_json = CAST(? AS JSON), evidence_link = NULLIF(?, ''), updated_by = ?
		WHERE record_id = ? AND company_id = ?
	`, rec.TypeID, rec.DepartmentID, rec.Title, rec.Summary, rec.Content, rec.PlannedDate, rec.PublishedDate, rec.Status, string(attachmentsJSON), rec.EvidenceLink, rec.UpdatedBy, rec.RecordID, rec.CompanyID)
	if err != nil {
		return nil, fmt.Errorf("disclosure update: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, perr.NewHTTPError(404, perr.CodeInvalidRequest, "record not found", nil)
	}
	return r.FindByID(ctx, rec.CompanyID, rec.RecordID)
}

func (r *Repository) FindByID(ctx context.Context, companyID, recordID string) (*disclosureapp.RecordDTO, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT record_id, company_id, COALESCE(type_id, ''), department_id, title, COALESCE(summary, ''), content, COALESCE(DATE_FORMAT(planned_date, '%Y-%m-%d'), ''), COALESCE(DATE_FORMAT(published_date, '%Y-%m-%d'), ''), status, attachments_json, COALESCE(evidence_link, ''), created_by, updated_by, created_at, updated_at
		FROM disclosure_records WHERE company_id = ? AND record_id = ?
	`, companyID, recordID)
	var rec disclosureapp.RecordDTO
	var attachmentsRaw []byte
	if err := row.Scan(&rec.RecordID, &rec.CompanyID, &rec.TypeID, &rec.DepartmentID, &rec.Title, &rec.Summary, &rec.Content, &rec.PlannedDate, &rec.PublishedDate, &rec.Status, &attachmentsRaw, &rec.EvidenceLink, &rec.CreatedBy, &rec.UpdatedBy, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(404, perr.CodeInvalidRequest, "record not found", nil)
		}
		return nil, err
	}
	if err := decodeAttachments(attachmentsRaw, &rec.Attachments); err != nil {
		return nil, fmt.Errorf("decode attachments: %w", err)
	}
	return &rec, nil
}

func (r *Repository) List(ctx context.Context, companyID string) ([]disclosureapp.RecordDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT record_id, company_id, COALESCE(type_id, ''), department_id, title, COALESCE(summary, ''), content, COALESCE(DATE_FORMAT(planned_date, '%Y-%m-%d'), ''), COALESCE(DATE_FORMAT(published_date, '%Y-%m-%d'), ''), status, attachments_json, COALESCE(evidence_link, ''), created_by, updated_by, created_at, updated_at
		FROM disclosure_records WHERE company_id = ? ORDER BY created_at DESC
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []disclosureapp.RecordDTO
	for rows.Next() {
		var rec disclosureapp.RecordDTO
		var attachmentsRaw []byte
		if err := rows.Scan(&rec.RecordID, &rec.CompanyID, &rec.TypeID, &rec.DepartmentID, &rec.Title, &rec.Summary, &rec.Content, &rec.PlannedDate, &rec.PublishedDate, &rec.Status, &attachmentsRaw, &rec.EvidenceLink, &rec.CreatedBy, &rec.UpdatedBy, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		if err := decodeAttachments(attachmentsRaw, &rec.Attachments); err != nil {
			return nil, fmt.Errorf("decode attachments: %w", err)
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func decodeAttachments(raw []byte, target *[]disclosureapp.AttachmentDTO) error {
	if len(raw) == 0 {
		*target = []disclosureapp.AttachmentDTO{}
		return nil
	}
	var parsed []disclosureapp.AttachmentDTO
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return err
	}
	*target = parsed
	return nil
}

func (r *Repository) ListTypeGroups(ctx context.Context, _ string) ([]disclosureapp.DisclosureGroupDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT group_id, name, description, icon, display_order
		FROM disclosure_type_groups
		ORDER BY display_order ASC, group_id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]disclosureapp.DisclosureGroupDTO, 0)
	for rows.Next() {
		var item disclosureapp.DisclosureGroupDTO
		if err := rows.Scan(&item.GroupID, &item.Name, &item.Description, &item.Icon, &item.DisplayOrder); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) ListTypes(ctx context.Context, companyID, groupID, query string) ([]disclosureapp.DisclosureTypeSummaryDTO, error) {
	args := []any{companyID}
	conditions := []string{"(t.company_id IS NULL OR t.company_id = ?)"}
	if strings.TrimSpace(groupID) != "" {
		conditions = append(conditions, "t.group_id = ?")
		args = append(args, strings.TrimSpace(groupID))
	}
	if strings.TrimSpace(query) != "" {
		conditions = append(conditions, "(LOWER(v.name) LIKE ? OR LOWER(v.description) LIKE ?)")
		like := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
		args = append(args, like, like)
	}
	sqlText := `
		SELECT t.type_id, t.group_id, v.name, v.category, v.template_category, v.description, v.deadline_rule, v.tags_json
		FROM disclosure_types t
		INNER JOIN disclosure_type_versions v
			ON v.type_id = t.type_id AND v.version_no = t.active_version_no
		WHERE ` + strings.Join(conditions, " AND ") + `
		ORDER BY t.type_id ASC
	`
	rows, err := r.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]disclosureapp.DisclosureTypeSummaryDTO, 0)
	for rows.Next() {
		var item disclosureapp.DisclosureTypeSummaryDTO
		var tagsRaw []byte
		if err := rows.Scan(&item.TypeID, &item.GroupID, &item.Name, &item.Category, &item.TemplateCategory, &item.Description, &item.DeadlineRule, &tagsRaw); err != nil {
			return nil, err
		}
		if err := decodeTags(tagsRaw, &item.Tags); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) GetTypeDetail(ctx context.Context, companyID, typeID string) (*disclosureapp.DisclosureTypeDTO, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			t.type_id, t.group_id, t.active_version_no,
			v.name, v.category, v.template_category, COALESCE(v.deadline_strategy, ''), v.description,
			COALESCE(v.legal_basis, ''), COALESCE(v.applicability, ''), COALESCE(v.implementation_content, ''), COALESCE(v.implementation_notes, ''),
			COALESCE(v.special_cases, ''), COALESCE(v.report_content, ''), COALESCE(v.required_docs, ''),
			COALESCE(v.deadline_rule, ''), COALESCE(v.periodicity, ''), COALESCE(v.channels_text, ''), COALESCE(v.beneficiaries, ''),
			COALESCE(v.receiving_authorities, ''), COALESCE(v.format, ''), COALESCE(v.legal_risks_text, ''), COALESCE(v.general_info, ''),
			COALESCE(v.reminder_milestones_json, JSON_ARRAY()),
			COALESCE(v.legal_bases_json, JSON_ARRAY()),
			COALESCE(v.checklist_json, JSON_ARRAY()),
			v.tags_json
		FROM disclosure_types t
		INNER JOIN disclosure_type_versions v ON v.type_id = t.type_id AND v.version_no = t.active_version_no
		WHERE t.type_id = ? AND (t.company_id IS NULL OR t.company_id = ?)
	`, typeID, companyID)
	var item disclosureapp.DisclosureTypeDTO
	var reminderMilestonesRaw []byte
	var legalBasesRaw []byte
	var checklistRaw []byte
	var tagsRaw []byte
	if err := row.Scan(
		&item.TypeID, &item.GroupID, &item.VersionNo,
		&item.Name, &item.Category, &item.TemplateCategory, &item.DeadlineStrategy, &item.Description,
		&item.LegalBasis, &item.Applicability, &item.ImplementationContent, &item.ImplementationNotes,
		&item.SpecialCases, &item.ReportContent, &item.RequiredDocs,
		&item.DeadlineRule, &item.Periodicity, &item.ChannelsText, &item.Beneficiaries,
		&item.ReceivingAuthorities, &item.Format, &item.LegalRisksText, &item.GeneralInfo,
		&reminderMilestonesRaw,
		&legalBasesRaw,
		&checklistRaw,
		&tagsRaw,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type not found", nil)
		}
		return nil, err
	}
	if err := decodeTags(tagsRaw, &item.Tags); err != nil {
		return nil, err
	}
	if err := decodeStringListJSON(reminderMilestonesRaw, &item.ReminderMilestones); err != nil {
		return nil, err
	}
	if err := decodeJSONList(legalBasesRaw, &item.LegalBases); err != nil {
		return nil, err
	}
	if err := decodeJSONList(checklistRaw, &item.Checklist); err != nil {
		return nil, err
	}
	blocks, err := r.listTemplateBlocks(ctx, typeID, item.VersionNo)
	if err != nil {
		return nil, err
	}
	item.Blocks = blocks
	return &item, nil
}

func (r *Repository) GetTypeVersionDetail(ctx context.Context, companyID, typeID string, versionNo int) (*disclosureapp.DisclosureTypeDTO, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			t.type_id, t.group_id, v.version_no,
			v.name, v.category, v.template_category, COALESCE(v.deadline_strategy, ''), v.description,
			COALESCE(v.legal_basis, ''), COALESCE(v.applicability, ''), COALESCE(v.implementation_content, ''), COALESCE(v.implementation_notes, ''),
			COALESCE(v.special_cases, ''), COALESCE(v.report_content, ''), COALESCE(v.required_docs, ''),
			COALESCE(v.deadline_rule, ''), COALESCE(v.periodicity, ''), COALESCE(v.channels_text, ''), COALESCE(v.beneficiaries, ''),
			COALESCE(v.receiving_authorities, ''), COALESCE(v.format, ''), COALESCE(v.legal_risks_text, ''), COALESCE(v.general_info, ''),
			COALESCE(v.reminder_milestones_json, JSON_ARRAY()),
			COALESCE(v.legal_bases_json, JSON_ARRAY()),
			COALESCE(v.checklist_json, JSON_ARRAY()),
			v.tags_json
		FROM disclosure_type_versions v
		INNER JOIN disclosure_types t ON t.type_id = v.type_id
		WHERE v.type_id = ? AND v.version_no = ? AND (t.company_id IS NULL OR t.company_id = ?)
	`, typeID, versionNo, companyID)
	var item disclosureapp.DisclosureTypeDTO
	var reminderMilestonesRaw []byte
	var legalBasesRaw []byte
	var checklistRaw []byte
	var tagsRaw []byte
	if err := row.Scan(
		&item.TypeID, &item.GroupID, &item.VersionNo,
		&item.Name, &item.Category, &item.TemplateCategory, &item.DeadlineStrategy, &item.Description,
		&item.LegalBasis, &item.Applicability, &item.ImplementationContent, &item.ImplementationNotes,
		&item.SpecialCases, &item.ReportContent, &item.RequiredDocs,
		&item.DeadlineRule, &item.Periodicity, &item.ChannelsText, &item.Beneficiaries,
		&item.ReceivingAuthorities, &item.Format, &item.LegalRisksText, &item.GeneralInfo,
		&reminderMilestonesRaw,
		&legalBasesRaw,
		&checklistRaw,
		&tagsRaw,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type version not found", nil)
		}
		return nil, err
	}
	if err := decodeTags(tagsRaw, &item.Tags); err != nil {
		return nil, err
	}
	if err := decodeStringListJSON(reminderMilestonesRaw, &item.ReminderMilestones); err != nil {
		return nil, err
	}
	if err := decodeJSONList(legalBasesRaw, &item.LegalBases); err != nil {
		return nil, err
	}
	if err := decodeJSONList(checklistRaw, &item.Checklist); err != nil {
		return nil, err
	}
	blocks, err := r.listTemplateBlocks(ctx, typeID, versionNo)
	if err != nil {
		return nil, err
	}
	item.Blocks = blocks
	return &item, nil
}

func (r *Repository) UpsertTypeVersion(ctx context.Context, req disclosureapp.UpsertTypeVersionRequest) (*disclosureapp.UpsertTypeVersionResponse, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	companyID := strings.TrimSpace(req.Subject.CompanyID)
	groupID := strings.TrimSpace(req.GroupID)
	changeNote := strings.TrimSpace(req.ChangeNote)
	var currentVersion sql.NullInt64
	var currentCompany sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT active_version_no, company_id FROM disclosure_types WHERE type_id = ? FOR UPDATE`, req.TypeID).Scan(&currentVersion, &currentCompany); err != nil {
		if err != sql.ErrNoRows {
			return nil, err
		}
	}
	if currentCompany.Valid && currentCompany.String != "" && currentCompany.String != companyID {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "cannot modify type from another company", nil)
	}
	if !currentCompany.Valid {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO disclosure_types (type_id, company_id, group_id, active_version_no, status)
			VALUES (?, ?, ?, 0, 'active')
		`, req.TypeID, companyID, groupID); err != nil {
			return nil, err
		}
	}
	nextVersion := 1
	if currentVersion.Valid && currentVersion.Int64 > 0 {
		nextVersion = int(currentVersion.Int64) + 1
	}
	tagsJSON, err := json.Marshal(req.Tags)
	if err != nil {
		return nil, fmt.Errorf("marshal tags: %w", err)
	}
	reminderMilestonesJSON, err := json.Marshal(req.ReminderMilestones)
	if err != nil {
		return nil, fmt.Errorf("marshal reminder milestones: %w", err)
	}
	legalBasesJSON, err := json.Marshal(req.LegalBases)
	if err != nil {
		return nil, fmt.Errorf("marshal legal bases: %w", err)
	}
	checklistJSON, err := json.Marshal(req.Checklist)
	if err != nil {
		return nil, fmt.Errorf("marshal checklist: %w", err)
	}
	blocks, err := normalizeTemplateBlocks(req.Blocks)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO disclosure_type_versions (
			type_id, version_no, name, category, template_category, deadline_strategy, description, legal_basis, applicability, implementation_content,
			implementation_notes, special_cases, report_content, required_docs, deadline_rule, periodicity, channels_text,
			beneficiaries, receiving_authorities, format, legal_risks_text, general_info, reminder_milestones_json, legal_bases_json, checklist_json, tags_json, change_note, updated_by, activated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''),
		          NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''),
		          NULLIF(?, ''), NULLIF(?, ''), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), NULLIF(?, ''), ?, ?)
	`, req.TypeID, nextVersion, req.Name, req.Category, req.TemplateCategory, req.DeadlineStrategy, req.Description,
		req.LegalBasis, req.Applicability, req.ImplementationContent, req.ImplementationNotes, req.SpecialCases, req.ReportContent,
		req.RequiredDocs, req.DeadlineRule, req.Periodicity, req.ChannelsText, req.Beneficiaries, req.ReceivingAuthorities,
		req.Format, req.LegalRisksText, req.GeneralInfo, string(reminderMilestonesJSON), string(legalBasesJSON), string(checklistJSON), string(tagsJSON), changeNote, req.Subject.UserID, now)
	if err != nil {
		return nil, err
	}
	for _, block := range blocks {
		configJSON, err := json.Marshal(block.Config)
		if err != nil {
			return nil, fmt.Errorf("marshal block config: %w", err)
		}
		validationJSON, err := json.Marshal(block.Validation)
		if err != nil {
			return nil, fmt.Errorf("marshal block validation: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO disclosure_template_blocks (
				type_id, version_no, block_id, block_key, block_type, title, description, config_json, validation_json, display_order, enabled
			) VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''), CAST(? AS JSON), CAST(? AS JSON), ?, ?)
		`, req.TypeID, nextVersion, block.BlockID, block.BlockKey, block.BlockType, block.Title, block.Description, string(configJSON), string(validationJSON), block.DisplayOrder, block.Enabled); err != nil {
			return nil, err
		}
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE disclosure_types
		SET group_id = ?, active_version_no = ?, status = 'active', updated_at = CURRENT_TIMESTAMP
		WHERE type_id = ?
	`, groupID, nextVersion, req.TypeID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &disclosureapp.UpsertTypeVersionResponse{
		TypeID:      req.TypeID,
		VersionNo:   nextVersion,
		IsActive:    true,
		UpdatedBy:   req.Subject.UserID,
		ActivatedAt: now,
	}, nil
}

func (r *Repository) ListTypeVersions(ctx context.Context, companyID, typeID string) ([]disclosureapp.DisclosureTypeVersionDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT v.type_id, v.version_no, (v.version_no = t.active_version_no) AS is_active, COALESCE(v.change_note, ''), v.updated_by, v.activated_at
		FROM disclosure_type_versions v
		INNER JOIN disclosure_types t ON t.type_id = v.type_id
		WHERE v.type_id = ? AND (t.company_id IS NULL OR t.company_id = ?)
		ORDER BY v.version_no DESC
	`, typeID, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]disclosureapp.DisclosureTypeVersionDTO, 0)
	for rows.Next() {
		var item disclosureapp.DisclosureTypeVersionDTO
		if err := rows.Scan(&item.TypeID, &item.VersionNo, &item.IsActive, &item.ChangeNote, &item.UpdatedBy, &item.ActivatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type not found", nil)
	}
	return out, rows.Err()
}

func (r *Repository) ActivateTypeVersion(ctx context.Context, req disclosureapp.ActivateTypeVersionRequest) (*disclosureapp.ActivateTypeVersionResponse, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var currentCompany sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT company_id FROM disclosure_types WHERE type_id = ? FOR UPDATE`, req.TypeID).Scan(&currentCompany); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type not found", nil)
		}
		return nil, err
	}
	if currentCompany.Valid && currentCompany.String != "" && currentCompany.String != req.Subject.CompanyID {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "cannot modify type from another company", nil)
	}

	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM disclosure_type_versions WHERE type_id = ? AND version_no = ?`, req.TypeID, req.VersionNo).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type version not found", nil)
	}

	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, `
		UPDATE disclosure_type_versions
		SET updated_by = CASE WHEN version_no = ? THEN ? ELSE updated_by END,
		    activated_at = CASE WHEN version_no = ? THEN ? ELSE activated_at END
		WHERE type_id = ?
	`, req.VersionNo, req.Subject.UserID, req.VersionNo, now, req.TypeID)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE disclosure_types
		SET active_version_no = ?, updated_at = CURRENT_TIMESTAMP
		WHERE type_id = ?
	`, req.VersionNo, req.TypeID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &disclosureapp.ActivateTypeVersionResponse{
		TypeID:      req.TypeID,
		VersionNo:   req.VersionNo,
		IsActive:    true,
		UpdatedBy:   req.Subject.UserID,
		ActivatedAt: now,
	}, nil
}

func decodeTags(raw []byte, target *[]string) error {
	if len(raw) == 0 {
		*target = []string{}
		return nil
	}
	var parsed []string
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return err
	}
	*target = parsed
	return nil
}

func decodeStringListJSON(raw []byte, target *[]string) error {
	if len(raw) == 0 {
		*target = []string{}
		return nil
	}
	var parsed []string
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return err
	}
	if parsed == nil {
		parsed = []string{}
	}
	*target = parsed
	return nil
}

func decodeJSONList[T any](raw []byte, target *[]T) error {
	if len(raw) == 0 {
		*target = []T{}
		return nil
	}
	var parsed []T
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return err
	}
	if parsed == nil {
		parsed = []T{}
	}
	*target = parsed
	return nil
}

func normalizeTemplateBlocks(blocks []disclosureapp.TemplateBlockDTO) ([]disclosureapp.TemplateBlockDTO, error) {
	if len(blocks) == 0 {
		return []disclosureapp.TemplateBlockDTO{}, nil
	}
	out := make([]disclosureapp.TemplateBlockDTO, 0, len(blocks))
	for _, block := range blocks {
		if block.Config == nil {
			block.Config = map[string]any{}
		}
		if block.Validation == nil {
			block.Validation = map[string]any{}
		}
		out = append(out, block)
	}
	return out, nil
}

func (r *Repository) listTemplateBlocks(ctx context.Context, typeID string, versionNo int) ([]disclosureapp.TemplateBlockDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT block_id, block_key, block_type, title, COALESCE(description, ''), config_json, validation_json, display_order, enabled
		FROM disclosure_template_blocks
		WHERE type_id = ? AND version_no = ?
		ORDER BY display_order ASC, block_id ASC
	`, typeID, versionNo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]disclosureapp.TemplateBlockDTO, 0)
	for rows.Next() {
		var item disclosureapp.TemplateBlockDTO
		var configRaw []byte
		var validationRaw []byte
		if err := rows.Scan(
			&item.BlockID,
			&item.BlockKey,
			&item.BlockType,
			&item.Title,
			&item.Description,
			&configRaw,
			&validationRaw,
			&item.DisplayOrder,
			&item.Enabled,
		); err != nil {
			return nil, err
		}
		if err := decodeJSONMap(configRaw, &item.Config); err != nil {
			return nil, fmt.Errorf("decode block config: %w", err)
		}
		if err := decodeJSONMap(validationRaw, &item.Validation); err != nil {
			return nil, fmt.Errorf("decode block validation: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func decodeJSONMap(raw []byte, target *map[string]any) error {
	if len(raw) == 0 {
		*target = map[string]any{}
		return nil
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return err
	}
	if parsed == nil {
		parsed = map[string]any{}
	}
	*target = parsed
	return nil
}
