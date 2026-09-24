package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	sqldriver "github.com/go-sql-driver/mysql"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type mysqlGlobalRecordsRepo struct {
	db  *sql.DB
	idg idgen.Generator
}

// NewMySQLGlobalRecordsRepository implements Repository against MySQL.
func NewMySQLGlobalRecordsRepository(db *sql.DB, idg idgen.Generator) Repository {
	if idg == nil {
		idg = idgen.UUIDv7Generator{}
	}
	return &mysqlGlobalRecordsRepo{db: db, idg: idg}
}

func (r *mysqlGlobalRecordsRepo) Create(ctx context.Context, rec GlobalRecord) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO cms_global_records (
			id, template_id, cycle_key, title, summary, content, status,
			template_version_no, created_by, updated_by, created_at, updated_at,
			cycle_start, due_date
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, rec.ID, rec.TemplateID, rec.CycleKey, rec.Title, nullIfEmpty(rec.Summary), rec.Content, rec.Status,
		nullInt(rec.TemplateVersionNo), nullIfEmpty(rec.CreatedBy), nullIfEmpty(rec.UpdatedBy),
		rec.CreatedAt.UTC(), rec.UpdatedAt.UTC(), nullTime(rec.CycleStart), nullTime(rec.DueDate))
	if err != nil {
		if isDupKey(err) {
			return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "global record already exists for template_id+cycle_key", nil)
		}
		return fmt.Errorf("cms_global_records insert: %w", err)
	}
	return nil
}

