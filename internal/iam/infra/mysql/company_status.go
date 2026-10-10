package mysql

import (
	"context"
	"database/sql"
	"errors"
)

// CompanyStatusReader reads companies.status by primary key (token inspection).
type CompanyStatusReader struct {
	db *sql.DB
}

func NewCompanyStatusReader(db *sql.DB) *CompanyStatusReader {
	return &CompanyStatusReader{db: db}
}

// CompanyStatus returns the company's operational status, or "" when the company does not exist.
func (r *CompanyStatusReader) CompanyStatus(ctx context.Context, companyID string) (string, error) {
	var status string
	err := r.db.QueryRowContext(ctx, `SELECT status FROM companies WHERE company_id = ?`, companyID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return status, err
}
