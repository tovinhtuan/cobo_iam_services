package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	sqldriver "github.com/go-sql-driver/mysql"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// isDuplicateRecordIDError reports whether err is a MySQL duplicate-key error
// (1062 / ER_DUP_ENTRY) on disclosure_records' primary key (record_id is the
// table's only unique constraint — see migration 0004_p1_business_tables).
// Scoped to the key name so unrelated duplicate-key errors are not swallowed.
func isDuplicateRecordIDError(err error) bool {
	if err == nil {
		return false
	}
	var me *sqldriver.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		msg := strings.ToLower(me.Message)
		return strings.Contains(msg, "primary") || strings.Contains(msg, "record_id")
	}
	return false
}

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
		if isDuplicateRecordIDError(err) {
			return nil, disclosureapp.ErrDuplicateRecordID
		}
		return nil, fmt.Errorf("disclosure insert: %w", err)
	}
	return r.FindByID(ctx, rec.CompanyID, rec.RecordID)
}

func (r *Repository) Update(ctx context.Context, rec disclosureapp.RecordDTO) (*disclosureapp.RecordDTO, error) {
	attachmentsJSON, err := json.Marshal(rec.Attachments)
	if err != nil {
		return nil, fmt.Errorf("marshal attachments: %w", err)
	}
	var completedAt any
	if rec.CompletedAt != nil {
		completedAt = rec.CompletedAt.UTC()
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE disclosure_records
		SET type_id = NULLIF(?, ''), department_id = ?, title = ?, summary = ?, content = ?, planned_date = NULLIF(?, ''), published_date = NULLIF(?, ''), status = ?, attachments_json = CAST(? AS JSON), evidence_link = NULLIF(?, ''), updated_by = ?,
			completed_at = COALESCE(completed_at, ?),
			completed_source = COALESCE(completed_source, NULLIF(?, ''))
		WHERE record_id = ? AND company_id = ?
	`, rec.TypeID, rec.DepartmentID, rec.Title, rec.Summary, rec.Content, rec.PlannedDate, rec.PublishedDate, rec.Status, string(attachmentsJSON), rec.EvidenceLink, rec.UpdatedBy, completedAt, rec.CompletedSource, rec.RecordID, rec.CompanyID)
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
		SELECT dr.record_id, dr.company_id, COALESCE(dr.type_id, ''), dr.department_id, dr.title, COALESCE(dr.summary, ''), dr.content,
			COALESCE(DATE_FORMAT(dr.planned_date, '%Y-%m-%d'), ''), COALESCE(DATE_FORMAT(dr.published_date, '%Y-%m-%d'), ''), dr.status,
			dr.attachments_json, COALESCE(dr.evidence_link, ''),
			COALESCE((
				SELECT wi.workflow_instance_id
				FROM workflow_instances wi
				WHERE wi.company_id = dr.company_id AND wi.record_id = dr.record_id
				ORDER BY wi.workflow_instance_id ASC
				LIMIT 1
			), ''),
			dr.created_by, dr.updated_by, dr.created_at, dr.updated_at,
			dr.completed_at, COALESCE(dr.completed_source, '')
		FROM disclosure_records dr
		WHERE dr.company_id = ? AND dr.record_id = ?
	`, companyID, recordID)
	var rec disclosureapp.RecordDTO
	var attachmentsRaw []byte
	var completedAt sql.NullTime
	if err := row.Scan(&rec.RecordID, &rec.CompanyID, &rec.TypeID, &rec.DepartmentID, &rec.Title, &rec.Summary, &rec.Content, &rec.PlannedDate, &rec.PublishedDate, &rec.Status, &attachmentsRaw, &rec.EvidenceLink, &rec.WorkflowInstanceID, &rec.CreatedBy, &rec.UpdatedBy, &rec.CreatedAt, &rec.UpdatedAt, &completedAt, &rec.CompletedSource); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(404, perr.CodeInvalidRequest, "record not found", nil)
		}
		return nil, err
	}
	if completedAt.Valid {
		t := completedAt.Time.UTC()
		rec.CompletedAt = &t
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

func (r *Repository) ListDisplayGroups(ctx context.Context) ([]disclosureapp.DisplayGroupDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT display_group_code, name_vi, name_en, description, icon, display_order, is_active, is_system
		FROM disclosure_display_groups
		WHERE is_active = 1
		ORDER BY display_order ASC, display_group_code ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]disclosureapp.DisplayGroupDTO, 0)
	for rows.Next() {
		var item disclosureapp.DisplayGroupDTO
		if err := rows.Scan(&item.DisplayGroupCode, &item.NameVI, &item.NameEN, &item.Description, &item.Icon, &item.DisplayOrder, &item.IsActive, &item.IsSystem); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
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

func (r *Repository) ListTypes(ctx context.Context, params disclosureapp.ListTypesParams) ([]disclosureapp.DisclosureTypeSummaryDTO, int, error) {
	companyID := params.CompanyID
	groupID := strings.TrimSpace(params.GroupID)
	displayGroupCode := strings.TrimSpace(params.DisplayGroupCode)
	query := strings.TrimSpace(params.Query)

	page := params.Page
	pageSize := params.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	sortCol := map[string]string{"name": "v.name", "created_at": "t.created_at"}[params.SortBy]
	if sortCol == "" {
		sortCol = "t.created_at"
	}
	sortDir := "DESC"
	if strings.ToUpper(params.SortDir) == "ASC" {
		sortDir = "ASC"
	}

	joins := ""
	args := []any{companyID}
	conditions := []string{"(t.company_id IS NULL OR t.company_id = ?)"}

	if groupID != "" {
		conditions = append(conditions, "t.group_id = ?")
		args = append(args, groupID)
	}
	// Display group filter: use junction table when provided (new many-to-many model).
	if displayGroupCode != "" {
		joins = " INNER JOIN template_display_groups tdg ON tdg.template_id = t.type_id AND tdg.display_group_code = ?"
		args = append([]any{displayGroupCode}, args...)
		// Re-order args: displayGroupCode goes to the JOIN placeholder, rest follow WHERE.
		// Simpler: rebuild args in WHERE order after JOIN.
		args = []any{displayGroupCode, companyID}
		if groupID != "" {
			conditions = append(conditions[1:], "t.group_id = ?")
			args = append(args, groupID)
			conditions = append([]string{"(t.company_id IS NULL OR t.company_id = ?)"}, conditions[1:]...)
		}
	}
	if query != "" {
		conditions = append(conditions, "(LOWER(v.name) LIKE ? OR LOWER(v.description) LIKE ?)")
		like := "%" + strings.ToLower(query) + "%"
		args = append(args, like, like)
	}
	if len(params.Tags) > 0 {
		tagConds := make([]string, 0, len(params.Tags))
		for _, tag := range params.Tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			// tags_json is a JSON array of strings; match any selected tag (OR).
			tagConds = append(tagConds, "JSON_CONTAINS(COALESCE(v.tags_json, JSON_ARRAY()), JSON_QUOTE(?), '$')")
			args = append(args, tag)
		}
		if len(tagConds) > 0 {
			conditions = append(conditions, "("+strings.Join(tagConds, " OR ")+")")
		}
	}
	if freq := disclosureapp.NormalizePeriodicityFilter(params.Periodicity); freq != "" {
		switch freq {
		case "ad_hoc":
			conditions = append(conditions,
				"(LOWER(COALESCE(v.periodicity, '')) IN ('ad_hoc', 'event_based') OR LOWER(COALESCE(v.template_category, '')) IN ('irregular', 'custom'))")
		case "yearly":
			conditions = append(conditions, "LOWER(COALESCE(v.periodicity, '')) IN ('yearly', 'annual')")
		default:
			conditions = append(conditions, "LOWER(COALESCE(v.periodicity, '')) = ?")
			args = append(args, freq)
		}
	}
	if deptID := strings.TrimSpace(params.DepartmentID); deptID != "" {
		conditions = append(conditions, `EXISTS (
			SELECT 1 FROM global_workflows gw
			INNER JOIN global_workflow_steps gws ON gws.workflow_id = gw.workflow_id
			WHERE gw.type_id = t.type_id
			  AND COALESCE(gw.is_active, 0) = 1
			  AND gws.department_id = ?
		)`)
		args = append(args, deptID)
	}
	if len(params.TypeIDs) > 0 {
		placeholders := make([]string, len(params.TypeIDs))
		for i, id := range params.TypeIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		conditions = append(conditions, "t.type_id IN ("+strings.Join(placeholders, ",")+")")
	}

	whereClause := strings.Join(conditions, " AND ")
	baseSQL := `FROM disclosure_types t` + joins + `
		INNER JOIN disclosure_type_versions v
			ON v.type_id = t.type_id AND v.version_no = t.active_version_no
		WHERE ` + whereClause

	total := 0
	if len(params.TypeIDs) == 0 {
		countSQL := `SELECT COUNT(*) ` + baseSQL
		if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
			return nil, 0, err
		}
	}

	var dataSQL string
	if params.LightweightOnly {
		dataSQL = `SELECT t.type_id, t.company_id, v.name, t.created_at, v.applicability_rules_json
		` + baseSQL + `
		ORDER BY ` + sortCol + ` ` + sortDir
	} else {
		dataSQL = `SELECT t.type_id, t.group_id, COALESCE(t.display_group_code, ''), t.company_id,
		       COALESCE(t.is_mandatory, 0), COALESCE(t.review_status, ''),
		       v.name, v.category, v.template_category, COALESCE(LEFT(v.description, 1024), ''), v.deadline_rule, v.tags_json,
		       COALESCE(v.periodicity, ''), v.applicability_rules_json, t.created_at
		` + baseSQL + `
		ORDER BY ` + sortCol + ` ` + sortDir
	}
	dataArgs := args
	if page > 0 {
		offset := (page - 1) * pageSize
		dataSQL += " LIMIT ? OFFSET ?"
		dataArgs = append(dataArgs, pageSize, offset)
	}

	rows, err := r.db.QueryContext(ctx, dataSQL, dataArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]disclosureapp.DisclosureTypeSummaryDTO, 0)
	typeIDs := make([]string, 0)
	for rows.Next() {
		var item disclosureapp.DisclosureTypeSummaryDTO
		if params.LightweightOnly {
			var ownerCompanyID sql.NullString
			var rulesRaw []byte
			if err := rows.Scan(&item.TypeID, &ownerCompanyID, &item.Name, &item.CreatedAt, &rulesRaw); err != nil {
				return nil, 0, err
			}
			if rules, err := applicability.ParseRulesJSON(rulesRaw); err != nil {
				return nil, 0, err
			} else {
				item.ApplicabilityRules = rules
			}
			item.OwnerCompanyID = ownerCompanyID.String
			if ownerCompanyID.Valid && strings.TrimSpace(ownerCompanyID.String) != "" {
				item.Scope = "company"
			} else {
				item.Scope = "global"
			}
			out = append(out, item)
			continue
		}

		var tagsRaw []byte
		var rulesRaw []byte
		var ownerCompanyID sql.NullString
		if err := rows.Scan(
			&item.TypeID, &item.GroupID, &item.DisplayGroupCode, &ownerCompanyID,
			&item.IsMandatory, &item.ReviewStatus,
			&item.Name, &item.Category, &item.TemplateCategory, &item.Description, &item.DeadlineRule, &tagsRaw,
			&item.Periodicity, &rulesRaw, &item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		if rules, err := applicability.ParseRulesJSON(rulesRaw); err != nil {
			return nil, 0, err
		} else {
			item.ApplicabilityRules = rules
		}
		item.OwnerCompanyID = ownerCompanyID.String
		if ownerCompanyID.Valid && strings.TrimSpace(ownerCompanyID.String) != "" {
			item.Scope = "company"
		} else {
			item.Scope = "global"
		}
		if err := decodeTags(tagsRaw, &item.Tags); err != nil {
			return nil, 0, err
		}
		item.DisplayGroupCodes = []string{}
		typeIDs = append(typeIDs, item.TypeID)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if params.LightweightOnly || len(typeIDs) == 0 {
		return out, total, nil
	}
	// Batch-load display_group_codes from junction table (DBA-001 / many-to-many).
	if len(typeIDs) > 0 {
		groupsByType, err := r.batchLoadDisplayGroupCodes(ctx, typeIDs)
		if err != nil {
			return nil, 0, err
		}
		workflowFlags, err := r.batchLoadActiveWorkflowFlags(ctx, companyID, typeIDs)
		if err != nil {
			return nil, 0, err
		}
		for i := range out {
			if codes, ok := groupsByType[out[i].TypeID]; ok {
				out[i].DisplayGroupCodes = codes
			}
			out[i].HasWorkflow = workflowFlags[out[i].TypeID]
		}
	}
	return out, total, nil
}

func (r *Repository) ListTypeFilterOptions(ctx context.Context, companyID string) (*disclosureapp.ListTypeFilterOptionsResponse, error) {
	companyID = strings.TrimSpace(companyID)
	out := &disclosureapp.ListTypeFilterOptionsResponse{
		Tags:        []disclosureapp.TypeFilterOptionDTO{},
		Departments: []disclosureapp.TypeFilterOptionDTO{},
		Frequencies: disclosureapp.DefaultFrequencyFilterOptions(),
	}

	tagRows, err := r.db.QueryContext(ctx, `
		SELECT COALESCE(v.tags_json, JSON_ARRAY())
		FROM disclosure_types t
		INNER JOIN disclosure_type_versions v
			ON v.type_id = t.type_id AND v.version_no = t.active_version_no
		WHERE (t.company_id IS NULL OR t.company_id = ?)
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer tagRows.Close()
	tagSeen := map[string]struct{}{}
	for tagRows.Next() {
		var raw []byte
		if err := tagRows.Scan(&raw); err != nil {
			return nil, err
		}
		var tags []string
		if err := decodeTags(raw, &tags); err != nil {
			continue
		}
		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			key := strings.ToLower(tag)
			if _, ok := tagSeen[key]; ok {
				continue
			}
			tagSeen[key] = struct{}{}
			out.Tags = append(out.Tags, disclosureapp.TypeFilterOptionDTO{ID: tag, Name: tag})
		}
	}
	if err := tagRows.Err(); err != nil {
		return nil, err
	}

	deptRows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT gws.department_id,
		       COALESCE(NULLIF(d.department_name, ''), gws.department_id) AS department_name
		FROM global_workflows gw
		INNER JOIN global_workflow_steps gws ON gws.workflow_id = gw.workflow_id
		INNER JOIN disclosure_types t ON t.type_id = gw.type_id
		LEFT JOIN departments d
			ON d.department_id = gws.department_id
			AND d.company_id = ?
			AND d.status != 'inactive'
		WHERE (t.company_id IS NULL OR t.company_id = ?)
		  AND COALESCE(gw.is_active, 0) = 1
		  AND gws.department_id IS NOT NULL
		  AND TRIM(gws.department_id) <> ''
		ORDER BY department_name ASC
	`, companyID, companyID)
	if err != nil {
		// Workflow/department tables may be unavailable in some environments — keep empty departments.
		return out, nil
	}
	defer deptRows.Close()
	for deptRows.Next() {
		var id, name string
		if err := deptRows.Scan(&id, &name); err != nil {
			return nil, err
		}
		id = strings.TrimSpace(id)
		name = strings.TrimSpace(name)
		if id == "" {
			continue
		}
		if name == "" {
			name = "Chưa xác định"
		}
		out.Departments = append(out.Departments, disclosureapp.TypeFilterOptionDTO{ID: id, Name: name})
	}
	if err := deptRows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// batchLoadDisplayGroupCodes fetches display_group_codes for a set of type IDs from the
// template_display_groups junction table. Falls back gracefully if the table does not
// exist yet (pre-migration 0053 environments).
func (r *Repository) batchLoadDisplayGroupCodes(ctx context.Context, typeIDs []string) (map[string][]string, error) {
	if len(typeIDs) == 0 {
		return map[string][]string{}, nil
	}
	placeholders := make([]string, len(typeIDs))
	args := make([]any, len(typeIDs))
	for i, id := range typeIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	sqlText := `SELECT template_id, display_group_code FROM template_display_groups
	             WHERE template_id IN (` + strings.Join(placeholders, ",") + `)
	             ORDER BY template_id, display_order ASC`
	rows, err := r.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		// Table may not exist before migration 0053 — return empty map gracefully.
		return map[string][]string{}, nil
	}
	defer rows.Close()
	result := make(map[string][]string)
	for rows.Next() {
		var templateID, code string
		if err := rows.Scan(&templateID, &code); err != nil {
			return nil, err
		}
		result[templateID] = append(result[templateID], code)
	}
	return result, rows.Err()
}

// batchLoadActiveWorkflowFlags checks both CMS enterprise_workflow blocks (nhánh 1) and
// company-specific workflow overrides (nhánh 2). companyID scopes nhánh 2 so that
// Company B cannot inherit has_workflow=true from Company A's approved override.
func (r *Repository) batchLoadActiveWorkflowFlags(ctx context.Context, companyID string, typeIDs []string) (map[string]bool, error) {
	if len(typeIDs) == 0 {
		return map[string]bool{}, nil
	}
	placeholders := make([]string, len(typeIDs))
	args := make([]any, len(typeIDs))
	for i, id := range typeIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	ph := strings.Join(placeholders, ",")

	// Nhánh 1: CMS template — block must contain at least one actual step.
	rows, err := r.db.QueryContext(ctx, `
		SELECT b.type_id, b.config_json
		FROM disclosure_template_blocks b
		INNER JOIN disclosure_types t ON t.type_id = b.type_id AND t.active_version_no = b.version_no
		WHERE b.block_key = 'enterprise_workflow' AND b.type_id IN (`+ph+`)
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]bool, len(typeIDs))
	for rows.Next() {
		var typeID string
		var configRaw []byte
		if err := rows.Scan(&typeID, &configRaw); err != nil {
			return nil, err
		}
		var cfg map[string]any
		if err := decodeJSONMap(configRaw, &cfg); err != nil {
			return nil, fmt.Errorf("decode workflow config: %w", err)
		}
		result[typeID] = len(disclosureapp.ExtractTemplateWorkflow([]disclosureapp.TemplateBlockDTO{{
			BlockKey: "enterprise_workflow",
			Config:   cfg,
		}})) > 0
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Nhánh 2: Company-defined template — approved workflow-override for THIS company only.
	// active_version_no INT NOT NULL DEFAULT 0: draft/archived=0, approved=version_no>0.
	// company_id filter prevents Company B from inheriting Company A's approved override.
	oRows, err := r.db.QueryContext(ctx, `
		SELECT type_id
		FROM company_template_workflow_overrides
		WHERE active_version_no > 0
		  AND company_id = ?
		  AND type_id IN (`+ph+`)
	`, append([]any{companyID}, args...)...)
	if err != nil {
		return nil, err
	}
	defer oRows.Close()
	for oRows.Next() {
		var typeID string
		if err := oRows.Scan(&typeID); err != nil {
			return nil, err
		}
		result[typeID] = true
	}
	return result, oRows.Err()
}

func (r *Repository) HasActiveEnterpriseWorkflow(ctx context.Context, companyID, typeID string) (bool, error) {
	flags, err := r.batchLoadActiveWorkflowFlags(ctx, companyID, []string{typeID})
	if err != nil {
		return false, err
	}
	return flags[typeID], nil
}

func (r *Repository) replaceTemplateDisplayGroups(ctx context.Context, tx *sql.Tx, typeID string, codes []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM template_display_groups WHERE template_id = ?`, typeID); err != nil {
		return fmt.Errorf("delete template_display_groups: %w", err)
	}
	for i, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO template_display_groups (template_id, display_group_code, display_order)
			VALUES (?, ?, ?)
		`, typeID, code, i+1); err != nil {
			return fmt.Errorf("insert template_display_groups: %w", err)
		}
	}
	return nil
}

func (r *Repository) GetTypeDetail(ctx context.Context, companyID, typeID string) (*disclosureapp.DisclosureTypeDTO, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			t.type_id, t.group_id, COALESCE(t.display_group_code, ''), t.company_id, t.active_version_no,
			COALESCE(t.is_mandatory, 0), COALESCE(t.review_status, ''),
			v.name, v.category, v.template_category, COALESCE(v.deadline_strategy, ''), v.description,
			COALESCE(v.legal_basis, ''), COALESCE(v.applicability, ''), COALESCE(v.implementation_content, ''), COALESCE(v.implementation_notes, ''),
			COALESCE(v.special_cases, ''), COALESCE(v.report_content, ''), COALESCE(v.required_docs, ''),
			COALESCE(v.deadline_rule, ''), COALESCE(v.periodicity, ''), COALESCE(v.channels_text, ''), COALESCE(v.beneficiaries, ''),
			COALESCE(v.receiving_authorities, ''), COALESCE(v.format, ''), COALESCE(v.legal_risks_text, ''), COALESCE(v.general_info, ''),
			COALESCE(v.deadline_config_json, JSON_OBJECT()),
			COALESCE(v.legal_bases_json, JSON_ARRAY()),
			COALESCE(v.checklist_json, JSON_ARRAY()),
			v.tags_json,
			v.applicability_rules_json
		FROM disclosure_types t
		INNER JOIN disclosure_type_versions v ON v.type_id = t.type_id AND v.version_no = t.active_version_no
		WHERE t.type_id = ? AND (t.company_id IS NULL OR t.company_id = ?)
	`, typeID, companyID)
	var item disclosureapp.DisclosureTypeDTO
	var ownerCompanyID sql.NullString
	var deadlineConfigRaw []byte
	var legalBasesRaw []byte
	var checklistRaw []byte
	var tagsRaw []byte
	var rulesRaw []byte
	if err := row.Scan(
		&item.TypeID, &item.GroupID, &item.DisplayGroupCode, &ownerCompanyID, &item.VersionNo,
		&item.IsMandatory, &item.ReviewStatus,
		&item.Name, &item.Category, &item.TemplateCategory, &item.DeadlineStrategy, &item.Description,
		&item.LegalBasis, &item.Applicability, &item.ImplementationContent, &item.ImplementationNotes,
		&item.SpecialCases, &item.ReportContent, &item.RequiredDocs,
		&item.DeadlineRule, &item.Periodicity, &item.ChannelsText, &item.Beneficiaries,
		&item.ReceivingAuthorities, &item.Format, &item.LegalRisksText, &item.GeneralInfo,
		&deadlineConfigRaw,
		&legalBasesRaw,
		&checklistRaw,
		&tagsRaw,
		&rulesRaw,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type not found", nil)
		}
		return nil, err
	}
	if rules, err := applicability.ParseRulesJSON(rulesRaw); err != nil {
		return nil, err
	} else {
		item.ApplicabilityRules = rules
	}
	if err := decodeTags(tagsRaw, &item.Tags); err != nil {
		return nil, err
	}
	item.OwnerCompanyID = ownerCompanyID.String
	if strings.TrimSpace(item.OwnerCompanyID) == "" {
		item.Scope = "global"
	} else {
		item.Scope = "company"
	}
	if err := decodeDeadlineConfigJSON(deadlineConfigRaw, &item.DeadlineConfig); err != nil {
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
	// batchLoadActiveWorkflowFlags checks both CMS enterprise_workflow blocks and
	// approved company_template_workflow_overrides (nhánh 2 — active_version_no > 0).
	// companyID scopes nhánh 2 to this company's overrides only.
	if wfFlags, err2 := r.batchLoadActiveWorkflowFlags(ctx, companyID, []string{typeID}); err2 == nil {
		item.HasWorkflow = wfFlags[typeID]
	} else {
		item.HasWorkflow = disclosureapp.TemplateHasWorkflow(item.Blocks)
	}
	// Load display_group_codes from junction table (migration 0053).
	groupsByType, err := r.batchLoadDisplayGroupCodes(ctx, []string{typeID})
	if err != nil {
		return nil, err
	}
	if codes, ok := groupsByType[typeID]; ok {
		item.DisplayGroupCodes = codes
	} else {
		item.DisplayGroupCodes = []string{}
	}
	return &item, nil
}