func (r *mysqlGlobalRecordsRepo) Update(ctx context.Context, rec GlobalRecord) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE cms_global_records SET
			cycle_key = ?, title = ?, summary = ?, content = ?, status = ?,
			template_version_no = ?, updated_by = ?, updated_at = ?,
			published_by = ?, published_at = ?, archived_by = ?, archived_at = ?,
			cycle_start = ?, due_date = ?
		WHERE id = ?
	`, rec.CycleKey, rec.Title, nullIfEmpty(rec.Summary), rec.Content, rec.Status,
		nullInt(rec.TemplateVersionNo), nullIfEmpty(rec.UpdatedBy), rec.UpdatedAt.UTC(),
		nullIfEmpty(rec.PublishedBy), nullTime(rec.PublishedAt), nullIfEmpty(rec.ArchivedBy), nullTime(rec.ArchivedAt),
		nullTime(rec.CycleStart), nullTime(rec.DueDate), rec.ID)
	if err != nil {
		if isDupKey(err) {
			return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "global record cycle_key conflict", nil)
		}
		return fmt.Errorf("cms_global_records update: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "global record not found", nil)
	}
	return nil
}

func (r *mysqlGlobalRecordsRepo) GetByID(ctx context.Context, id string) (*GlobalRecord, error) {
	return r.scanOne(ctx, `
		SELECT id, template_id, cycle_key, title, COALESCE(summary,''), content, status,
			template_version_no, COALESCE(created_by,''), COALESCE(updated_by,''),
			COALESCE(published_by,''), COALESCE(archived_by,''),
			created_at, updated_at, published_at, archived_at, cycle_start, due_date
		FROM cms_global_records WHERE id = ?
	`, id)
}

func (r *mysqlGlobalRecordsRepo) GetByTemplateCycle(ctx context.Context, templateID, cycleKey string) (*GlobalRecord, error) {
	return r.scanOne(ctx, `
		SELECT id, template_id, cycle_key, title, COALESCE(summary,''), content, status,
			template_version_no, COALESCE(created_by,''), COALESCE(updated_by,''),
			COALESCE(published_by,''), COALESCE(archived_by,''),
			created_at, updated_at, published_at, archived_at, cycle_start, due_date
		FROM cms_global_records WHERE template_id = ? AND cycle_key = ?
	`, templateID, cycleKey)
}

func (r *mysqlGlobalRecordsRepo) scanOne(ctx context.Context, q string, args ...any) (*GlobalRecord, error) {
	row := r.db.QueryRowContext(ctx, q, args...)
	var rec GlobalRecord
	var ver sql.NullInt64
	var publishedAt, archivedAt, cycleStart, dueDate sql.NullTime
	err := row.Scan(
		&rec.ID, &rec.TemplateID, &rec.CycleKey, &rec.Title, &rec.Summary, &rec.Content, &rec.Status,
		&ver, &rec.CreatedBy, &rec.UpdatedBy, &rec.PublishedBy, &rec.ArchivedBy,
		&rec.CreatedAt, &rec.UpdatedAt, &publishedAt, &archivedAt, &cycleStart, &dueDate,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if ver.Valid {
		v := int(ver.Int64)
		rec.TemplateVersionNo = &v
	}
	if publishedAt.Valid {
		t := publishedAt.Time.UTC()
		rec.PublishedAt = &t
	}
	if archivedAt.Valid {
		t := archivedAt.Time.UTC()
		rec.ArchivedAt = &t
	}
	if cycleStart.Valid {
		t := cycleStart.Time.UTC()
		rec.CycleStart = &t
	}
	if dueDate.Valid {
		t := dueDate.Time.UTC()
		rec.DueDate = &t
	}
	rec.CreatedAt = rec.CreatedAt.UTC()
	rec.UpdatedAt = rec.UpdatedAt.UTC()
	return &rec, nil
}

func (r *mysqlGlobalRecordsRepo) ListByTemplate(ctx context.Context, f ListGlobalRecordsFilter) (ListGlobalRecordsResult, error) {
	where := []string{"template_id = ?"}
	args := []any{f.TemplateID}
	if s := strings.TrimSpace(f.Status); s != "" {
		where = append(where, "status = ?")
		args = append(args, s)
	}
	if ck := strings.TrimSpace(f.CycleKey); ck != "" {
		where = append(where, "cycle_key = ?")
		args = append(args, ck)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		where = append(where, "title LIKE ?")
		args = append(args, "%"+q+"%")
	}
	clause := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cms_global_records WHERE "+clause, args...).Scan(&total); err != nil {
		return ListGlobalRecordsResult{}, err
	}

	page, pageSize := f.Page, f.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	listArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, template_id, cycle_key, title, COALESCE(summary,''), content, status,
			template_version_no, COALESCE(created_by,''), COALESCE(updated_by,''),
			COALESCE(published_by,''), COALESCE(archived_by,''),
			created_at, updated_at, published_at, archived_at, cycle_start, due_date
		FROM cms_global_records WHERE `+clause+`
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, listArgs...)
	if err != nil {
		return ListGlobalRecordsResult{}, err
	}
	defer rows.Close()

	items := make([]GlobalRecord, 0)
	for rows.Next() {
		var rec GlobalRecord
		var ver sql.NullInt64
		var publishedAt, archivedAt, cycleStart, dueDate sql.NullTime
		if err := rows.Scan(
			&rec.ID, &rec.TemplateID, &rec.CycleKey, &rec.Title, &rec.Summary, &rec.Content, &rec.Status,
			&ver, &rec.CreatedBy, &rec.UpdatedBy, &rec.PublishedBy, &rec.ArchivedBy,
			&rec.CreatedAt, &rec.UpdatedAt, &publishedAt, &archivedAt, &cycleStart, &dueDate,
		); err != nil {
			return ListGlobalRecordsResult{}, err
		}
		if ver.Valid {
			v := int(ver.Int64)
			rec.TemplateVersionNo = &v
		}
		if publishedAt.Valid {
			t := publishedAt.Time.UTC()
			rec.PublishedAt = &t
		}
		if archivedAt.Valid {
			t := archivedAt.Time.UTC()
			rec.ArchivedAt = &t
		}
		if cycleStart.Valid {
			t := cycleStart.Time.UTC()
			rec.CycleStart = &t
		}
		if dueDate.Valid {
			t := dueDate.Time.UTC()
			rec.DueDate = &t
		}
		rec.CreatedAt = rec.CreatedAt.UTC()
		rec.UpdatedAt = rec.UpdatedAt.UTC()
		items = append(items, rec)
	}
	return ListGlobalRecordsResult{Items: items, Page: page, PageSize: pageSize, Total: total}, rows.Err()
}

func (r *mysqlGlobalRecordsRepo) CountCompaniesByCMSRecord(ctx context.Context, cmsRecordID string) (*CompanyCounts, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT status, COUNT(*) FROM disclosure_records
		WHERE cms_record_id = ?
		GROUP BY status
	`, cmsRecordID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := &CompanyCounts{}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		counts.Total += n
		switch MapCompanyStatusForCounts(status) {
		case "not_started":
			counts.NotStarted += n
		case "pending_review":
			counts.PendingReview += n
		case "approved":
			counts.Approved += n
		case "published":
			counts.Published += n
		case "completed":
			counts.Completed += n
		case "failed":
			counts.Failed += n
		default:
			counts.InProgress += n
		}
	}
	return counts, rows.Err()
}

