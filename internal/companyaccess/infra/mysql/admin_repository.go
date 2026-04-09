package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/google/uuid"
)

// AdminRepository persists company access admin operations (memberships, roles, rules).
type AdminRepository struct {
	db *sql.DB
}

func NewAdminRepository(db *sql.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

func (r *AdminRepository) CreateMembership(ctx context.Context, m caapp.MembershipView) (*caapp.MembershipView, error) {
	var dummy string
	if err := r.db.QueryRowContext(ctx, `SELECT user_id FROM users WHERE user_id = ?`, m.UserID).Scan(&dummy); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "user not found", nil)
		}
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT company_id FROM companies WHERE company_id = ?`, m.CompanyID).Scan(&dummy); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company not found", nil)
		}
		return nil, err
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO memberships (membership_id, user_id, company_id, membership_status)
		VALUES (?, ?, ?, ?)
	`, m.MembershipID, m.UserID, m.CompanyID, m.Status)
	if err != nil {
		if isMySQLDuplicate(err) {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeInvalidRequest, "membership already exists for user and company", nil)
		}
		return nil, fmt.Errorf("create membership: %w", err)
	}
	return r.getMembershipView(ctx, m.MembershipID)
}

func (r *AdminRepository) UpdateMembershipStatus(ctx context.Context, membershipID, status string) (*caapp.MembershipView, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE memberships SET membership_status = ? WHERE membership_id = ?
	`, status, membershipID)
	if err != nil {
		return nil, fmt.Errorf("update membership: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeMembershipNotFound, "membership not found", nil)
	}
	return r.getMembershipView(ctx, membershipID)
}

func (r *AdminRepository) DeleteMembership(ctx context.Context, membershipID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, q := range []string{
		`DELETE FROM membership_roles WHERE membership_id = ?`,
		`DELETE FROM department_memberships WHERE membership_id = ?`,
		`DELETE FROM membership_titles WHERE membership_id = ?`,
	} {
		if _, err := tx.ExecContext(ctx, q, membershipID); err != nil {
			return err
		}
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM memberships WHERE membership_id = ?`, membershipID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeMembershipNotFound, "membership not found", nil)
	}
	return tx.Commit()
}

func (r *AdminRepository) ListMembershipsByCompany(ctx context.Context, companyID string) ([]caapp.MembershipView, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.membership_id, m.user_id, m.company_id, c.company_name, m.membership_status
		FROM memberships m
		INNER JOIN companies c ON c.company_id = m.company_id
		WHERE m.company_id = ?
		ORDER BY m.created_at DESC
	`, companyID)
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

func (r *AdminRepository) AddRole(ctx context.Context, membershipID, roleID string) error {
	if err := r.ensureMembership(ctx, membershipID); err != nil {
		return err
	}
	if err := r.ensureRoleForMembership(ctx, membershipID, roleID); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO membership_roles (membership_id, role_id, status)
		VALUES (?, ?, 'active')
		ON DUPLICATE KEY UPDATE status = 'active', updated_at = CURRENT_TIMESTAMP
	`, membershipID, roleID)
	return err
}

func (r *AdminRepository) RemoveRole(ctx context.Context, membershipID, roleID string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM membership_roles WHERE membership_id = ? AND role_id = ?
	`, membershipID, roleID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "membership role not found", nil)
	}
	return nil
}

func (r *AdminRepository) AddDepartment(ctx context.Context, membershipID, departmentID string) error {
	if err := r.ensureMembership(ctx, membershipID); err != nil {
		return err
	}
	if err := r.ensureDepartmentForMembership(ctx, membershipID, departmentID); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO department_memberships (membership_id, department_id, status)
		VALUES (?, ?, 'active')
		ON DUPLICATE KEY UPDATE status = 'active', updated_at = CURRENT_TIMESTAMP
	`, membershipID, departmentID)
	return err
}

func (r *AdminRepository) RemoveDepartment(ctx context.Context, membershipID, departmentID string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM department_memberships WHERE membership_id = ? AND department_id = ?
	`, membershipID, departmentID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "department membership not found", nil)
	}
	return nil
}

func (r *AdminRepository) AddTitle(ctx context.Context, membershipID, titleID string) error {
	if err := r.ensureMembership(ctx, membershipID); err != nil {
		return err
	}
	if err := r.ensureTitleForMembership(ctx, membershipID, titleID); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO membership_titles (membership_id, title_id, status)
		VALUES (?, ?, 'active')
		ON DUPLICATE KEY UPDATE status = 'active', updated_at = CURRENT_TIMESTAMP
	`, membershipID, titleID)
	return err
}

