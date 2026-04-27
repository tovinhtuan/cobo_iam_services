package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

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
