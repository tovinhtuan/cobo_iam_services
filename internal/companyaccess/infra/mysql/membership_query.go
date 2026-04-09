package mysql

import (
	"context"
	"database/sql"
	"net/http"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type MembershipQueryService struct {
	db *sql.DB
}

func NewMembershipQueryService(db *sql.DB) *MembershipQueryService {
	return &MembershipQueryService{db: db}
}

func (s *MembershipQueryService) GetMembershipsByUser(ctx context.Context, userID string) ([]caapp.MembershipView, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.membership_id, m.user_id, m.company_id, c.company_name, m.membership_status
		FROM memberships m
		INNER JOIN companies c ON c.company_id = m.company_id
		WHERE m.user_id = ?
		ORDER BY c.company_name
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []caapp.MembershipView
	for rows.Next() {
		var v caapp.MembershipView
		if err := rows.Scan(&v.MembershipID, &v.UserID, &v.CompanyID, &v.CompanyName, &v.Status); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *MembershipQueryService) GetActiveMembership(ctx context.Context, userID, companyID string) (*caapp.MembershipView, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT m.membership_id, m.user_id, m.company_id, c.company_name, m.membership_status
		FROM memberships m
		INNER JOIN companies c ON c.company_id = m.company_id
		WHERE m.user_id = ? AND m.company_id = ? AND LOWER(m.membership_status) = 'active'
	`, userID, companyID)
	var v caapp.MembershipView
	if err := row.Scan(&v.MembershipID, &v.UserID, &v.CompanyID, &v.CompanyName, &v.Status); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodeMembershipNotFound, "membership not found", nil)
		}
		return nil, err
	}
	return &v, nil
}

func (s *MembershipQueryService) GetMembershipRoles(ctx context.Context, membershipID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.role_code
		FROM membership_roles mr
		INNER JOIN roles r ON r.role_id = mr.role_id
		WHERE mr.membership_id = ? AND mr.status = 'active' AND r.status = 'active'
		ORDER BY r.role_code
	`, membershipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, rows.Err()
}

func (s *MembershipQueryService) GetMembershipDepartments(ctx context.Context, membershipID string) ([]caapp.DepartmentView, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT d.department_id, d.department_name
		FROM department_memberships dm
		INNER JOIN departments d ON d.department_id = dm.department_id
		WHERE dm.membership_id = ? AND dm.status = 'active' AND d.status = 'active'
		ORDER BY d.department_name
	`, membershipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []caapp.DepartmentView
	for rows.Next() {
		var v caapp.DepartmentView
		if err := rows.Scan(&v.DepartmentID, &v.DepartmentName); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *MembershipQueryService) GetMembershipTitles(ctx context.Context, membershipID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.title_name
		FROM membership_titles mt
		INNER JOIN titles t ON t.title_id = mt.title_id
		WHERE mt.membership_id = ? AND mt.status = 'active' AND t.status = 'active'
		ORDER BY t.title_name
	`, membershipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}
