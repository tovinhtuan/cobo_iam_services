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
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			dr.company_id,
			dr.record_id,
			COALESCE(dr.type_id, ''),
			dr.title,
			COALESCE(dtv.name, ''),
			COALESCE((
				SELECT TRIM(SUBSTRING_INDEX(ah.change_note, '\n', 1))
				FROM ad_hoc_proposals ah
				WHERE ah.company_id = dr.company_id
				  AND ah.record_id = dr.record_id
				  AND ah.status = 'approved'
				ORDER BY ah.updated_at DESC
				LIMIT 1
			), ''),
			dr.status,
			COALESCE(DATE_FORMAT(dr.planned_date, '%Y-%m-%d'), ''),
			COALESCE(wi.workflow_instance_id, ''),
			COALESCE(wi.current_step_code, ''),
			wi.snapshot_json,
			COALESCE((
				SELECT DATE_FORMAT(
					COALESCE(
						ah.final_deadline_date,
						CASE
							WHEN ah.proposed_t0_date IS NOT NULL
								AND ah.proposed_deadline_days IS NOT NULL
								AND ah.proposed_deadline_days > 0
							THEN DATE_ADD(DATE(ah.proposed_t0_date), INTERVAL ah.proposed_deadline_days DAY)
							ELSE NULL
						END,
						CASE
							WHEN ah.proposed_deadline_date IS NOT NULL
								AND ah.proposed_deadline_date >= '2000-01-01'
							THEN DATE(ah.proposed_deadline_date)
							ELSE NULL
						END
					),
					'%Y-%m-%d'
				)
				FROM ad_hoc_proposals ah
				WHERE ah.company_id = dr.company_id
				  AND ah.record_id = dr.record_id
				  AND ah.status = 'approved'
				ORDER BY ah.updated_at DESC
				LIMIT 1
			), ''),
			COALESCE(JSON_UNQUOTE(JSON_EXTRACT(dtv.deadline_config_json, '$.template_category')), ''),
			COALESCE(dtv.deadline_config_json, JSON_OBJECT()),
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
		var snapshot sql.NullString
		var deadlineConfig sql.NullString
		var confirmedBy sql.NullString
		var confirmedAt sql.NullTime
		if err := rows.Scan(
			&row.CompanyID,
			&row.RecordID,
			&row.TypeID,
			&row.Title,
			&row.TypeName,
			&row.AdHocTitleLine,
			&row.RecordStatus,
			&row.PlannedDate,
			&row.WorkflowInstanceID,
			&row.CurrentStepCode,
			&snapshot,
			&row.AdHocDeadlineDate,
			&row.TemplateCategory,
			&deadlineConfig,
			&confirmedBy,
			&confirmedAt,
		); err != nil {
			return nil, err
		}
		if snapshot.Valid && snapshot.String != "" && snapshot.String != "null" {
			row.SnapshotJSON = []byte(snapshot.String)
		}
		if deadlineConfig.Valid && deadlineConfig.String != "" && deadlineConfig.String != "null" {
			row.DeadlineConfigJSON = []byte(deadlineConfig.String)
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