func (r *Repository) GetTypeVersionDetail(ctx context.Context, companyID, typeID string, versionNo int) (*disclosureapp.DisclosureTypeDTO, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			t.type_id, t.group_id, t.company_id, v.version_no,
			v.name, v.category, v.template_category, COALESCE(v.deadline_strategy, ''), v.description,
			COALESCE(v.legal_basis, ''), COALESCE(v.applicability, ''), COALESCE(v.implementation_content, ''), COALESCE(v.implementation_notes, ''),
			COALESCE(v.special_cases, ''), COALESCE(v.report_content, ''), COALESCE(v.required_docs, ''),
			COALESCE(v.deadline_rule, ''), COALESCE(v.periodicity, ''), COALESCE(v.channels_text, ''), COALESCE(v.beneficiaries, ''),
			COALESCE(v.receiving_authorities, ''), COALESCE(v.format, ''), COALESCE(v.legal_risks_text, ''), COALESCE(v.general_info, ''),
			COALESCE(v.deadline_config_json, JSON_OBJECT()),
			COALESCE(v.legal_bases_json, JSON_ARRAY()),
			COALESCE(v.checklist_json, JSON_ARRAY()),
			v.tags_json,
			v.applicability_rules_json
		FROM disclosure_type_versions v
		INNER JOIN disclosure_types t ON t.type_id = v.type_id
		WHERE v.type_id = ? AND v.version_no = ? AND (t.company_id IS NULL OR t.company_id = ?)
	`, typeID, versionNo, companyID)
	var item disclosureapp.DisclosureTypeDTO
	var ownerCompanyID sql.NullString
	var deadlineConfigRaw []byte
	var legalBasesRaw []byte
	var checklistRaw []byte
	var tagsRaw []byte
	var rulesRaw []byte
	if err := row.Scan(
		&item.TypeID, &item.GroupID, &ownerCompanyID, &item.VersionNo,
		&item.Name, &item.Category, &item.TemplateCategory, &item.DeadlineStrategy, &item.Description,
		&item.LegalBasis, &item.Applicability, &item.ImplementationContent, &item.ImplementationNotes,
		&item.SpecialCases, &item.ReportContent, &item.RequiredDocs,
		&item.DeadlineRule, &item.Periodicity, &item.ChannelsText, &item.Beneficiaries,
		&item.ReceivingAuthorities, &item.Format, &item.LegalRisksText, &item.GeneralInfo,
		&deadlineConfigRaw,
		&legalBasesRaw,
		&checklistRaw,
		&tagsRaw,
		&rulesRaw,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type version not found", nil)
		}
		return nil, err
	}
	if rules, err := applicability.ParseRulesJSON(rulesRaw); err != nil {
		return nil, err
	} else {
		item.ApplicabilityRules = rules
	}
	if err := decodeTags(tagsRaw, &item.Tags); err != nil {
		return nil, err
	}
	item.OwnerCompanyID = ownerCompanyID.String
	if strings.TrimSpace(item.OwnerCompanyID) == "" {
		item.Scope = "global"
	} else {
		item.Scope = "company"
	}
	if err := decodeDeadlineConfigJSON(deadlineConfigRaw, &item.DeadlineConfig); err != nil {
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
	if wfFlags, err2 := r.batchLoadActiveWorkflowFlags(ctx, companyID, []string{typeID}); err2 == nil {
		item.HasWorkflow = wfFlags[typeID]
	} else {
		item.HasWorkflow = disclosureapp.TemplateHasWorkflow(item.Blocks)
	}
	// Load display_group_codes from junction table (same as GetTypeDetail).
	groupsByType, err := r.batchLoadDisplayGroupCodes(ctx, []string{typeID})
	if err != nil {
		return nil, err
	}
	if codes, ok := groupsByType[typeID]; ok {
		item.DisplayGroupCodes = codes
	} else {
		item.DisplayGroupCodes = []string{}
	}
	return &item, nil
}

func (r *Repository) UpsertTypeVersion(ctx context.Context, req disclosureapp.UpsertTypeVersionRequest) (*disclosureapp.UpsertTypeVersionResponse, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	companyID := strings.TrimSpace(req.Subject.CompanyID)
	requestedScope := strings.ToLower(strings.TrimSpace(req.Scope))
	if requestedScope == "" {
		requestedScope = "global"
	}
	groupID := strings.TrimSpace(req.GroupID)
	changeNote := strings.TrimSpace(req.ChangeNote)
	var currentVersion sql.NullInt64
	var currentCompany sql.NullString
	typeExists := false
	if err := tx.QueryRowContext(ctx, `SELECT active_version_no, company_id FROM disclosure_types WHERE type_id = ? FOR UPDATE`, req.TypeID).Scan(&currentVersion, &currentCompany); err != nil {
		if err != sql.ErrNoRows {
			return nil, err
		}
	} else {
		typeExists = true
	}
	if typeExists && currentCompany.Valid && currentCompany.String != "" && currentCompany.String != companyID {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "cannot modify type from another company", nil)
	}
	if typeExists && !currentCompany.Valid && requestedScope != "global" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "cannot change existing global type to company scope", nil)
	}
	if typeExists && currentCompany.Valid && currentCompany.String != "" && requestedScope != "company" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "cannot change existing company type to global scope", nil)
	}
	if !typeExists {
		typeCompanyID := ""
		if requestedScope == "company" {
			typeCompanyID = companyID
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO disclosure_types (type_id, company_id, group_id, active_version_no, status)
			VALUES (?, ?, ?, 0, 'active')
		`, req.TypeID, nullIfBlank(typeCompanyID), groupID); err != nil {
			return nil, err
		}
	}

	activeVersionNo := 0
	if currentVersion.Valid {
		activeVersionNo = int(currentVersion.Int64)
	}
	// First create / no portal active yet → new row becomes active+released.
	nextIsActive := !typeExists || activeVersionNo <= 0

	// Single open draft: highest mutable (not released) version that is not portal-active.
	var openDraft sql.NullInt64
	if typeExists && activeVersionNo > 0 {
		if err := tx.QueryRowContext(ctx, `
			SELECT MAX(version_no) FROM disclosure_type_versions
			WHERE type_id = ? AND version_no <> ? AND COALESCE(is_released, 0) = 0
		`, req.TypeID, activeVersionNo).Scan(&openDraft); err != nil {
			return nil, err
		}
	}

	tagsJSON, err := json.Marshal(req.Tags)
	if err != nil {
		return nil, fmt.Errorf("marshal tags: %w", err)
	}
	legalBasesJSON, err := json.Marshal(req.LegalBases)
	if err != nil {
		return nil, fmt.Errorf("marshal legal bases: %w", err)
	}
	checklistJSON, err := json.Marshal(req.Checklist)
	if err != nil {
		return nil, fmt.Errorf("marshal checklist: %w", err)
	}
	deadlineConfigJSON, err := json.Marshal(req.DeadlineConfig)
	if err != nil {
		return nil, fmt.Errorf("marshal deadline config: %w", err)
	}
	applicabilityRulesJSON, err := applicability.MarshalRulesJSON(req.ApplicabilityRules)
	if err != nil {
		return nil, fmt.Errorf("marshal applicability rules: %w", err)
	}
	blocks, err := normalizeTemplateBlocks(req.Blocks)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	versionDescription := strings.TrimSpace(req.Description)

	var targetVersion int
	overwriteDraft := openDraft.Valid && openDraft.Int64 > 0 && !nextIsActive
	if overwriteDraft {
		targetVersion = int(openDraft.Int64)
		if _, err := tx.ExecContext(ctx, `
			UPDATE disclosure_type_versions SET
				name = ?, category = ?, template_category = ?, deadline_strategy = ?,
				description = '', legal_basis = NULLIF(?, ''), applicability = NULLIF(?, ''),
				implementation_content = NULLIF(?, ''), implementation_notes = NULLIF(?, ''),
				special_cases = NULLIF(?, ''), report_content = NULLIF(?, ''), required_docs = NULLIF(?, ''),
				deadline_rule = NULLIF(?, ''), periodicity = NULLIF(?, ''), channels_text = NULLIF(?, ''),
				beneficiaries = NULLIF(?, ''), receiving_authorities = NULLIF(?, ''), format = NULLIF(?, ''),
				legal_risks_text = NULLIF(?, ''), general_info = NULLIF(?, ''),
				deadline_config_json = CAST(? AS JSON), legal_bases_json = CAST(? AS JSON),
				checklist_json = CAST(? AS JSON), tags_json = CAST(? AS JSON),
				applicability_rules_json = CAST(? AS JSON), change_note = NULLIF(?, ''),
				updated_by = ?, is_released = 0
			WHERE type_id = ? AND version_no = ?
		`, req.Name, req.Category, req.TemplateCategory, req.DeadlineStrategy,
			req.LegalBasis, req.Applicability, req.ImplementationContent, req.ImplementationNotes, req.SpecialCases, req.ReportContent,
			req.RequiredDocs, req.DeadlineRule, req.Periodicity, req.ChannelsText, req.Beneficiaries, req.ReceivingAuthorities,
			req.Format, req.LegalRisksText, req.GeneralInfo, string(deadlineConfigJSON), string(legalBasesJSON), string(checklistJSON), string(tagsJSON), string(applicabilityRulesJSON), changeNote, req.Subject.UserID,
			req.TypeID, targetVersion); err != nil {
			return nil, err
		}
		if versionDescription != "" {
			if _, err := tx.ExecContext(ctx, `
				UPDATE disclosure_type_versions SET description = ?
				WHERE type_id = ? AND version_no = ?
			`, versionDescription, req.TypeID, targetVersion); err != nil {
				return nil, err
			}
		}
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM disclosure_template_blocks WHERE type_id = ? AND version_no = ?
		`, req.TypeID, targetVersion); err != nil {
			return nil, err
		}
	} else {
		var maxVersion sql.NullInt64
		if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(version_no), 0) FROM disclosure_type_versions WHERE type_id = ?
		`, req.TypeID).Scan(&maxVersion); err != nil {
			return nil, err
		}
		targetVersion = int(maxVersion.Int64) + 1
		if targetVersion < 1 {
			targetVersion = 1
		}
		isReleased := 0
		if nextIsActive {
			isReleased = 1
		}
		// Insert with empty description first: combined INSERT packet can exceed max_allowed_packet.
		_, err = tx.ExecContext(ctx, `
			INSERT INTO disclosure_type_versions (
				type_id, version_no, name, category, template_category, deadline_strategy, description, legal_basis, applicability, implementation_content,
				implementation_notes, special_cases, report_content, required_docs, deadline_rule, periodicity, channels_text,
				beneficiaries, receiving_authorities, format, legal_risks_text, general_info, deadline_config_json, legal_bases_json, checklist_json, tags_json, applicability_rules_json, change_note, is_released, updated_by, activated_at
			) VALUES (?, ?, ?, ?, ?, ?, '', NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''),
			          NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''),
			          NULLIF(?, ''), NULLIF(?, ''), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), NULLIF(?, ''), ?, ?, ?)
		`, req.TypeID, targetVersion, req.Name, req.Category, req.TemplateCategory, req.DeadlineStrategy,
			req.LegalBasis, req.Applicability, req.ImplementationContent, req.ImplementationNotes, req.SpecialCases, req.ReportContent,
			req.RequiredDocs, req.DeadlineRule, req.Periodicity, req.ChannelsText, req.Beneficiaries, req.ReceivingAuthorities,
			req.Format, req.LegalRisksText, req.GeneralInfo, string(deadlineConfigJSON), string(legalBasesJSON), string(checklistJSON), string(tagsJSON), string(applicabilityRulesJSON), changeNote, isReleased, req.Subject.UserID, now)
		if err != nil {
			return nil, err
		}
		if versionDescription != "" {
			if _, err := tx.ExecContext(ctx, `
				UPDATE disclosure_type_versions SET description = ?
				WHERE type_id = ? AND version_no = ?
			`, versionDescription, req.TypeID, targetVersion); err != nil {
				return nil, err
			}
		}
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
		title := strings.TrimSpace(block.Title)
		nameEn := strings.TrimSpace(block.NameEN)
		nameVi := strings.TrimSpace(block.NameVI)
		if nameVi == "" && title != "" {
			nameVi = title
		}
		desc := strings.TrimSpace(block.Description)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO disclosure_template_blocks (
				type_id, version_no, block_id, block_key, block_type, title, name_en, name_vi, description, config_json, validation_json, display_order, enabled
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CAST(? AS JSON), CAST(? AS JSON), ?, ?)
		`, req.TypeID, targetVersion, block.BlockID, block.BlockKey, block.BlockType, title, nullIfBlank(nameEn), nullIfBlank(nameVi), nullIfBlank(desc), string(configJSON), string(validationJSON), block.DisplayOrder, block.Enabled); err != nil {
			return nil, err
		}
	}

	persistedActive := activeVersionNo
	if nextIsActive {
		persistedActive = targetVersion
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE disclosure_types
		SET group_id = ?, active_version_no = ?, status = 'active', updated_at = CURRENT_TIMESTAMP
		WHERE type_id = ?
	`, groupID, persistedActive, req.TypeID)
	if err != nil {
		return nil, err
	}
	if err := r.replaceTemplateDisplayGroups(ctx, tx, req.TypeID, req.DisplayGroupCodes); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &disclosureapp.UpsertTypeVersionResponse{
		TypeID:      req.TypeID,
		VersionNo:   targetVersion,
		IsActive:    nextIsActive,
		UpdatedBy:   req.Subject.UserID,
		ActivatedAt: now,
	}, nil
}