func (r *AdminRepository) RemoveTitle(ctx context.Context, membershipID, titleID string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM membership_titles WHERE membership_id = ? AND title_id = ?
	`, membershipID, titleID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "membership title not found", nil)
	}
	return nil
}

func (r *AdminRepository) ListPermissions(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT permission_id FROM permissions WHERE status = 'active' ORDER BY permission_code
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStringRows(rows)
}

func (r *AdminRepository) ListRoles(ctx context.Context, companyID string) ([]string, error) {
	q := `
		SELECT role_id FROM roles
		WHERE status = 'active' AND company_id IS NULL
	`
	args := []any{}
	if strings.TrimSpace(companyID) != "" {
		q = `
			SELECT role_id FROM roles
			WHERE status = 'active' AND (company_id IS NULL OR company_id = ?)
			ORDER BY role_code
		`
		args = append(args, companyID)
	} else {
		q += ` ORDER BY role_code`
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStringRows(rows)
}

func (r *AdminRepository) AddRolePermission(ctx context.Context, roleID, permissionID string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO role_permissions (role_id, permission_id, status)
		VALUES (?, ?, 'active')
		ON DUPLICATE KEY UPDATE status = 'active', created_at = created_at
	`, roleID, permissionID)
	return err
}

func (r *AdminRepository) RemoveRolePermission(ctx context.Context, roleID, permissionID string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM role_permissions WHERE role_id = ? AND permission_id = ?
	`, roleID, permissionID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "role permission binding not found", nil)
	}
	return nil
}

func (r *AdminRepository) AddResourceScopeRule(ctx context.Context, rule map[string]any) error {
	companyID, ok := strFromMap(rule, "company_id")
	if !ok {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id required", nil)
	}
	ruleCode, ok := strFromMap(rule, "rule_code")
	if !ok {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "rule_code required", nil)
	}
	resourceType, ok := strFromMap(rule, "resource_type")
	if !ok {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "resource_type required", nil)
	}
	scopeType, ok := strFromMap(rule, "scope_type")
	if !ok {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "scope_type required", nil)
	}
	subjectType, ok := strFromMap(rule, "subject_type")
	if !ok {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "subject_type required", nil)
	}
	subjectRef, ok := strFromMap(rule, "subject_ref_id")
	if !ok {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "subject_ref_id required", nil)
	}
	var meta interface{}
	if raw, err := json.Marshal(rule); err == nil {
		meta = raw
	}
	id := uuid.NewString()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO resource_scope_rules (
			resource_scope_rule_id, company_id, rule_code, resource_type, scope_type, subject_type, subject_ref_id, metadata_json, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'active')
	`, id, companyID, ruleCode, resourceType, scopeType, subjectType, subjectRef, meta)
	if err != nil {
		if isMySQLDuplicate(err) {
			return perr.NewHTTPError(http.StatusConflict, perr.CodeInvalidRequest, "rule_code already exists for company", nil)
		}
		return fmt.Errorf("insert resource_scope_rule: %w", err)
	}
	return nil
}

func (r *AdminRepository) AddWorkflowAssigneeRule(ctx context.Context, rule map[string]any) error {
	companyID, ok := strFromMap(rule, "company_id")
	if !ok {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id required", nil)
	}
	ruleCode, ok := strFromMap(rule, "rule_code")
	if !ok {
		ruleCode = uuid.NewString()
	}
	payload, err := json.Marshal(rule)
	if err != nil {
		return err
	}
	id := uuid.NewString()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO workflow_assignee_rules (workflow_assignee_rule_id, company_id, rule_code, payload_json, status)
		VALUES (?, ?, ?, ?, 'active')
	`, id, companyID, ruleCode, payload)
	if err != nil {
		if isMySQLDuplicate(err) {
			return perr.NewHTTPError(http.StatusConflict, perr.CodeInvalidRequest, "rule_code already exists for company", nil)
		}
		return fmt.Errorf("insert workflow_assignee_rule: %w", err)
	}
	return nil
}

func (r *AdminRepository) AddNotificationRule(ctx context.Context, rule map[string]any) error {
	companyID, ok := strFromMap(rule, "company_id")
	if !ok {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id required", nil)
	}
	ruleCode, ok := strFromMap(rule, "rule_code")
	if !ok {
		ruleCode = uuid.NewString()
	}
	payload, err := json.Marshal(rule)
	if err != nil {
		return err
	}
	id := uuid.NewString()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO notification_rules (notification_rule_id, company_id, rule_code, payload_json, status)
		VALUES (?, ?, ?, ?, 'active')
	`, id, companyID, ruleCode, payload)
	if err != nil {
		if isMySQLDuplicate(err) {
			return perr.NewHTTPError(http.StatusConflict, perr.CodeInvalidRequest, "rule_code already exists for company", nil)
		}
		return fmt.Errorf("insert notification_rule: %w", err)
	}
	return nil
}

