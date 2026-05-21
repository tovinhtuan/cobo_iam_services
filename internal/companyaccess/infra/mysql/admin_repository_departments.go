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

func (r *AdminRepository) MembershipBelongsToCompany(ctx context.Context, membershipID, companyID string) (bool, error) {
	var cid string
	err := r.db.QueryRowContext(ctx,
		`SELECT company_id FROM memberships WHERE membership_id = ? AND membership_status != 'deleted'`,
		membershipID,
	).Scan(&cid)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return cid == companyID, nil
}

func (r *AdminRepository) ListCompanyDepartments(ctx context.Context, companyID string) ([]caapp.DepartmentView, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT d.department_id,
		       d.department_name,
		       d.head_membership_id,
		       u.full_name,
		       (SELECT COUNT(*) FROM department_memberships dm
		        WHERE dm.department_id = d.department_id AND dm.status = 'active') AS member_count,
		       d.status,
		       d.sort_order
		FROM departments d
		LEFT JOIN memberships m ON m.membership_id = d.head_membership_id
		LEFT JOIN users u ON u.user_id = m.user_id
		WHERE d.company_id = ? AND d.status = 'active'
		ORDER BY d.sort_order ASC, d.created_at ASC
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list departments: %w", mapMySQLSchemaErr(err))
	}
	defer rows.Close()

	var out []caapp.DepartmentView
	for rows.Next() {
		var v caapp.DepartmentView
		var headID sql.NullString
		var headName sql.NullString
		if err := rows.Scan(&v.DepartmentID, &v.DepartmentName, &headID, &headName, &v.MemberCount, &v.Status, &v.SortOrder); err != nil {
			return nil, err
		}
		v.Name = v.DepartmentName
		if headID.Valid {
			v.HeadMembershipID = &headID.String
		}
		if headName.Valid {
			v.HeadFullName = &headName.String
		}
		out = append(out, v)
	}
	if out == nil {
		out = []caapp.DepartmentView{}
	}
	return out, rows.Err()
}

func (r *AdminRepository) CreateDepartmentRow(ctx context.Context, companyID, deptID, deptCode, name string, headMembershipID *string, sortOrder int) (*caapp.DepartmentView, error) {
	var headArg interface{}
	if headMembershipID != nil && *headMembershipID != "" {
		headArg = *headMembershipID
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO departments (department_id, company_id, department_code, department_name, head_membership_id, sort_order, status)
		VALUES (?, ?, ?, ?, ?, ?, 'active')
	`, deptID, companyID, deptCode, name, headArg, sortOrder)
	if err != nil {
		if isMySQLDuplicate(err) {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "department name already exists in company", nil)
		}
		return nil, fmt.Errorf("create department: %w", mapMySQLSchemaErr(err))
	}
	return r.getDepartmentView(ctx, deptID)
}

func (r *AdminRepository) PatchDepartmentRow(ctx context.Context, companyID, deptID string, name *string, headMembershipID *string, clearHead bool, sortOrder *int, status *string) (*caapp.DepartmentView, error) {
	sets := []string{}
	args := []any{}

	if name != nil {
		sets = append(sets, "department_name = ?")
		args = append(args, *name)
	}
	if clearHead {
		sets = append(sets, "head_membership_id = NULL")
	} else if headMembershipID != nil && *headMembershipID != "" {
		sets = append(sets, "head_membership_id = ?")
		args = append(args, *headMembershipID)
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
		return r.getDepartmentView(ctx, deptID)
	}

	args = append(args, companyID, deptID)
	q := "UPDATE departments SET " + strings.Join(sets, ", ") + " WHERE company_id = ? AND department_id = ? AND status != 'inactive'"
	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		if isMySQLDuplicate(err) {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "department name already exists in company", nil)
		}
		return nil, fmt.Errorf("patch department: %w", mapMySQLSchemaErr(err))
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "department not found", nil)
	}
	return r.getDepartmentView(ctx, deptID)
}

func (r *AdminRepository) SoftDeleteDepartment(ctx context.Context, deptID, companyID string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE departments SET status = 'inactive' WHERE department_id = ? AND company_id = ? AND status = 'active'`,
		deptID, companyID,
	)
	if err != nil {
		return fmt.Errorf("soft delete department: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "department not found", nil)
	}
	return nil
}

func (r *AdminRepository) CountDepartmentMembers(ctx context.Context, deptID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM department_memberships WHERE department_id = ? AND status = 'active'`,
		deptID,
	).Scan(&n)
	return n, err
}

func (r *AdminRepository) getDepartmentView(ctx context.Context, deptID string) (*caapp.DepartmentView, error) {
	var v caapp.DepartmentView
	var headID sql.NullString
	var headName sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT d.department_id,
		       d.department_name,
		       d.head_membership_id,
		       u.full_name,
		       (SELECT COUNT(*) FROM department_memberships dm
		        WHERE dm.department_id = d.department_id AND dm.status = 'active') AS member_count,
		       d.status,
		       d.sort_order
		FROM departments d
		LEFT JOIN memberships m ON m.membership_id = d.head_membership_id
		LEFT JOIN users u ON u.user_id = m.user_id
		WHERE d.department_id = ?
	`, deptID).Scan(&v.DepartmentID, &v.DepartmentName, &headID, &headName, &v.MemberCount, &v.Status, &v.SortOrder)
	if err == sql.ErrNoRows {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "department not found", nil)
	}
	if err != nil {
		return nil, fmt.Errorf("get department: %w", mapMySQLSchemaErr(err))
	}
	v.Name = v.DepartmentName
	if headID.Valid {
		v.HeadMembershipID = &headID.String
	}
	if headName.Valid {
		v.HeadFullName = &headName.String
	}
	return &v, nil
}