func (r *Repository) ListTypeVersions(ctx context.Context, companyID, typeID string) ([]disclosureapp.DisclosureTypeVersionDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT v.type_id, v.version_no,
		       (v.version_no = t.active_version_no) AS is_active,
		       COALESCE(v.is_released, 0) AS is_released,
		       COALESCE(v.change_note, ''), v.updated_by, v.activated_at
		FROM disclosure_type_versions v
		INNER JOIN disclosure_types t ON t.type_id = v.type_id
		WHERE v.type_id = ? AND (t.company_id IS NULL OR t.company_id = ?)
		  AND (
		    v.version_no = t.active_version_no
		    OR COALESCE(v.is_released, 0) = 1
		    OR v.version_no = (
		      SELECT MAX(v2.version_no) FROM disclosure_type_versions v2
		      WHERE v2.type_id = v.type_id
		        AND v2.version_no <> t.active_version_no
		        AND COALESCE(v2.is_released, 0) = 0
		    )
		  )
		ORDER BY v.version_no DESC
	`, typeID, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]disclosureapp.DisclosureTypeVersionDTO, 0)
	for rows.Next() {
		var item disclosureapp.DisclosureTypeVersionDTO
		var isReleased int
		if err := rows.Scan(&item.TypeID, &item.VersionNo, &item.IsActive, &isReleased, &item.ChangeNote, &item.UpdatedBy, &item.ActivatedAt); err != nil {
			return nil, err
		}
		item.IsReleased = isReleased == 1
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
		    activated_at = CASE WHEN version_no = ? THEN ? ELSE activated_at END,
		    is_released = CASE WHEN version_no = ? THEN 1 ELSE is_released END
		WHERE type_id = ?
	`, req.VersionNo, req.Subject.UserID, req.VersionNo, now, req.VersionNo, req.TypeID)
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

