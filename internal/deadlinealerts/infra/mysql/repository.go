package mysql

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	deadlinealertsapp "github.com/cobo/cobo_iam_services/internal/deadlinealerts/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	disclosuremysql "github.com/cobo/cobo_iam_services/internal/disclosure/infra/mysql"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type Repository struct {
	db         *sql.DB
	disclosure *disclosuremysql.Repository
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, disclosure: disclosuremysql.NewRepository(db)}
}

func (r *Repository) ListRows(ctx context.Context, companyID string) ([]deadlinealertsapp.AlertRow, error) {
	deptByRecord, err := r.listCurrentStepDepartments(ctx, companyID)
	if err != nil {
		return nil, err
	}
	adHocByRecord, err := r.listLatestAdHocMeta(ctx, companyID)
	if err != nil {
		return nil, err
	}

	// Keep SQL compact: DEV MySQL may use max_allowed_packet=2048; avoid large correlated subqueries.
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			dr.company_id,
			dr.record_id,
			COALESCE(dr.type_id, ''),
			dr.title,
			COALESCE(dtv.name, ''),
			dr.status,
			COALESCE(DATE_FORMAT(dr.planned_date, '%Y-%m-%d'), ''),
			COALESCE(wi.workflow_instance_id, ''),
			COALESCE(wi.current_step_code, ''),
			COALESCE(JSON_UNQUOTE(JSON_EXTRACT(dtv.deadline_config_json, '$.template_category')), ''),
			COALESCE(dac.confirmed_by, ''),
			dac.confirmed_at
		FROM disclosure_records dr
		LEFT JOIN workflow_instances wi ON wi.company_id = dr.company_id
			AND wi.workflow_instance_id = (
				SELECT wi2.workflow_instance_id
				FROM workflow_instances wi2
				WHERE wi2.company_id = dr.company_id AND wi2.record_id = dr.record_id
				ORDER BY wi2.workflow_instance_id ASC
				LIMIT 1
			)
		LEFT JOIN disclosure_types dt ON dt.type_id = dr.type_id
		LEFT JOIN disclosure_type_versions dtv ON dtv.type_id = dt.type_id AND dtv.version_no = dt.active_version_no
		LEFT JOIN deadline_alert_confirmations dac ON dac.company_id = dr.company_id
			AND dac.record_id = dr.record_id
		WHERE dr.company_id = ?
		  AND LOWER(TRIM(dr.status)) <> 'draft'
		ORDER BY dr.created_at DESC
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []deadlinealertsapp.AlertRow
	for rows.Next() {
		var row deadlinealertsapp.AlertRow
		var confirmedBy sql.NullString
		var confirmedAt sql.NullTime
		if err := rows.Scan(
			&row.CompanyID,
			&row.RecordID,
			&row.TypeID,
			&row.Title,
			&row.TypeName,
			&row.RecordStatus,
			&row.PlannedDate,
			&row.WorkflowInstanceID,
			&row.CurrentStepCode,
			&row.TemplateCategory,
			&confirmedBy,
			&confirmedAt,
		); err != nil {
			return nil, err
		}
		row.CurrentStepDepartment = deptByRecord[row.RecordID]
		if meta, ok := adHocByRecord[row.RecordID]; ok {
			row.AdHocTitleLine = meta.titleLine
			row.AdHocDeadlineDate = meta.dueDate
		}
		if confirmedBy.Valid {
			row.ConfirmedBy = strings.TrimSpace(confirmedBy.String)
		}
		if confirmedAt.Valid {
			ts := confirmedAt.Time.UTC()
			row.ConfirmedAt = &ts
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

type adHocMeta struct {
	titleLine string
	dueDate   string
}

func (r *Repository) listLatestAdHocMeta(ctx context.Context, companyID string) (map[string]adHocMeta, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			record_id,
			COALESCE(change_note, ''),
			final_deadline_date,
			proposed_t0_date,
			proposed_deadline_days,
			proposed_deadline_date,
			updated_at
		FROM ad_hoc_proposals
		WHERE company_id = ? AND status = 'approved'
		ORDER BY updated_at DESC
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]adHocMeta{}
	for rows.Next() {
		var recordID string
		var changeNote string
		var finalDue sql.NullTime
		var proposedT0 sql.NullTime
		var proposedDays sql.NullInt64
		var proposedDue sql.NullTime
		var updatedAt time.Time
		if err := rows.Scan(&recordID, &changeNote, &finalDue, &proposedT0, &proposedDays, &proposedDue, &updatedAt); err != nil {
			return nil, err
		}
		if _, exists := out[recordID]; exists {
			continue
		}
		titleLine := strings.TrimSpace(strings.Split(changeNote, "\n")[0])
		out[recordID] = adHocMeta{
			titleLine: titleLine,
			dueDate:   formatAdHocDueDate(finalDue, proposedT0, proposedDays, proposedDue),
		}
	}
	return out, rows.Err()
}

func formatAdHocDueDate(finalDue sql.NullTime, proposedT0 sql.NullTime, proposedDays sql.NullInt64, proposedDue sql.NullTime) string {
	if finalDue.Valid {
		return finalDue.Time.UTC().Format("2006-01-02")
	}
	if proposedT0.Valid && proposedDays.Valid && proposedDays.Int64 > 0 {
		d := proposedT0.Time.UTC().AddDate(0, 0, int(proposedDays.Int64))
		return d.Format("2006-01-02")
	}
	if proposedDue.Valid && !proposedDue.Time.Before(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) {
		return proposedDue.Time.UTC().Format("2006-01-02")
	}
	return ""
}

func (r *Repository) listCurrentStepDepartments(ctx context.Context, companyID string) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT wi.record_id, wi.current_step_code, wi.snapshot_json
		FROM workflow_instances wi
		INNER JOIN disclosure_records dr ON dr.company_id = wi.company_id
			AND dr.record_id = wi.record_id
		WHERE wi.company_id = ?
		  AND LOWER(TRIM(dr.status)) <> 'draft'
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var recordID, stepCode string
		var snapshot []byte
		if err := rows.Scan(&recordID, &stepCode, &snapshot); err != nil {
			return nil, err
		}
		depts := deadlinealertsapp.ActiveDepartmentsFromSnapshot(stepCode, snapshot)
		if len(depts) == 0 {
			continue
		}
		out[recordID] = depts[0]
	}
	return out, rows.Err()
}