func (r *mysqlGlobalRecordsRepo) ListCompanyChildren(ctx context.Context, cmsRecordID string, status string, page, pageSize int) ([]CompanyChild, int, error) {
	where := []string{"cms_record_id = ?"}
	args := []any{cmsRecordID}
	if s := strings.TrimSpace(status); s != "" {
		where = append(where, "status = ?")
		args = append(args, s)
	}
	clause := strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM disclosure_records WHERE "+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	listArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT company_id, record_id, title, status, planned_date, submitted_at, completed_at, updated_at
		FROM disclosure_records WHERE `+clause+`
		ORDER BY updated_at DESC
		LIMIT ? OFFSET ?
	`, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]CompanyChild, 0)
	for rows.Next() {
		var c CompanyChild
		var planned sql.NullString
		var submitted, completed sql.NullTime
		if err := rows.Scan(&c.CompanyID, &c.RecordID, &c.Title, &c.Status, &planned, &submitted, &completed, &c.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if planned.Valid && planned.String != "" {
			if t, err := time.Parse("2006-01-02", planned.String); err == nil {
				tt := t.UTC()
				c.PlannedDate = &tt
			}
		}
		if submitted.Valid {
			t := submitted.Time.UTC()
			c.SubmittedAt = &t
		}
		if completed.Valid {
			t := completed.Time.UTC()
			c.CompletedAt = &t
		}
		c.UpdatedAt = c.UpdatedAt.UTC()
		out = append(out, c)
	}
	return out, total, rows.Err()
}

func (r *mysqlGlobalRecordsRepo) FindCompanyRecordLink(ctx context.Context, cmsRecordID, companyID string) (string, bool, error) {
	var recordID string
	err := r.db.QueryRowContext(ctx, `
		SELECT record_id FROM disclosure_records
		WHERE cms_record_id = ? AND company_id = ?
		LIMIT 1
	`, cmsRecordID, companyID).Scan(&recordID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return recordID, true, nil
}

func (r *mysqlGlobalRecordsRepo) CreateCompanyProcessingRecord(ctx context.Context, cmsRecordID, companyID, typeID, title, summary, content, createdBy string) (string, error) {
	recordID := r.idg.NewUUID()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO disclosure_records (
			record_id, company_id, type_id, cms_record_id, department_id, title, summary, content,
			status, attachments_json, created_by, updated_by
		) VALUES (?, ?, NULLIF(?, ''), ?, 'general', ?, ?, ?, 'NotStarted', CAST('[]' AS JSON), ?, ?)
	`, recordID, companyID, typeID, cmsRecordID, title, summary, content, createdBy, createdBy)
	if err != nil {
		if isDupKey(err) {
			return "", perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "company processing record already exists", nil)
		}
		return "", fmt.Errorf("materialize disclosure insert: %w", err)
	}
	return recordID, nil
}