func (r *Repository) GetCompanyWorkflowOverride(ctx context.Context, companyID, typeID string) (*disclosureapp.CompanyWorkflowOverrideViewDTO, error) {
	view := &disclosureapp.CompanyWorkflowOverrideViewDTO{
		TypeID:          typeID,
		CompanyID:       companyID,
		EffectiveSource: "global_template",
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT override_id, status, active_version_no, updated_at
		FROM company_template_workflow_overrides
		WHERE company_id = ? AND type_id = ?
	`, companyID, typeID)
	var header disclosureapp.CompanyWorkflowOverrideHeaderDTO
	if err := row.Scan(&header.OverrideID, &header.Status, &header.ActiveVersionNo, &header.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return view, nil
		}
		return nil, err
	}
	header.CompanyID = companyID
	header.TypeID = typeID
	view.Override = &header

	if header.ActiveVersionNo > 0 {
		active, err := r.getCompanyWorkflowOverrideVersion(ctx, header.OverrideID, header.ActiveVersionNo)
		if err != nil {
			return nil, err
		}
		view.ActiveVersion = active
		view.EffectiveSource = "company_override"
	}
	draftRow := r.db.QueryRowContext(ctx, `
		SELECT version_no, state, COALESCE(change_note, ''), workflow_json, created_by, COALESCE(approved_by, ''), approved_at, created_at
		FROM company_template_workflow_override_versions
		WHERE override_id = ? AND state = 'draft'
		ORDER BY version_no DESC
		LIMIT 1
	`, header.OverrideID)
	draft, err := scanCompanyOverrideVersion(draftRow)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if err == nil {
		view.DraftVersion = draft
	}
	return view, nil
}

func (r *Repository) UpsertCompanyWorkflowOverrideDraft(ctx context.Context, req disclosureapp.UpsertCompanyWorkflowOverrideDraftRequest) (*disclosureapp.UpsertCompanyWorkflowOverrideDraftResponse, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC()

	var overrideID string
	var activeVersionNo int
	err = tx.QueryRowContext(ctx, `
		SELECT override_id, active_version_no
		FROM company_template_workflow_overrides
		WHERE company_id = ? AND type_id = ?
		FOR UPDATE
	`, req.Subject.CompanyID, req.TypeID).Scan(&overrideID, &activeVersionNo)
	if err != nil {
		if err != sql.ErrNoRows {
			return nil, err
		}
		overrideID = fmt.Sprintf("ovr_%s_%s", req.Subject.CompanyID, req.TypeID)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO company_template_workflow_overrides (
				override_id, company_id, type_id, active_version_no, status, created_by, updated_by, created_at, updated_at
			) VALUES (?, ?, ?, 0, 'draft', ?, ?, ?, ?)
		`, overrideID, req.Subject.CompanyID, req.TypeID, req.Subject.UserID, req.Subject.UserID, now, now); err != nil {
			return nil, err
		}
	}

	// Stale etag check: if caller sent a base version, ensure it matches the latest draft.
	if req.BaseVersionNo > 0 {
		var latestDraft int
		if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(version_no), 0)
			FROM company_template_workflow_override_versions
			WHERE override_id = ? AND state = 'draft'
		`, overrideID).Scan(&latestDraft); err != nil {
			return nil, err
		}
		if latestDraft > 0 && req.BaseVersionNo != latestDraft {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStaleEtag, "draft has been modified by another session; reload and retry", nil)
		}
	}

	var nextVersion int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version_no), 0) + 1
		FROM company_template_workflow_override_versions
		WHERE override_id = ?
	`, overrideID).Scan(&nextVersion); err != nil {
		return nil, err
	}
	workflowJSON, err := json.Marshal(req.Workflow)
	if err != nil {
		return nil, fmt.Errorf("marshal workflow: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO company_template_workflow_override_versions (
			override_id, version_no, workflow_json, change_note, state, created_by, created_at, updated_at
		) VALUES (?, ?, CAST(? AS JSON), NULLIF(?, ''), 'draft', ?, ?, ?)
	`, overrideID, nextVersion, string(workflowJSON), req.ChangeNote, req.Subject.UserID, now, now); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE company_template_workflow_overrides
		SET status = 'draft', updated_by = ?, updated_at = ?
		WHERE override_id = ?
	`, req.Subject.UserID, now, overrideID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &disclosureapp.UpsertCompanyWorkflowOverrideDraftResponse{
		OverrideID:     overrideID,
		TypeID:         req.TypeID,
		CompanyID:      req.Subject.CompanyID,
		DraftVersionNo: nextVersion,
		State:          "draft",
		UpdatedAt:      now,
	}, nil
}