func (r *Repository) GetCompanyDeadlineContext(ctx context.Context, companyID string) (disclosureapp.CompanyDeadlineContext, error) {
	return r.disclosure.GetCompanyDeadlineContext(ctx, companyID)
}

func (r *Repository) GetCompanyTypeDeadlineContext(ctx context.Context, companyID, typeID string) (disclosureapp.CompanyDeadlineContext, error) {
	return r.disclosure.GetCompanyTypeDeadlineContext(ctx, companyID, typeID)
}

func (r *Repository) GetTypeDeadlineConfig(ctx context.Context, companyID, typeID string) (*disclosureapp.TemplateDeadlineConfig, error) {
	_ = companyID
	_, cfg, err := r.disclosure.GetActiveVersionDeadlineConfig(ctx, typeID)
	return cfg, err
}

func (r *Repository) HasDisclosureRecord(ctx context.Context, companyID, recordID string) (bool, error) {
	var exists int
	if err := r.db.QueryRowContext(
		ctx,
		`SELECT 1 FROM disclosure_records WHERE company_id = ? AND record_id = ? LIMIT 1`,
		companyID,
		recordID,
	).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *Repository) ConfirmDeadlineAlert(
	ctx context.Context,
	companyID,
	recordID,
	confirmedBy,
	note,
	idempotencyKey string,
	at time.Time,
) error {
	note = strings.TrimSpace(note)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey != "" {
		var existingRecordID string
		err := r.db.QueryRowContext(
			ctx,
			`SELECT record_id FROM deadline_alert_confirmations WHERE company_id = ? AND idempotency_key = ? LIMIT 1`,
			companyID,
			idempotencyKey,
		).Scan(&existingRecordID)
		if err == nil {
			if strings.TrimSpace(existingRecordID) == recordID {
				return nil
			}
			return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "idempotency key conflict", nil)
		}
		if err != sql.ErrNoRows {
			return err
		}
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO deadline_alert_confirmations
			(company_id, record_id, confirmed_by, confirmed_at, confirm_note, idempotency_key)
		VALUES (?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''))
		ON DUPLICATE KEY UPDATE
			confirmed_by = VALUES(confirmed_by),
			confirmed_at = VALUES(confirmed_at),
			confirm_note = VALUES(confirm_note),
			idempotency_key = COALESCE(deadline_alert_confirmations.idempotency_key, VALUES(idempotency_key)),
			updated_at = CURRENT_TIMESTAMP(3)
	`, companyID, recordID, confirmedBy, at.UTC(), note, idempotencyKey)
	return err
}
