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

func (r *AdminRepository) ListDepartmentTeams(ctx context.Context, companyID, departmentID string) ([]caapp.TeamView, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT o.org_unit_id, o.department_id, o.unit_name, o.status,
		       (SELECT COUNT(*) FROM org_unit_memberships m
		        WHERE m.org_unit_id = o.org_unit_id AND m.status = 'active') AS member_count
		FROM org_units o
		WHERE o.company_id = ? AND o.department_id = ? AND o.unit_type = 'team' AND o.status = 'active'
		ORDER BY o.created_at ASC
	`, companyID, departmentID)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", mapMySQLSchemaErr(err))
	}
	defer rows.Close()

	var out []caapp.TeamView
	for rows.Next() {
		var v caapp.TeamView
		var deptID sql.NullString
		if err := rows.Scan(&v.TeamID, &deptID, &v.Name, &v.Status, &v.MemberCount); err != nil {
			return nil, err
		}
		if deptID.Valid {
			v.DepartmentID = deptID.String
		}
		out = append(out, v)
	}
	if out == nil {
		out = []caapp.TeamView{}
	}
	return out, rows.Err()
}

func (r *AdminRepository) CreateTeamRow(ctx context.Context, companyID, departmentID, teamID, name string) (*caapp.TeamView, error) {
	unitCode := "team_" + teamID[:8]
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO org_units (org_unit_id, company_id, department_id, unit_code, unit_name, unit_type, status)
		VALUES (?, ?, ?, ?, ?, 'team', 'active')
	`, teamID, companyID, departmentID, unitCode, name)
	if err != nil {
		if isMySQLDuplicate(err) {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "team name already exists in department", nil)
		}
		return nil, fmt.Errorf("create team: %w", mapMySQLSchemaErr(err))
	}
	return &caapp.TeamView{TeamID: teamID, DepartmentID: departmentID, Name: name, Status: "active"}, nil
}

func (r *AdminRepository) PatchTeamRow(ctx context.Context, companyID, teamID string, name *string, status *string) (*caapp.TeamView, error) {
	sets := []string{}
	args := []any{}

	if name != nil {
		sets = append(sets, "unit_name = ?")
		args = append(args, *name)
	}
	if status != nil {
		sets = append(sets, "status = ?")
		args = append(args, *status)
	}
	if len(sets) == 0 {
		return r.getTeamView(ctx, teamID)
	}

	args = append(args, companyID, teamID)
	q := "UPDATE org_units SET " + strings.Join(sets, ", ") + " WHERE company_id = ? AND org_unit_id = ? AND unit_type = 'team'"
	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("patch team: %w", mapMySQLSchemaErr(err))
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "team not found", nil)
	}
	return r.getTeamView(ctx, teamID)
}

func (r *AdminRepository) DeleteTeamRow(ctx context.Context, companyID, teamID string) error {
	// Cascade: delete all org_unit_memberships first
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM org_unit_memberships WHERE org_unit_id = ?`, teamID); err != nil {
		return fmt.Errorf("cascade delete team members: %w", err)
	}
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM org_units WHERE org_unit_id = ? AND company_id = ? AND unit_type = 'team'`,
		teamID, companyID)
	if err != nil {
		return fmt.Errorf("delete team: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "team not found", nil)
	}
	return nil
}

func (r *AdminRepository) CountTeamsInDepartment(ctx context.Context, departmentID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM org_units WHERE department_id = ? AND unit_type = 'team' AND status = 'active'`,
		departmentID,
	).Scan(&n)
	return n, err
}

func (r *AdminRepository) AddTeamMember(ctx context.Context, companyID, teamID, membershipID string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO org_unit_memberships (company_id, membership_id, org_unit_id, position_code, status)
		VALUES (?, ?, ?, 'nhan_vien', 'active')
		ON DUPLICATE KEY UPDATE status = 'active', updated_at = CURRENT_TIMESTAMP
	`, companyID, membershipID, teamID)
	if err != nil {
		return fmt.Errorf("add team member: %w", mapMySQLSchemaErr(err))
	}
	return nil
}

func (r *AdminRepository) RemoveTeamMember(ctx context.Context, _, teamID, membershipID string) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM org_unit_memberships WHERE org_unit_id = ? AND membership_id = ?`,
		teamID, membershipID)
	if err != nil {
		return fmt.Errorf("remove team member: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "team member not found", nil)
	}
	return nil
}

func (r *AdminRepository) MemberBelongsToDepartment(ctx context.Context, membershipID, departmentID string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM department_memberships WHERE membership_id = ? AND department_id = ? AND status = 'active'`,
		membershipID, departmentID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *AdminRepository) getTeamView(ctx context.Context, teamID string) (*caapp.TeamView, error) {
	var v caapp.TeamView
	var deptID sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT o.org_unit_id, o.department_id, o.unit_name, o.status,
		       (SELECT COUNT(*) FROM org_unit_memberships m
		        WHERE m.org_unit_id = o.org_unit_id AND m.status = 'active') AS member_count
		FROM org_units o
		WHERE o.org_unit_id = ? AND o.unit_type = 'team'
	`, teamID).Scan(&v.TeamID, &deptID, &v.Name, &v.Status, &v.MemberCount)
	if err == sql.ErrNoRows {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "team not found", nil)
	}
	if err != nil {
		return nil, fmt.Errorf("get team: %w", mapMySQLSchemaErr(err))
	}
	if deptID.Valid {
		v.DepartmentID = deptID.String
	}
	return &v, nil
}