func (r *Repository) ApproveCompanyWorkflowOverride(ctx context.Context, req disclosureapp.ApproveCompanyWorkflowOverrideRequest) (*disclosureapp.ApproveCompanyWorkflowOverrideResponse, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC()

	var overrideID string
	if err := tx.QueryRowContext(ctx, `
		SELECT override_id
		FROM company_template_workflow_overrides
		WHERE company_id = ? AND type_id = ?
		FOR UPDATE
	`, req.Subject.CompanyID, req.TypeID).Scan(&overrideID); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "workflow override not found", nil)
		}
		return nil, err
	}
	var state, createdBy string
	if err := tx.QueryRowContext(ctx, `
		SELECT state, COALESCE(created_by, '')
		FROM company_template_workflow_override_versions
		WHERE override_id = ? AND version_no = ?
	`, overrideID, req.VersionNo).Scan(&state, &createdBy); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "workflow override version not found", nil)
		}
		return nil, err
	}
	if state != "draft" {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "workflow override version is not draft", nil)
	}
	// Self-approval guard: drafter cannot approve their own draft.
	// Bypassed when SkipSelfApprovalCheck=true (save+apply path).
	if !req.SkipSelfApprovalCheck && createdBy != "" && createdBy == req.Subject.UserID {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodeSelfApprovalNotAllowed, "drafter cannot approve their own workflow draft", nil)
	}
	// Stale etag guard: req.VersionNo must be the latest draft version.
	var latestDraft int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version_no), 0)
		FROM company_template_workflow_override_versions
		WHERE override_id = ? AND state = 'draft'
	`, overrideID).Scan(&latestDraft); err != nil {
		return nil, err
	}
	if latestDraft > 0 && req.VersionNo != latestDraft {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStaleEtag, "a newer draft version exists; reload and approve the latest draft", nil)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE company_template_workflow_override_versions
		SET state = CASE WHEN version_no = ? THEN 'approved' ELSE state END,
		    approved_by = CASE WHEN version_no = ? THEN ? ELSE approved_by END,
		    approved_at = CASE WHEN version_no = ? THEN ? ELSE approved_at END,
		    updated_at = CASE WHEN version_no = ? THEN ? ELSE updated_at END
		WHERE override_id = ?
	`, req.VersionNo, req.VersionNo, req.Subject.UserID, req.VersionNo, now, req.VersionNo, now, overrideID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE company_template_workflow_overrides
		SET active_version_no = ?, status = 'approved', approved_by = ?, approved_at = ?, updated_by = ?, updated_at = ?
		WHERE override_id = ?
	`, req.VersionNo, req.Subject.UserID, now, req.Subject.UserID, now, overrideID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &disclosureapp.ApproveCompanyWorkflowOverrideResponse{
		OverrideID:      overrideID,
		TypeID:          req.TypeID,
		CompanyID:       req.Subject.CompanyID,
		ActiveVersionNo: req.VersionNo,
		State:           "approved",
		ApprovedBy:      req.Subject.UserID,
		ApprovedAt:      now,
		EffectiveSource: "company_override",
	}, nil
}

func (r *Repository) DeleteCompanyWorkflowOverrideDraft(ctx context.Context, req disclosureapp.DeleteCompanyWorkflowOverrideDraftRequest) (*disclosureapp.DeleteCompanyWorkflowOverrideDraftResponse, error) {
	res, err := r.db.ExecContext(ctx, `
		DELETE v FROM company_template_workflow_override_versions v
		INNER JOIN company_template_workflow_overrides o ON o.override_id = v.override_id
		WHERE o.company_id = ? AND o.type_id = ? AND v.version_no = ? AND v.state = 'draft'
	`, req.Subject.CompanyID, req.TypeID, req.VersionNo)
	if err != nil {
		return nil, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "draft version not found", nil)
	}
	return &disclosureapp.DeleteCompanyWorkflowOverrideDraftResponse{Deleted: true, VersionNo: req.VersionNo}, nil
}

func (r *Repository) ResetCompanyWorkflowOverrideActive(ctx context.Context, req disclosureapp.ResetCompanyWorkflowOverrideActiveRequest) (*disclosureapp.ResetCompanyWorkflowOverrideActiveResponse, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE company_template_workflow_overrides
		SET active_version_no = 0, status = 'archived', updated_by = ?, updated_at = CURRENT_TIMESTAMP
		WHERE company_id = ? AND type_id = ?
	`, req.Subject.UserID, req.Subject.CompanyID, req.TypeID)
	if err != nil {
		return nil, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return &disclosureapp.ResetCompanyWorkflowOverrideActiveResponse{
			TypeID:          req.TypeID,
			CompanyID:       req.Subject.CompanyID,
			ActiveVersionNo: 0,
			State:           "archived",
			EffectiveSource: "global_template",
		}, nil
	}
	var overrideID string
	_ = r.db.QueryRowContext(ctx, `
		SELECT override_id FROM company_template_workflow_overrides WHERE company_id = ? AND type_id = ?
	`, req.Subject.CompanyID, req.TypeID).Scan(&overrideID)
	return &disclosureapp.ResetCompanyWorkflowOverrideActiveResponse{
		OverrideID:      overrideID,
		TypeID:          req.TypeID,
		CompanyID:       req.Subject.CompanyID,
		ActiveVersionNo: 0,
		State:           "archived",
		EffectiveSource: "global_template",
	}, nil
}

