package mysql

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// OrgDirectory validates department and membership refs for proposal workflow steps.
type OrgDirectory struct {
	db *sql.DB
}

func NewOrgDirectory(db *sql.DB) *OrgDirectory {
	return &OrgDirectory{db: db}
}

func (d *OrgDirectory) IsActiveDepartmentInCompany(ctx context.Context, companyID, departmentID string) (bool, error) {
	if d == nil || d.db == nil {
		return false, nil
	}
	var n int
	err := d.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM departments
		WHERE department_id = ? AND company_id = ? AND status = 'active'
	`, departmentID, companyID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (d *OrgDirectory) IsActiveMembershipInCompany(ctx context.Context, companyID, membershipID string) (bool, error) {
	if d == nil || d.db == nil {
		return false, nil
	}
	var n int
	err := d.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM memberships
		WHERE membership_id = ? AND company_id = ? AND membership_status = 'active'
	`, membershipID, companyID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (d *OrgDirectory) MemberBelongsToDepartment(ctx context.Context, membershipID, departmentID string) (bool, error) {
	if d == nil || d.db == nil {
		return false, nil
	}
	var n int
	err := d.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM department_memberships
		WHERE membership_id = ? AND department_id = ? AND status = 'active'
	`, membershipID, departmentID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ResolveDepartmentHeadMembership reads departments.head_membership_id and validates
// active + same company + belongs to department. No title heuristics / deputy / admin fallback.
func (d *OrgDirectory) ResolveDepartmentHeadMembership(ctx context.Context, companyID, departmentID string) (string, error) {
	if d == nil || d.db == nil {
		return "", perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "organization directory is unavailable", nil)
	}
	companyID = strings.TrimSpace(companyID)
	departmentID = strings.TrimSpace(departmentID)
	if companyID == "" || departmentID == "" {
		return "", headErr("department_head_invalid", "department_head_invalid: department_id is required")
	}

	var head sql.NullString
	err := d.db.QueryRowContext(ctx, `
		SELECT head_membership_id FROM departments
		WHERE department_id = ? AND company_id = ? AND status = 'active'
	`, departmentID, companyID).Scan(&head)
	if err == sql.ErrNoRows {
		return "", headErr("department_head_invalid", "department_head_invalid: department not found or inactive in company")
	}
	if err != nil {
		return "", err
	}
	if !head.Valid || strings.TrimSpace(head.String) == "" {
		return "", headErr("department_head_not_configured", "department_head_not_configured: department has no head_membership_id")
	}
	headID := strings.TrimSpace(head.String)

	ok, err := d.IsActiveMembershipInCompany(ctx, companyID, headID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", headErr("department_head_invalid", "department_head_invalid: head membership is inactive or not in company")
	}
	belongs, err := d.MemberBelongsToDepartment(ctx, headID, departmentID)
	if err != nil {
		return "", err
	}
	if !belongs {
		return "", headErr("department_head_invalid", "department_head_invalid: head membership does not belong to department")
	}
	return headID, nil
}

func headErr(code, message string) error {
	he := perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, message, nil)
	he.Details = map[string]any{"code": code}
	return he
}
