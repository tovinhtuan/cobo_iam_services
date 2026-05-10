package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	iamregmysql "github.com/cobo/cobo_iam_services/internal/iam/registrationmysql"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func (r *AdminRepository) CreateStandaloneCompany(ctx context.Context, displayName string, boot caapp.CreateCompanyBootstrap) (string, string, error) {
	p := iamregmysql.CompanyBootstrapProfile{
		VerificationStatus: boot.VerificationStatus,
		TaxCode:              boot.TaxCode,
		RegistrationNumber: boot.RegistrationNumber,
		Address:              boot.Address,
		Phone:                boot.Phone,
		ContactEmail:         boot.ContactEmail,
		RepresentativeName:   boot.RepresentativeName,
	}
	id, code, err := iamregmysql.CreateStandaloneCompany(ctx, r.db, displayName, p)
	return id, code, mapMySQLSchemaErr(err)
}

func (r *AdminRepository) ListCompaniesPlatform(ctx context.Context, req caapp.ListPlatformCompaniesRequest) (*caapp.ListPlatformCompaniesResult, error) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	limit := req.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	sortCol := "c.created_at"
	switch strings.ToLower(strings.TrimSpace(req.SortBy)) {
	case "name", "company_name":
		sortCol = "c.company_name"
	case "updated_at":
		sortCol = "c.updated_at"
	case "created_at":
		sortCol = "c.created_at"
	}
	order := "DESC"
	if strings.EqualFold(strings.TrimSpace(req.SortOrder), "asc") {
		order = "ASC"
	}

	qkw := strings.TrimSpace(req.Q)
	like := "%" + qkw + "%"
	status := strings.TrimSpace(req.Status)
	vstat := strings.TrimSpace(req.VerificationStatus)

	var where []string
	var args []any
	if qkw != "" {
		where = append(where, `(c.company_name LIKE ? OR c.company_code LIKE ? OR COALESCE(c.tax_code,'') LIKE ? OR COALESCE(c.registration_number,'') LIKE ?)`)
		args = append(args, like, like, like, like)
	}
	if status != "" {
		where = append(where, `LOWER(TRIM(c.status)) = LOWER(?)`)
		args = append(args, status)
	}
	if vstat != "" {
		where = append(where, `LOWER(TRIM(c.verification_status)) = LOWER(?)`)
		args = append(args, vstat)
	}
	whereSQL := ""
	if len(where) > 0 {
		whereSQL = "WHERE " + strings.Join(where, " AND ")
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM companies c %s`, whereSQL)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	argsWithPage := append(append([]any{}, args...), limit, offset)
	query := fmt.Sprintf(`
		SELECT c.company_id, c.company_code, c.company_name, c.status, c.verification_status,
			COALESCE(c.tax_code, ''), COALESCE(c.registration_number, ''),
			(SELECT COUNT(*) FROM memberships m WHERE m.company_id = c.company_id AND LOWER(TRIM(m.membership_status)) = 'active'),
			c.created_at, c.updated_at
		FROM companies c
		%s
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, whereSQL, sortCol, order)

	rows, err := r.db.QueryContext(ctx, query, argsWithPage...)
	if err != nil {
		return nil, mapMySQLSchemaErr(err)
	}
	defer rows.Close()
	var items []caapp.PlatformCompanySummary
	for rows.Next() {
		var s caapp.PlatformCompanySummary
		var created, updated time.Time
		if err := rows.Scan(
			&s.CompanyID, &s.CompanyCode, &s.CompanyName, &s.Status, &s.VerificationStatus,
			&s.TaxCode, &s.RegistrationNumber, &s.MemberCount, &created, &updated,
		); err != nil {
			return nil, mapMySQLSchemaErr(err)
		}
		s.CreatedAtRFC3339 = created.UTC().Format(time.RFC3339)
		s.UpdatedAtRFC3339 = updated.UTC().Format(time.RFC3339)
		items = append(items, s)
	}
	if err := rows.Err(); err != nil {
		return nil, mapMySQLSchemaErr(err)
	}
	return &caapp.ListPlatformCompaniesResult{Items: items, Total: total}, nil
}