func (r *Repository) ListCompanyWorkflowOverrideVersions(ctx context.Context, companyID, typeID string, page, pageSize int) ([]disclosureapp.CompanyWorkflowOverrideVersionDTO, int, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM company_template_workflow_override_versions v
		INNER JOIN company_template_workflow_overrides o ON o.override_id = v.override_id
		WHERE o.company_id = ? AND o.type_id = ?
	`, companyID, typeID)
	var total int
	if err := row.Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	rows, err := r.db.QueryContext(ctx, `
		SELECT v.version_no, v.state, COALESCE(v.change_note, ''), v.workflow_json, v.created_by, COALESCE(v.approved_by, ''), v.approved_at, v.created_at
		FROM company_template_workflow_override_versions v
		INNER JOIN company_template_workflow_overrides o ON o.override_id = v.override_id
		WHERE o.company_id = ? AND o.type_id = ?
		ORDER BY v.version_no DESC
		LIMIT ? OFFSET ?
	`, companyID, typeID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]disclosureapp.CompanyWorkflowOverrideVersionDTO, 0)
	for rows.Next() {
		item, err := scanCompanyOverrideVersion(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *item)
	}
	return out, total, rows.Err()
}

func (r *Repository) GetEffectiveWorkflow(ctx context.Context, companyID, typeID string) (*disclosureapp.EffectiveWorkflowDTO, error) {
	view, err := r.GetCompanyWorkflowOverride(ctx, companyID, typeID)
	if err != nil {
		return nil, err
	}
	dto := &disclosureapp.EffectiveWorkflowDTO{
		TypeID:    typeID,
		CompanyID: companyID,
		Source:    "global_template",
		VersionNo: 0,
		Workflow:  []disclosureapp.WorkflowStepDTO{},
	}
	if view.ActiveVersion != nil {
		dto.Source = "company_override"
		dto.VersionNo = view.ActiveVersion.VersionNo
		dto.Workflow = view.ActiveVersion.Workflow
		if len(dto.Workflow) == 0 {
			dto.OverrideInvalidEmpty = true
			if steps, _, ok, err := r.loadActiveGlobalWorkflow(ctx, typeID); err != nil {
				return nil, err
			} else if ok && len(steps) > 0 {
				dto.GlobalWorkflowAvailable = true
			}
		}
		return dto, nil
	}
	if steps, versionNo, ok, err := r.loadActiveGlobalWorkflow(ctx, typeID); err != nil {
		return nil, err
	} else if ok {
		dto.Source = "global_workflow"
		dto.VersionNo = versionNo
		dto.Workflow = steps
		return dto, nil
	}
	detail, err := r.GetTypeDetail(ctx, companyID, typeID)
	if err != nil {
		return nil, err
	}
	dto.VersionNo = detail.VersionNo
	dto.Workflow = disclosureapp.ExtractTemplateWorkflow(detail.Blocks)
	return dto, nil
}

func (r *Repository) getCompanyWorkflowOverrideVersion(ctx context.Context, overrideID string, versionNo int) (*disclosureapp.CompanyWorkflowOverrideVersionDTO, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT version_no, state, COALESCE(change_note, ''), workflow_json, created_by, COALESCE(approved_by, ''), approved_at, created_at
		FROM company_template_workflow_override_versions
		WHERE override_id = ? AND version_no = ?
	`, overrideID, versionNo)
	item, err := scanCompanyOverrideVersion(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "workflow override version not found", nil)
		}
		return nil, err
	}
	return item, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCompanyOverrideVersion(scanner rowScanner) (*disclosureapp.CompanyWorkflowOverrideVersionDTO, error) {
	var item disclosureapp.CompanyWorkflowOverrideVersionDTO
	var workflowRaw []byte
	var approvedBy string
	var approvedAt sql.NullTime
	if err := scanner.Scan(&item.VersionNo, &item.State, &item.ChangeNote, &workflowRaw, &item.CreatedBy, &approvedBy, &approvedAt, &item.CreatedAt); err != nil {
		return nil, err
	}
	if len(workflowRaw) > 0 {
		if err := json.Unmarshal(workflowRaw, &item.Workflow); err != nil {
			return nil, err
		}
	} else {
		item.Workflow = []disclosureapp.WorkflowStepDTO{}
	}
	item.ApprovedBy = strings.TrimSpace(approvedBy)
	if approvedAt.Valid {
		t := approvedAt.Time
		item.ApprovedAt = &t
	}
	return &item, nil
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

func decodeDeadlineConfigJSON(raw []byte, target **disclosureapp.TemplateDeadlineConfig) error {
	if len(raw) == 0 {
		*target = nil
		return nil
	}
	var parsed disclosureapp.TemplateDeadlineConfig
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return err
	}
	if strings.TrimSpace(parsed.DeadlineMode) == "" {
		*target = nil
		return nil
	}
	*target = &parsed
	return nil
}