func (r *mysqlGlobalRecordsRepo) EvaluateEligibility(ctx context.Context, templateID, cycleKey, cmsRecordID string) ([]EligibilityDecision, error) {
	freq, slot, _ := ParseCycleKeySlot(cycleKey)
	rulesJSON, fromMode, fromSlot, applicableTo, err := r.loadTemplateEligibilityConfig(ctx, templateID)
	if err != nil {
		return nil, err
	}
	var rules *applicability.TemplateApplicabilityRules
	if len(rulesJSON) > 0 && string(rulesJSON) != "null" {
		parsed, pErr := applicability.ParseRulesJSON(rulesJSON)
		if pErr == nil {
			rules = parsed
		}
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT c.company_id, COALESCE(c.company_name, c.company_id), LOWER(COALESCE(c.status,'')),
			COALESCE(c.is_listed, 0), COALESCE(c.is_large_public, 0), COALESCE(c.is_non_large_public, 0),
			COALESCE(c.has_subsidiaries, 0), COALESCE(c.has_subordinate_accounting_units, 0),
			COALESCE(c.business_sector, ''), c.business_sectors
		FROM companies c
		ORDER BY c.company_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type companyRow struct {
		id, name, status string
		profile          applicability.CompanyApplicabilityProfile
	}
	companies := make([]companyRow, 0)
	companyIDs := make([]string, 0)
	for rows.Next() {
		var cr companyRow
		var isListed, isLarge, isNonLarge, hasSub, hasAcct int
		var sector string
		var sectorsJSON []byte
		if err := rows.Scan(&cr.id, &cr.name, &cr.status, &isListed, &isLarge, &isNonLarge, &hasSub, &hasAcct, &sector, &sectorsJSON); err != nil {
			return nil, err
		}
		cr.profile = applicability.ProfileFromCompanyDetail(
			isListed == 1, isLarge == 1, isNonLarge == 1, hasSub == 1, hasAcct == 1, sector, sectorsJSON,
		)
		companies = append(companies, cr)
		companyIDs = append(companyIDs, cr.id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	prefDisabled := map[string]bool{}
	if len(companyIDs) > 0 {
		prefs, pErr := r.loadAutoCreateDisabled(ctx, templateID)
		if pErr != nil {
			return nil, pErr
		}
		prefDisabled = prefs
	}

	entitled := map[string]string{}
	if len(companyIDs) > 0 {
		ent, eErr := r.loadEntitlements(ctx, companyIDs)
		if eErr != nil {
			return nil, eErr
		}
		entitled = ent
	}

	links := map[string]string{}
	if cmsRecordID != "" {
		linkRows, lErr := r.db.QueryContext(ctx, `
			SELECT company_id, record_id FROM disclosure_records WHERE cms_record_id = ?
		`, cmsRecordID)
		if lErr != nil {
			return nil, lErr
		}
		for linkRows.Next() {
			var cid, rid string
			if err := linkRows.Scan(&cid, &rid); err != nil {
				linkRows.Close()
				return nil, err
			}
			links[cid] = rid
		}
		linkRows.Close()
	}

	loc := AsiaHCM()
	now := time.Now().In(loc)
	out := make([]EligibilityDecision, 0, len(companies))
	for _, cr := range companies {
		d := EligibilityDecision{
			CompanyID:         cr.id,
			CompanyName:       cr.name,
			Active:            cr.status == "active",
			AutoCreateEnabled: !prefDisabled[cr.id],
			ApplicableFromOK:  true,
			ApplicableToOK:    true,
		}
		if rid, ok := links[cr.id]; ok {
			d.AlreadyMaterialized = true
			d.ExistingCompanyRecID = rid
		}
		if planStatus, ok := entitled[cr.id]; ok {
			d.EntitlementStatus = planStatus
		} else {
			d.EntitlementStatus = "NONE"
		}

		switch {
		case !d.Active:
			d.ReasonCode = ReasonCompanyInactive
		case d.EntitlementStatus == "NONE" || d.EntitlementStatus == "EXPIRED" || d.EntitlementStatus == "CANCELLED":
			d.ReasonCode = ReasonEntitlementMissing
		case !applicability.IsApplicable(rules, cr.profile, true):
			d.ApplicabilityMatch = false
			d.ReasonCode = ReasonApplicabilityNotMatched
		case !d.AutoCreateEnabled:
			d.ReasonCode = ReasonAutoCreateDisabled
		default:
			d.ApplicabilityMatch = true
			if freq != "" && slot != "" {
				fromOK, fromDecision, fromErr := disclosureapp.EvaluateApplicableFromEligibility(freq, slot, fromMode, fromSlot)
				if fromErr != nil || !fromOK {
					d.ApplicableFromOK = false
					d.ReasonCode = ReasonNotYetApplicable
					if fromDecision != "" {
						d.ReasonMessage = fromDecision
					}
				} else {
					// Approximate occurrence T as start of slot for applicable_to.
					occT, tErr := disclosureapp.SlotStartDateForEligibility(freq, slot, loc)
					if tErr != nil {
						occT = now
					}
					toOK, toDecision, toErr := disclosureapp.EvaluateApplicableToEligibility(occT, applicableTo, loc)
					if toErr != nil || !toOK {
						d.ApplicableToOK = false
						d.ReasonCode = ReasonNoLongerApplicable
						if toDecision != "" {
							d.ReasonMessage = toDecision
						}
					}
				}
			}
			if d.ReasonCode == "" {
				if d.AlreadyMaterialized {
					d.ReasonCode = ReasonAlreadyMaterialized
				} else {
					d.Eligible = true
					d.ReasonCode = ReasonEligible
				}
			}
		}
		if d.ReasonMessage == "" {
			d.ReasonMessage = ReasonMessageVI(d.ReasonCode)
		}
		if d.AlreadyMaterialized && d.ReasonCode != ReasonEligible {
			// Prefer ALREADY_MATERIALIZED when link exists and otherwise eligible
			if d.ReasonCode == ReasonEligible || d.Eligible {
				d.Eligible = false
				d.ReasonCode = ReasonAlreadyMaterialized
				d.ReasonMessage = ReasonMessageVI(ReasonAlreadyMaterialized)
			}
		}
		out = append(out, d)
	}
	return out, nil
}

func (r *mysqlGlobalRecordsRepo) loadTemplateEligibilityConfig(ctx context.Context, templateID string) (rulesJSON []byte, fromMode, fromSlot, applicableTo string, err error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(v.applicability_rules_json, CAST('null' AS JSON)),
			COALESCE(JSON_UNQUOTE(JSON_EXTRACT(v.deadline_config_json, '$.applicable_from_mode')), ''),
			COALESCE(JSON_UNQUOTE(JSON_EXTRACT(v.deadline_config_json, '$.applicable_from_slot')), ''),
			COALESCE(JSON_UNQUOTE(JSON_EXTRACT(v.deadline_config_json, '$.applicable_to')), '')
		FROM disclosure_types t
		LEFT JOIN disclosure_type_versions v
			ON v.type_id = t.type_id AND v.version_no = t.active_version_no AND t.active_version_no > 0
		WHERE t.type_id = ?
	`, templateID)
	err = row.Scan(&rulesJSON, &fromMode, &fromSlot, &applicableTo)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", "", "", nil
	}
	if err != nil {
		return nil, "", "", "", err
	}
	if fromMode == "null" {
		fromMode = ""
	}
	if fromSlot == "null" {
		fromSlot = ""
	}
	if applicableTo == "null" {
		applicableTo = ""
	}
	return rulesJSON, fromMode, fromSlot, applicableTo, nil
}

func (r *mysqlGlobalRecordsRepo) loadAutoCreateDisabled(ctx context.Context, templateID string) (map[string]bool, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT company_id FROM company_type_preferences
		WHERE type_id = ? AND auto_create_enabled = 0
	`, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

func (r *mysqlGlobalRecordsRepo) loadEntitlements(ctx context.Context, companyIDs []string) (map[string]string, error) {
	out := map[string]string{}
	if len(companyIDs) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(companyIDs))
	args := make([]any, 0, len(companyIDs)+1)
	for i, id := range companyIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	now := time.Now().UTC()
	args = append(args, now, now)
	q := fmt.Sprintf(`
		SELECT company_id, status FROM company_subscriptions
		WHERE company_id IN (%s)
		  AND status IN ('ACTIVE','TRIAL')
		  AND effective_from <= ?
		  AND (expires_at IS NULL OR expires_at > ?)
		ORDER BY company_id, FIELD(status,'ACTIVE','TRIAL'), effective_from DESC
	`, strings.Join(placeholders, ","))
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, status string
		if err := rows.Scan(&cid, &status); err != nil {
			return nil, err
		}
		if _, ok := out[cid]; !ok {
			out[cid] = status
		}
	}
	return out, rows.Err()
}

func (r *mysqlGlobalRecordsRepo) SaveMaterializationRun(ctx context.Context, run MaterializeResult, actor Actor, cmsRecordID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	dry := 0
	if run.DryRun {
		dry = 1
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO cms_materialization_runs (
			run_id, cms_record_id, actor_user_id, actor_membership_id, mode, dry_run, status,
			requested_count, created_count, exists_count, skipped_count, failed_count,
			started_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, run.RunID, cmsRecordID, nullIfEmpty(actor.UserID), nullIfEmpty(actor.MembershipID), run.Mode, dry, run.Status,
		run.Requested, run.Created, run.Exists, run.Skipped, run.Failed, now, now)
	if err != nil {
		return err
	}
	for _, item := range run.Results {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO cms_materialization_items (
				run_id, company_id, outcome, company_record_id, error_code, error_message, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?)
		`, run.RunID, item.CompanyID, item.Outcome, nullIfEmpty(item.CompanyRecordID),
			nullIfEmpty(item.ErrorCode), nullIfEmpty(item.ErrorMessage), now)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *mysqlGlobalRecordsRepo) GetTemplateInfo(ctx context.Context, templateID string) (*TemplateInfo, error) {
	// template_category / periodicity live on active version when present.
	row := r.db.QueryRowContext(ctx, `
		SELECT t.type_id,
			CASE WHEN t.company_id IS NULL OR t.company_id = '' THEN 'global' ELSE 'company' END AS scope,
			COALESCE(t.active_version_no, 0),
			COALESCE(t.status, ''),
			COALESCE(t.review_status, ''),
			COALESCE(v.template_category, ''),
			COALESCE(JSON_UNQUOTE(JSON_EXTRACT(v.deadline_config_json, '$.frequency_unit')), '')
		FROM disclosure_types t
		LEFT JOIN disclosure_type_versions v
			ON v.type_id = t.type_id AND v.version_no = t.active_version_no AND t.active_version_no > 0
		WHERE t.type_id = ?
	`, templateID)
	var t TemplateInfo
	err := row.Scan(&t.TypeID, &t.Scope, &t.ActiveVersionNo, &t.PortalState, &t.ReviewStatus, &t.TemplateCategory, &t.Periodicity)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func nullInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullTime(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UTC()
}

func isDupKey(err error) bool {
	var me *sqldriver.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}