func (r *AdminRepository) GetCompanyPlatform(ctx context.Context, companyID string) (*caapp.PlatformCompanyDetail, error) {
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id is required", nil)
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT company_id, company_code, company_name, status, verification_status,
			COALESCE(tax_code, ''), COALESCE(registration_number, ''),
			COALESCE(address, ''), COALESCE(phone, ''), COALESCE(contact_email, ''), COALESCE(representative_name, ''),
			created_at, updated_at
		FROM companies WHERE company_id = ?
	`, companyID)
	var d caapp.PlatformCompanyDetail
	var created, updated time.Time
	if err := row.Scan(
		&d.CompanyID, &d.CompanyCode, &d.CompanyName, &d.Status, &d.VerificationStatus,
		&d.TaxCode, &d.RegistrationNumber,
		&d.Address, &d.Phone, &d.ContactEmail, &d.RepresentativeName,
		&created, &updated,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "company not found", err)
		}
		return nil, err
	}
	d.CreatedAtRFC3339 = created.UTC().Format(time.RFC3339)
	d.UpdatedAtRFC3339 = updated.UTC().Format(time.RFC3339)

	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memberships WHERE company_id = ? AND LOWER(TRIM(membership_status)) = 'active'`, companyID).Scan(&d.MemberCount); err != nil {
		return nil, mapMySQLSchemaErr(err)
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM disclosure_records WHERE company_id = ?`, companyID).Scan(&d.DisclosureCount); err != nil {
		return nil, mapMySQLSchemaErr(err)
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM disclosure_types WHERE company_id = ? AND LOWER(TRIM(status)) = 'active'`, companyID).Scan(&d.TemplateCount); err != nil {
		return nil, mapMySQLSchemaErr(err)
	}

	return &d, nil
}

func (r *AdminRepository) UpdateCompanyPlatform(ctx context.Context, req caapp.UpdatePlatformCompanyRequest) error {
	companyID := strings.TrimSpace(req.CompanyID)
	if companyID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id is required", nil)
	}
	var sets []string
	var args []any
	if req.CompanyName != nil {
		v := strings.TrimSpace(*req.CompanyName)
		if v == "" {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_name cannot be empty", nil)
		}
		sets = append(sets, "company_name = ?")
		args = append(args, v)
	}
	if req.TaxCode != nil {
		sets = append(sets, "tax_code = NULLIF(TRIM(?), '')")
		args = append(args, *req.TaxCode)
	}
	if req.RegistrationNumber != nil {
		sets = append(sets, "registration_number = NULLIF(TRIM(?), '')")
		args = append(args, *req.RegistrationNumber)
	}
	if req.Address != nil {
		sets = append(sets, "address = NULLIF(TRIM(?), '')")
		args = append(args, *req.Address)
	}
	if req.Phone != nil {
		sets = append(sets, "phone = NULLIF(TRIM(?), '')")
		args = append(args, *req.Phone)
	}
	if req.ContactEmail != nil {
		sets = append(sets, "contact_email = NULLIF(TRIM(?), '')")
		args = append(args, *req.ContactEmail)
	}
	if req.RepresentativeName != nil {
		sets = append(sets, "representative_name = NULLIF(TRIM(?), '')")
		args = append(args, *req.RepresentativeName)
	}
	if req.VerificationStatus != nil {
		sets = append(sets, "verification_status = ?")
		args = append(args, strings.TrimSpace(*req.VerificationStatus))
	}
	if len(sets) == 0 {
		return nil
	}
	query := "UPDATE companies SET " + strings.Join(sets, ", ") + " WHERE company_id = ?"
	args = append(args, companyID)
	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		if isMySQLDuplicate(err) {
			return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "duplicate tax code or constraint", err)
		}
		return mapMySQLSchemaErr(err)
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		return nil
	}
	// MySQL default: unchanged rows report RowsAffected=0. Distinguish "no field changed" from missing company.
	var one int
	if qerr := r.db.QueryRowContext(ctx, `SELECT 1 FROM companies WHERE company_id = ? LIMIT 1`, companyID).Scan(&one); qerr != nil {
		if qerr == sql.ErrNoRows {
			return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "company not found", nil)
		}
		return mapMySQLSchemaErr(qerr)
	}
	return nil
}

func (r *AdminRepository) SetCompanyStatusPlatform(ctx context.Context, companyID, status string) error {
	companyID = strings.TrimSpace(companyID)
	status = strings.ToLower(strings.TrimSpace(status))
	if companyID == "" || status == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id and status are required", nil)
	}
	res, err := r.db.ExecContext(ctx, `UPDATE companies SET status = ? WHERE company_id = ?`, status, companyID)
	if err != nil {
		return mapMySQLSchemaErr(err)
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		return nil
	}
	var one int
	if qerr := r.db.QueryRowContext(ctx, `SELECT 1 FROM companies WHERE company_id = ? LIMIT 1`, companyID).Scan(&one); qerr != nil {
		if qerr == sql.ErrNoRows {
			return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "company not found", nil)
		}
		return mapMySQLSchemaErr(qerr)
	}
	return nil
}