func (r *Repository) GetCompanyApplicabilityProfile(ctx context.Context, companyID string) (applicability.CompanyApplicabilityProfile, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(is_listed, 0), COALESCE(is_large_public, 0), COALESCE(is_non_large_public, 0),
			COALESCE(has_subsidiaries, 0), COALESCE(has_subordinate_accounting_units, 0),
			COALESCE(business_sector, '')
		FROM companies
		WHERE company_id = ?
	`, companyID)
	var isListed, isLargePublic, isNonLargePublic, hasSubsidiaries, hasSubordinateAccountingUnits int
	var businessSector string
	if err := row.Scan(&isListed, &isLargePublic, &isNonLargePublic, &hasSubsidiaries, &hasSubordinateAccountingUnits, &businessSector); err != nil {
		if err == sql.ErrNoRows {
			return applicability.CompanyApplicabilityProfile{}, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "company not found", nil)
		}
		return applicability.CompanyApplicabilityProfile{}, err
	}
	return applicability.ProfileFromCompanyDetail(
		isListed == 1, isLargePublic == 1, isNonLargePublic == 1,
		hasSubsidiaries == 1, hasSubordinateAccountingUnits == 1,
		businessSector,
	), nil
}

func (r *Repository) GetCompanyDeadlineContext(ctx context.Context, companyID string) (disclosureapp.CompanyDeadlineContext, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT established_date, established_month, established_day
		FROM companies
		WHERE company_id = ?
	`, companyID)
	var establishedDate sql.NullTime
	var establishedMonth int
	var establishedDay int
	if err := row.Scan(&establishedDate, &establishedMonth, &establishedDay); err != nil {
		if err == sql.ErrNoRows {
			return disclosureapp.CompanyDeadlineContext{}, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "company not found", nil)
		}
		return disclosureapp.CompanyDeadlineContext{}, err
	}
	ctxOut := disclosureapp.CompanyDeadlineContext{
		CompanyID:        companyID,
		CurrentYear:      time.Now().UTC().Year(),
		EstablishedMonth: establishedMonth,
		EstablishedDay:   establishedDay,
	}
	if establishedDate.Valid {
		t := establishedDate.Time
		ctxOut.EstablishedDate = &t
	}
	return ctxOut, nil
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
		SELECT block_id, block_key, block_type, title,
			NULLIF(TRIM(COALESCE(name_en, '')), ''),
			NULLIF(TRIM(COALESCE(name_vi, '')), ''),
			COALESCE(description, ''), config_json, validation_json, display_order, enabled
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
		var nameEn, nameVi sql.NullString
		if err := rows.Scan(
			&item.BlockID,
			&item.BlockKey,
			&item.BlockType,
			&item.Title,
			&nameEn,
			&nameVi,
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
		if nameEn.Valid {
			item.NameEN = strings.TrimSpace(nameEn.String)
		}
		if nameVi.Valid {
			item.NameVI = strings.TrimSpace(nameVi.String)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	disclosureapp.EnrichTemplateBlockDisplayNames(out)
	return out, nil
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

func nullIfBlank(value string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func (r *Repository) GetActiveVersionDeadlineConfig(ctx context.Context, typeID string) (int, *disclosureapp.TemplateDeadlineConfig, error) {
	var versionNo int
	var rawConfig sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT v.version_no, v.deadline_config_json
		FROM disclosure_type_versions v
		INNER JOIN disclosure_types t ON t.type_id = v.type_id AND t.active_version_no = v.version_no
		WHERE v.type_id = ?
	`, typeID).Scan(&versionNo, &rawConfig)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type not found", nil)
		}
		return 0, nil, fmt.Errorf("get active version deadline config: %w", err)
	}
	if !rawConfig.Valid || rawConfig.String == "" || rawConfig.String == "null" {
		return versionNo, nil, nil
	}
	var cfg disclosureapp.TemplateDeadlineConfig
	if err := json.Unmarshal([]byte(rawConfig.String), &cfg); err != nil {
		return versionNo, nil, fmt.Errorf("unmarshal deadline_config_json: %w", err)
	}
	return versionNo, &cfg, nil
}

func (r *Repository) UpdateActiveVersionDeadlineConfig(ctx context.Context, typeID string, cfg disclosureapp.TemplateDeadlineConfig, updatedBy string) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal deadline config: %w", err)
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE disclosure_type_versions v
		INNER JOIN disclosure_types t ON t.type_id = v.type_id AND t.active_version_no = v.version_no
		SET v.deadline_config_json = CAST(? AS JSON), v.updated_by = ?
		WHERE v.type_id = ?
	`, string(raw), updatedBy, typeID)
	if err != nil {
		return fmt.Errorf("update active version deadline config: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "disclosure type not found or no active version", nil)
	}
	return nil
}

// ─── Sprint C: Groups / tổ nhóm ─────────────────────────────────────────────

// ListCompanyGroups returns team-level org units (tổ/nhóm) for a company,
// optionally filtered by department (parent org_unit) and active status.
func (r *Repository) ListCompanyGroups(ctx context.Context, companyID, departmentID string, isActive *bool) ([]disclosureapp.CompanyGroupDTO, error) {
	query := `
		SELECT t.org_unit_id, t.unit_name, d.org_unit_id, d.unit_name, t.status
		FROM org_units t
		INNER JOIN org_units d ON d.org_unit_id = t.parent_org_unit_id AND d.unit_type = 'department'
		WHERE t.company_id = ? AND t.unit_type = 'team'
	`
	args := []any{companyID}
	if strings.TrimSpace(departmentID) != "" {
		query += " AND t.parent_org_unit_id = ?"
		args = append(args, departmentID)
	}
	if isActive != nil {
		if *isActive {
			query += " AND t.status = 'active'"
		} else {
			query += " AND t.status != 'active'"
		}
	}
	query += " ORDER BY d.unit_name ASC, t.unit_name ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list company groups: %w", err)
	}
	defer rows.Close()
	out := make([]disclosureapp.CompanyGroupDTO, 0)
	for rows.Next() {
		var item disclosureapp.CompanyGroupDTO
		var status string
		if err := rows.Scan(&item.GroupID, &item.GroupName, &item.DepartmentID, &item.DepartmentName, &status); err != nil {
			return nil, err
		}
		item.IsActive = status == "active"
		out = append(out, item)
	}
	return out, rows.Err()
}

// UpdateWorkflowOverrideStepGroups patches the groups field of one step in the latest draft version.
// The entire workflow JSON for that version is read, the step is located, groups are updated, and the JSON is written back.
func (r *Repository) UpdateWorkflowOverrideStepGroups(ctx context.Context, req disclosureapp.UpdateWorkflowOverrideStepGroupsRequest) (*disclosureapp.UpdateWorkflowOverrideStepGroupsResponse, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// Resolve override and find draft version.
	var overrideID string
	if err := tx.QueryRowContext(ctx, `
		SELECT override_id FROM company_template_workflow_overrides
		WHERE company_id = ? AND type_id = ? FOR UPDATE
	`, req.Subject.CompanyID, req.TypeID).Scan(&overrideID); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "workflow override not found", nil)
		}
		return nil, err
	}

	// Resolve base_etag → version no.
	baseVersionNo := disclosureapp.ResolveWorkflowBaseVersionNo(0, req.BaseEtag)

	var draftVersionNo int
	var workflowRaw []byte
	if err := tx.QueryRowContext(ctx, `
		SELECT version_no, workflow_json
		FROM company_template_workflow_override_versions
		WHERE override_id = ? AND state = 'draft'
		ORDER BY version_no DESC LIMIT 1
	`, overrideID).Scan(&draftVersionNo, &workflowRaw); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "no draft version found", nil)
		}
		return nil, err
	}
	if baseVersionNo > 0 && baseVersionNo != draftVersionNo {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStaleEtag, "draft has been modified; reload and retry", nil)
	}

	// Unmarshal workflow, locate step, validate groups belong to step's department.
	var steps []disclosureapp.WorkflowStepDTO
	if len(workflowRaw) > 0 {
		if err := json.Unmarshal(workflowRaw, &steps); err != nil {
			return nil, fmt.Errorf("unmarshal workflow: %w", err)
		}
	}
	stepIdx := -1
	for i := range steps {
		if steps[i].StepID == req.StepID {
			stepIdx = i
			break
		}
	}
	if stepIdx < 0 {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "step not found in draft", nil)
	}
	stepDeptID := steps[stepIdx].DepartmentID

	// Validate all groups belong to step's department (if we can look them up)
	// and fetch their names in the same pass.
	groupNameMap := make(map[string]string, len(req.Groups))
	var deptName string
	if len(req.Groups) > 0 {
		placeholders := make([]string, len(req.Groups))
		args := []any{req.Subject.CompanyID}
		for i, g := range req.Groups {
			placeholders[i] = "?"
			args = append(args, g.GroupID)
		}
		// Fetch group names and validate department membership in one query.
		nameQuery := fmt.Sprintf(`
			SELECT org_unit_id, unit_name, parent_org_unit_id
			FROM org_units
			WHERE company_id = ? AND org_unit_id IN (%s) AND unit_type = 'team'
		`, strings.Join(placeholders, ","))
		nameRows, nameErr := tx.QueryContext(ctx, nameQuery, args...)
		if nameErr != nil {
			return nil, nameErr
		}
		var rowScanErr error
		for nameRows.Next() {
			var gid, gname, parentID string
			if err := nameRows.Scan(&gid, &gname, &parentID); err != nil {
				rowScanErr = err
				break
			}
			groupNameMap[gid] = gname
			if stepDeptID != "" && parentID != stepDeptID {
				rowScanErr = perr.NewHTTPError(http.StatusBadRequest, perr.CodeGroupNotInDepartment, "one or more groups do not belong to the step's department", nil)
				break
			}
		}
		_ = nameRows.Close()
		if rowScanErr != nil {
			return nil, rowScanErr
		}
		if err := nameRows.Err(); err != nil {
			return nil, err
		}
		if stepDeptID != "" && len(groupNameMap) != len(req.Groups) {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeGroupNotInDepartment, "one or more groups do not belong to the step's department", nil)
		}
	}

	// Fetch department name for the step (best-effort; skip on error).
	if stepDeptID != "" {
		_ = tx.QueryRowContext(ctx,
			`SELECT unit_name FROM org_units WHERE org_unit_id = ? AND unit_type = 'department' LIMIT 1`,
			stepDeptID,
		).Scan(&deptName)
	}

	// Build updated groups slice.
	var updatedGroups []disclosureapp.WorkflowStepGroupDTO
	if !req.ClearAll && len(req.Groups) > 0 {
		updatedGroups = make([]disclosureapp.WorkflowStepGroupDTO, 0, len(req.Groups))
		for _, g := range req.Groups {
			dto := disclosureapp.WorkflowStepGroupDTO{
				GroupID:        g.GroupID,
				GroupName:      groupNameMap[g.GroupID],
				DepartmentID:   stepDeptID,
				DepartmentName: deptName,
				Source:         "manual",
				DurationMode:   g.DurationMode,
				DisplayOrder:   g.DisplayOrder,
				IsActive:       true,
			}
			if g.ProcessingDays != nil {
				dto.ProcessingDays = g.ProcessingDays
			}
			updatedGroups = append(updatedGroups, dto)
		}
	}
	steps[stepIdx].Groups = updatedGroups

	// Write back updated workflow JSON.
	newJSON, err := json.Marshal(steps)
	if err != nil {
		return nil, fmt.Errorf("marshal updated workflow: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE company_template_workflow_override_versions
		SET workflow_json = CAST(? AS JSON), updated_at = ?
		WHERE override_id = ? AND version_no = ?
	`, string(newJSON), time.Now().UTC(), overrideID, draftVersionNo); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	newEtag := disclosureapp.WorkflowDraftEtagFromVersion(draftVersionNo)
	return &disclosureapp.UpdateWorkflowOverrideStepGroupsResponse{
		DraftEtag: newEtag,
		StepID:    req.StepID,
		Groups:    updatedGroups,
	}, nil
}

// ── Periodic auto-creation ────────────────────────────────────────────────────

func (r *Repository) ListActivePeriodicTypes(ctx context.Context) ([]disclosureapp.PeriodicTypeRow, error) {
	const q = `
		SELECT dt.type_id,
		       COALESCE(JSON_UNQUOTE(JSON_EXTRACT(dtv.deadline_config_json, '$.frequency_unit')), '')   AS frequency_unit,
		       COALESCE(JSON_EXTRACT(dtv.deadline_config_json, '$.frequency_interval'), 1)              AS frequency_interval,
		       COALESCE(JSON_EXTRACT(dtv.deadline_config_json, '$.deadline_days'), 0)                   AS deadline_days,
		       COALESCE(JSON_EXTRACT(dtv.deadline_config_json, '$.cycle_anchor_day'), 0)                AS anchor_day,
		       COALESCE(JSON_EXTRACT(dtv.deadline_config_json, '$.cycle_anchor_month'), 0)              AS anchor_month,
		       CASE WHEN dt.company_id IS NULL THEN 1 ELSE 0 END AS is_global,
		       dtv.applicability_rules_json
		FROM disclosure_types dt
		JOIN disclosure_type_versions dtv ON dtv.type_id = dt.type_id AND dtv.version_no = dt.active_version_no
		WHERE JSON_UNQUOTE(JSON_EXTRACT(dtv.deadline_config_json, '$.template_category')) IN ('periodic', 'custom')
		  AND JSON_UNQUOTE(JSON_EXTRACT(dtv.deadline_config_json, '$.frequency_unit'))    IN ('monthly', 'quarterly', 'yearly')
		  AND dt.status = 'active'`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list periodic types: %w", err)
	}
	defer rows.Close()
	var out []disclosureapp.PeriodicTypeRow
	for rows.Next() {
		var row disclosureapp.PeriodicTypeRow
		var isGlobal int
		var rulesRaw []byte
		if err := rows.Scan(&row.TypeID, &row.FrequencyUnit, &row.FrequencyInterval,
			&row.DeadlineDays, &row.CycleAnchorDay, &row.CycleAnchorMonth, &isGlobal, &rulesRaw); err != nil {
			return nil, fmt.Errorf("scan periodic type row: %w", err)
		}
		row.IsGlobal = isGlobal == 1
		if rules, err := applicability.ParseRulesJSON(rulesRaw); err != nil {
			return nil, fmt.Errorf("parse applicability rules: %w", err)
		} else {
			row.ApplicabilityRules = rules
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *Repository) UpsertPeriodicCycle(ctx context.Context, in disclosureapp.PeriodicCycleRow) error {
	var cycleStart any
	if !in.CycleStart.IsZero() {
		cycleStart = in.CycleStart.Format("2006-01-02")
	}
	const q = `
		INSERT INTO periodic_cycles (cycle_id, type_id, company_id, cycle_label, cycle_start, due_date)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE cycle_id = cycle_id`
	_, err := r.db.ExecContext(ctx, q, in.CycleID, in.TypeID, in.CompanyID, in.CycleLabel, cycleStart, in.DueDate.Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("upsert periodic cycle: %w", err)
	}
	return nil
}

func (r *Repository) ListPendingCycles(ctx context.Context, asOf time.Time, bufferDays int) ([]disclosureapp.PeriodicCycleRow, error) {
	cutoff := asOf.AddDate(0, 0, bufferDays).Format("2006-01-02")
	const q = `
		SELECT pc.cycle_id, pc.type_id, COALESCE(dtv.name, ''), pc.company_id, pc.cycle_label,
		       pc.cycle_start, pc.due_date
		FROM periodic_cycles pc
		INNER JOIN disclosure_types dt ON dt.type_id = pc.type_id
		INNER JOIN disclosure_type_versions dtv ON dtv.type_id = dt.type_id AND dtv.version_no = dt.active_version_no
		WHERE pc.record_id IS NULL AND pc.materialized_at IS NULL AND pc.due_date <= ?
		ORDER BY pc.due_date ASC
		LIMIT 200`
	rows, err := r.db.QueryContext(ctx, q, cutoff)
	if err != nil {
		return nil, fmt.Errorf("list pending cycles: %w", err)
	}
	defer rows.Close()
	var out []disclosureapp.PeriodicCycleRow
	for rows.Next() {
		var row disclosureapp.PeriodicCycleRow
		var cycleStart, dueDate sql.NullTime
		if err := rows.Scan(&row.CycleID, &row.TypeID, &row.TypeName, &row.CompanyID, &row.CycleLabel, &cycleStart, &dueDate); err != nil {
			return nil, fmt.Errorf("scan pending cycle: %w", err)
		}
		if cycleStart.Valid {
			row.CycleStart = dateOnlyUTC(cycleStart.Time)
		}
		if dueDate.Valid {
			row.DueDate = dateOnlyUTC(dueDate.Time)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// TryClaimPeriodicCycle sets materialized_at as an optimistic claim (single-winner).
func (r *Repository) TryClaimPeriodicCycle(ctx context.Context, cycleID string) (bool, error) {
	const q = `
		UPDATE periodic_cycles
		SET materialized_at = NOW(3)
		WHERE cycle_id = ? AND record_id IS NULL AND materialized_at IS NULL`
	res, err := r.db.ExecContext(ctx, q, cycleID)
	if err != nil {
		return false, fmt.Errorf("claim periodic cycle: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("claim periodic cycle rows affected: %w", err)
	}
	return n == 1, nil
}

// ReleasePeriodicCycleClaim clears the claim when materialize fails before record_id is set.
func (r *Repository) ReleasePeriodicCycleClaim(ctx context.Context, cycleID string) error {
	const q = `
		UPDATE periodic_cycles
		SET materialized_at = NULL
		WHERE cycle_id = ? AND record_id IS NULL`
	_, err := r.db.ExecContext(ctx, q, cycleID)
	if err != nil {
		return fmt.Errorf("release periodic cycle claim: %w", err)
	}
	return nil
}

func (r *Repository) UpdateCycleRecord(ctx context.Context, cycleID, recordID string) error {
	const q = `
		UPDATE periodic_cycles
		SET record_id = ?, materialized_at = NOW(3)
		WHERE cycle_id = ? AND record_id IS NULL`
	res, err := r.db.ExecContext(ctx, q, recordID, cycleID)
	if err != nil {
		return fmt.Errorf("update cycle record: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update cycle record rows affected: %w", err)
	}
	if n != 1 {
		return fmt.Errorf("update cycle record: cycle %s not pending", cycleID)
	}
	return nil
}

func (r *Repository) ListAllActiveCompanyIDs(ctx context.Context) ([]string, error) {
	const q = `SELECT company_id FROM companies WHERE status = 'active'`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list active companies: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *Repository) GetCompanyTypePreference(ctx context.Context, companyID, typeID string) (*disclosureapp.CompanyTypePreference, error) {
	const q = `SELECT auto_create_enabled, COALESCE(updated_by, ''),
	           COALESCE(cycle_anchor_month, 0), COALESCE(cycle_anchor_day, 0)
	           FROM company_type_preferences WHERE company_id = ? AND type_id = ?`
	var pref disclosureapp.CompanyTypePreference
	var enabled int
	err := r.db.QueryRowContext(ctx, q, companyID, typeID).Scan(
		&enabled, &pref.UpdatedBy, &pref.CycleAnchorMonth, &pref.CycleAnchorDay)
	if err == sql.ErrNoRows {
		return nil, nil // no row = default enabled
	}
	if err != nil {
		return nil, fmt.Errorf("get company type preference: %w", err)
	}
	pref.CompanyID = companyID
	pref.TypeID = typeID
	pref.AutoCreateEnabled = enabled == 1
	return &pref, nil
}

func (r *Repository) UpsertCompanyTypePreference(ctx context.Context, in disclosureapp.CompanyTypePreference) error {
	enabled := 0
	if in.AutoCreateEnabled {
		enabled = 1
	}
	const q = `
		INSERT INTO company_type_preferences (company_id, type_id, auto_create_enabled, updated_by, cycle_anchor_month, cycle_anchor_day)
		VALUES (?, ?, ?, ?, NULLIF(?, 0), NULLIF(?, 0))
		ON DUPLICATE KEY UPDATE
		  auto_create_enabled = VALUES(auto_create_enabled),
		  updated_by = VALUES(updated_by),
		  cycle_anchor_month = COALESCE(VALUES(cycle_anchor_month), cycle_anchor_month),
		  cycle_anchor_day   = COALESCE(VALUES(cycle_anchor_day),   cycle_anchor_day)`
	_, err := r.db.ExecContext(ctx, q, in.CompanyID, in.TypeID, enabled, in.UpdatedBy,
		in.CycleAnchorMonth, in.CycleAnchorDay)
	if err != nil {
		return fmt.Errorf("upsert company type preference: %w", err)
	}
	return nil
}

// GetCompanyTypeDeadlineContext returns CompanyDeadlineContext enriched with
// per-company cycle anchor override from company_type_preferences (if set).
func (r *Repository) GetCompanyTypeDeadlineContext(ctx context.Context, companyID, typeID string) (disclosureapp.CompanyDeadlineContext, error) {
	base, err := r.GetCompanyDeadlineContext(ctx, companyID)
	if err != nil {
		return base, err
	}
	pref, err := r.GetCompanyTypePreference(ctx, companyID, typeID)
	if err != nil || pref == nil {
		return base, err // no preference or error → use base context
	}
	if pref.CycleAnchorMonth > 0 {
		base.CycleAnchorMonth = pref.CycleAnchorMonth
	}
	if pref.CycleAnchorDay > 0 {
		base.CycleAnchorDay = pref.CycleAnchorDay
	}
	return base, nil
}

func (r *Repository) CountCompanyTemplatesByCompanyID(ctx context.Context, companyID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM disclosure_types WHERE company_id = ? AND status != 'archived'`,
		companyID).Scan(&count)
	return count, err
}

