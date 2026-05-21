package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func (r *AdminRepository) ListCompanyTitles(ctx context.Context, companyID string) ([]caapp.TitleView, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.title_id,
		       t.title_name,
		       (SELECT COUNT(*) FROM membership_titles mt
		        WHERE mt.title_id = t.title_id AND mt.status = 'active') AS member_count,
		       t.status,
		       t.sort_order
		FROM titles t
		WHERE t.company_id = ? AND t.status = 'active'
		ORDER BY t.sort_order ASC, t.created_at ASC
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list titles: %w", mapMySQLSchemaErr(err))
	}
	defer rows.Close()

	var out []caapp.TitleView
	for rows.Next() {
		var v caapp.TitleView
		if err := rows.Scan(&v.TitleID, &v.TitleName, &v.MemberCount, &v.Status, &v.SortOrder); err != nil {
			return nil, err
		}
		v.Name = v.TitleName
		out = append(out, v)
	}
	if out == nil {
		out = []caapp.TitleView{}
	}
	return out, rows.Err()
}

func (r *AdminRepository) CreateTitleRow(ctx context.Context, companyID, titleID, titleCode, name string, sortOrder int) (*caapp.TitleView, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO titles (title_id, company_id, title_code, title_name, sort_order, status)
		VALUES (?, ?, ?, ?, ?, 'active')
	`, titleID, companyID, titleCode, name, sortOrder)
	if err != nil {
		if isMySQLDuplicate(err) {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "title name already exists in company", nil)
		}
		return nil, fmt.Errorf("create title: %w", mapMySQLSchemaErr(err))
	}
	return r.getTitleView(ctx, titleID)
}

func (r *AdminRepository) PatchTitleRow(ctx context.Context, companyID, titleID string, name *string, sortOrder *int, status *string) (*caapp.TitleView, error) {
	sets := []string{}
	args := []any{}

	if name != nil {
		sets = append(sets, "title_name = ?")
		args = append(args, *name)
	}
	if sortOrder != nil {
		sets = append(sets, "sort_order = ?")
		args = append(args, *sortOrder)
	}
	if status != nil {
		sets = append(sets, "status = ?")
		args = append(args, *status)
	}

	if len(sets) == 0 {
		return r.getTitleView(ctx, titleID)
	}

	args = append(args, companyID, titleID)
	q := "UPDATE titles SET " + strings.Join(sets, ", ") + " WHERE company_id = ? AND title_id = ? AND status != 'inactive'"
	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		if isMySQLDuplicate(err) {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "title name already exists in company", nil)
		}
		return nil, fmt.Errorf("patch title: %w", mapMySQLSchemaErr(err))
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "title not found", nil)
	}
	return r.getTitleView(ctx, titleID)
}

func (r *AdminRepository) SoftDeleteTitle(ctx context.Context, titleID, companyID string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE titles SET status = 'inactive' WHERE title_id = ? AND company_id = ? AND status = 'active'`,
		titleID, companyID,
	)
	if err != nil {
		return fmt.Errorf("soft delete title: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "title not found", nil)
	}
	return nil
}

func (r *AdminRepository) CountTitleMembers(ctx context.Context, titleID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM membership_titles WHERE title_id = ? AND status = 'active'`,
		titleID,
	).Scan(&n)
	return n, err
}

func (r *AdminRepository) getTitleView(ctx context.Context, titleID string) (*caapp.TitleView, error) {
	var v caapp.TitleView
	err := r.db.QueryRowContext(ctx, `
		SELECT t.title_id,
		       t.title_name,
		       (SELECT COUNT(*) FROM membership_titles mt
		        WHERE mt.title_id = t.title_id AND mt.status = 'active') AS member_count,
		       t.status,
		       t.sort_order
		FROM titles t
		WHERE t.title_id = ?
	`, titleID).Scan(&v.TitleID, &v.TitleName, &v.MemberCount, &v.Status, &v.SortOrder)
	if err == sql.ErrNoRows {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "title not found", nil)
	}
	if err != nil {
		return nil, fmt.Errorf("get title: %w", mapMySQLSchemaErr(err))
	}
	v.Name = v.TitleName
	return &v, nil
}