func (r *AdminRepository) getMembershipView(ctx context.Context, membershipID string) (*caapp.MembershipView, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT m.membership_id, m.user_id, m.company_id, c.company_name, m.membership_status
		FROM memberships m
		INNER JOIN companies c ON c.company_id = m.company_id
		WHERE m.membership_id = ?
	`, membershipID)
	var v caapp.MembershipView
	if err := row.Scan(&v.MembershipID, &v.UserID, &v.CompanyID, &v.CompanyName, &v.Status); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeMembershipNotFound, "membership not found", nil)
		}
		return nil, err
	}
	return &v, nil
}

func (r *AdminRepository) ensureMembership(ctx context.Context, membershipID string) error {
	var x string
	if err := r.db.QueryRowContext(ctx, `SELECT membership_id FROM memberships WHERE membership_id = ?`, membershipID).Scan(&x); err != nil {
		if err == sql.ErrNoRows {
			return perr.NewHTTPError(http.StatusNotFound, perr.CodeMembershipNotFound, "membership not found", nil)
		}
		return err
	}
	return nil
}

func (r *AdminRepository) ensureRoleForMembership(ctx context.Context, membershipID, roleID string) error {
	var companyID string
	if err := r.db.QueryRowContext(ctx, `SELECT company_id FROM memberships WHERE membership_id = ?`, membershipID).Scan(&companyID); err != nil {
		return err
	}
	var rCompany sql.NullString
	if err := r.db.QueryRowContext(ctx, `SELECT company_id FROM roles WHERE role_id = ? AND status = 'active'`, roleID).Scan(&rCompany); err != nil {
		if err == sql.ErrNoRows {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "role not found", nil)
		}
		return err
	}
	if rCompany.Valid && rCompany.String != "" && rCompany.String != companyID {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "role does not belong to membership company", nil)
	}
	return nil
}

func (r *AdminRepository) ensureDepartmentForMembership(ctx context.Context, membershipID, departmentID string) error {
	var mCompany, dCompany string
	if err := r.db.QueryRowContext(ctx, `SELECT company_id FROM memberships WHERE membership_id = ?`, membershipID).Scan(&mCompany); err != nil {
		return err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT company_id FROM departments WHERE department_id = ? AND status = 'active'`, departmentID).Scan(&dCompany); err != nil {
		if err == sql.ErrNoRows {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "department not found", nil)
		}
		return err
	}
	if mCompany != dCompany {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "department company mismatch", nil)
	}
	return nil
}

func (r *AdminRepository) ensureTitleForMembership(ctx context.Context, membershipID, titleID string) error {
	var mCompany, tCompany string
	if err := r.db.QueryRowContext(ctx, `SELECT company_id FROM memberships WHERE membership_id = ?`, membershipID).Scan(&mCompany); err != nil {
		return err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT company_id FROM titles WHERE title_id = ? AND status = 'active'`, titleID).Scan(&tCompany); err != nil {
		if err == sql.ErrNoRows {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "title not found", nil)
		}
		return err
	}
	if mCompany != tCompany {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "title company mismatch", nil)
	}
	return nil
}

func scanStringRows(rows *sql.Rows) ([]string, error) {
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func strFromMap(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return "", false
	}
	switch x := v.(type) {
	case string:
		s := strings.TrimSpace(x)
		return s, s != ""
	default:
		s := strings.TrimSpace(fmt.Sprint(x))
		return s, s != ""
	}
}

func isMySQLDuplicate(err error) bool {
	if err == nil {
		return false
	}
	// go-sql-driver/mysql: Error 1062
	return strings.Contains(strings.ToLower(err.Error()), "duplicate")
}