// BE-004A — Company-defined template persistence (portal path).

func (r *Repository) CreateCompanyTemplate(ctx context.Context, req disclosureapp.CreateCompanyTemplateRequest) (*disclosureapp.CompanyTemplateWriteResponse, error) {
	typeID := "dt-co-" + strings.ReplaceAll(fmt.Sprintf("%d", time.Now().UnixNano()), "-", "")
	now := time.Now().UTC()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO disclosure_types (type_id, company_id, group_id, active_version_no, status, review_status, created_at, updated_at)
		 VALUES (?, ?, 'group-006', 1, 'active', 'draft', ?, ?)`,
		typeID, req.Subject.CompanyID, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert disclosure_types: %w", err)
	}

	tagsJSON, _ := json.Marshal(req.Tags)
	_, err = tx.ExecContext(ctx,
		`INSERT INTO disclosure_type_versions
		 (type_id, version_no, name, category, template_category, description, legal_basis, applicability,
		  deadline_rule, periodicity, tags_json, change_note, updated_by, activated_at, created_at)
		 VALUES (?, 1, ?, 'Tùy chỉnh', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		typeID, req.Name, req.TemplateCategory, req.Description,
		req.LegalBasis, req.Applicability, req.DeadlineRule, req.Periodicity,
		string(tagsJSON), req.ChangeNote, req.Subject.MembershipID, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert disclosure_type_versions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &disclosureapp.CompanyTemplateWriteResponse{
		TypeID:           typeID,
		CompanyID:        req.Subject.CompanyID,
		Name:             req.Name,
		TemplateCategory: req.TemplateCategory,
		Description:      req.Description,
		ReviewStatus:     "draft",
		DeadlineRule:     req.DeadlineRule,
		Periodicity:      req.Periodicity,
		Tags:             req.Tags,
		CreatedAt:        now.Format(time.RFC3339),
		UpdatedAt:        now.Format(time.RFC3339),
	}, nil
}

func (r *Repository) UpdateCompanyTemplate(ctx context.Context, req disclosureapp.UpdateCompanyTemplateRequest) (*disclosureapp.CompanyTemplateWriteResponse, error) {
	now := time.Now().UTC()
	tagsJSON, _ := json.Marshal(req.Tags)

	res, err := r.db.ExecContext(ctx,
		`UPDATE disclosure_type_versions dtv
		 INNER JOIN disclosure_types dt ON dt.type_id = dtv.type_id
		 SET dtv.name = ?, dtv.template_category = COALESCE(NULLIF(?, ''), dtv.template_category),
		     dtv.description = ?, dtv.legal_basis = ?, dtv.applicability = ?,
		     dtv.deadline_rule = ?, dtv.periodicity = ?,
		     dtv.tags_json = ?, dtv.change_note = ?, dtv.updated_by = ?,
		     dt.updated_at = ?
		 WHERE dt.type_id = ? AND dt.company_id = ? AND dtv.version_no = dt.active_version_no`,
		req.Name, req.TemplateCategory,
		req.Description, req.LegalBasis, req.Applicability,
		req.DeadlineRule, req.Periodicity,
		string(tagsJSON), req.ChangeNote, req.Subject.MembershipID, now,
		req.TypeID, req.Subject.CompanyID)
	if err != nil {
		return nil, fmt.Errorf("update company template: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "company template not found", nil)
	}

	return r.GetCompanyTemplateForLifecycle(ctx, req.Subject.CompanyID, req.TypeID)
}

func (r *Repository) GetCompanyTemplateForLifecycle(ctx context.Context, companyID, typeID string) (*disclosureapp.CompanyTemplateWriteResponse, error) {
	const q = `
		SELECT dt.type_id, dt.company_id, dtv.name, dtv.template_category, dtv.description,
		       COALESCE(dt.review_status, 'draft'),
		       COALESCE(dtv.deadline_rule, ''), COALESCE(dtv.periodicity, ''),
		       COALESCE(dtv.tags_json, '[]'),
		       dt.created_at, dt.updated_at
		FROM disclosure_types dt
		INNER JOIN disclosure_type_versions dtv ON dtv.type_id = dt.type_id AND dtv.version_no = dt.active_version_no
		WHERE dt.type_id = ? AND dt.company_id = ?`
	var resp disclosureapp.CompanyTemplateWriteResponse
	var tagsJSON string
	var createdAt, updatedAt time.Time
	err := r.db.QueryRowContext(ctx, q, typeID, companyID).Scan(
		&resp.TypeID, &resp.CompanyID, &resp.Name, &resp.TemplateCategory, &resp.Description,
		&resp.ReviewStatus, &resp.DeadlineRule, &resp.Periodicity,
		&tagsJSON, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "company template not found", nil)
	}
	if err != nil {
		return nil, fmt.Errorf("get company template: %w", err)
	}
	_ = json.Unmarshal([]byte(tagsJSON), &resp.Tags)
	resp.CreatedAt = createdAt.Format(time.RFC3339)
	resp.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &resp, nil
}

func (r *Repository) TransitionCompanyTemplateReviewStatus(ctx context.Context, companyID, typeID, newStatus, updatedBy string) error {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx,
		`UPDATE disclosure_types SET review_status = ?, updated_at = ? WHERE type_id = ? AND company_id = ?`,
		newStatus, now, typeID, companyID)
	if err != nil {
		return fmt.Errorf("transition review_status: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "company template not found", nil)
	}
	return nil
}
