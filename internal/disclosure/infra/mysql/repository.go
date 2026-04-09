package mysql

import (
	"context"
	"database/sql"
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
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO disclosure_records (
			record_id, company_id, department_id, title, content, status, created_by, updated_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, rec.RecordID, rec.CompanyID, rec.DepartmentID, rec.Title, rec.Content, rec.Status, rec.CreatedBy, rec.UpdatedBy)
	if err != nil {
		return nil, fmt.Errorf("disclosure insert: %w", err)
	}
	cp := rec
	return &cp, nil
}

func (r *Repository) Update(ctx context.Context, rec disclosureapp.RecordDTO) (*disclosureapp.RecordDTO, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE disclosure_records
		SET department_id = ?, title = ?, content = ?, status = ?, updated_by = ?
		WHERE record_id = ? AND company_id = ?
	`, rec.DepartmentID, rec.Title, rec.Content, rec.Status, rec.UpdatedBy, rec.RecordID, rec.CompanyID)
	if err != nil {
		return nil, fmt.Errorf("disclosure update: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, perr.NewHTTPError(404, perr.CodeInvalidRequest, "record not found", nil)
	}
	cp := rec
	return &cp, nil
}

func (r *Repository) FindByID(ctx context.Context, companyID, recordID string) (*disclosureapp.RecordDTO, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT record_id, company_id, department_id, title, content, status, created_by, updated_by
		FROM disclosure_records WHERE company_id = ? AND record_id = ?
	`, companyID, recordID)
	var rec disclosureapp.RecordDTO
	if err := row.Scan(&rec.RecordID, &rec.CompanyID, &rec.DepartmentID, &rec.Title, &rec.Content, &rec.Status, &rec.CreatedBy, &rec.UpdatedBy); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(404, perr.CodeInvalidRequest, "record not found", nil)
		}
		return nil, err
	}
	return &rec, nil
}

func (r *Repository) List(ctx context.Context, companyID string) ([]disclosureapp.RecordDTO, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT record_id, company_id, department_id, title, content, status, created_by, updated_by
		FROM disclosure_records WHERE company_id = ? ORDER BY created_at DESC
	`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []disclosureapp.RecordDTO
	for rows.Next() {
		var rec disclosureapp.RecordDTO
		if err := rows.Scan(&rec.RecordID, &rec.CompanyID, &rec.DepartmentID, &rec.Title, &rec.Content, &rec.Status, &rec.CreatedBy, &rec.UpdatedBy); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}
