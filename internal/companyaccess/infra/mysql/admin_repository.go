package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

// AdminRepository persists company access admin operations (memberships, roles, rules).
type AdminRepository struct {
	db *sql.DB
}

func NewAdminRepository(db *sql.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

func (r *AdminRepository) CreateUser(ctx context.Context, u caapp.UserView, passwordHash string, opts caapp.CreateUserOptions) (*caapp.UserView, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO users (user_id, login_id, full_name, email, phone, account_status)
		VALUES (?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?)
	`, u.UserID, u.LoginID, u.FullName, u.Email, u.Phone, u.AccountStatus)
	if err != nil {
		if isMySQLDuplicate(err) {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "login_id already exists", nil)
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO credentials (credential_id, user_id, credential_type, password_hash, password_algo, status, password_changed_at)
		VALUES (?, ?, 'password', ?, 'bcrypt', 'active', CURRENT_TIMESTAMP)
	`, uuid.NewString(), u.UserID, passwordHash)
	if err != nil {
		return nil, fmt.Errorf("create credential: %w", err)
	}

	if strings.TrimSpace(opts.CompanyID) != "" {
		var companyName string
		if err := tx.QueryRowContext(ctx, `SELECT company_name FROM companies WHERE company_id = ?`, opts.CompanyID).Scan(&companyName); err != nil {
			if err == sql.ErrNoRows {
				return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company not found", nil)
			}
			return nil, err
		}

		membershipStatus := strings.TrimSpace(opts.MembershipStatus)
		if membershipStatus == "" {
			membershipStatus = "active"
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO memberships (membership_id, user_id, company_id, membership_status)
			VALUES (?, ?, ?, ?)
		`, opts.MembershipID, u.UserID, opts.CompanyID, membershipStatus)
		if err != nil {
			if isMySQLDuplicate(err) {
				return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "membership already exists for user and company", nil)
			}
			return nil, fmt.Errorf("create membership: %w", err)
		}
		u.MembershipID = opts.MembershipID
		u.MembershipStatus = membershipStatus
		u.CompanyID = opts.CompanyID
		u.CompanyName = companyName
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if u.MembershipID != "" {
		return &u, nil
	}
	return r.getUserView(ctx, u.UserID)
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
		SELECT m.membership_id, m.user_id, m.company_id, c.company_name, m.membership_status,
		       u.login_id, u.full_name, u.account_status, m.is_primary_admin
		FROM memberships m
		INNER JOIN companies c ON c.company_id = m.company_id
		INNER JOIN users u ON u.user_id = m.user_id
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
		if err := rows.Scan(&v.MembershipID, &v.UserID, &v.CompanyID, &v.CompanyName, &v.Status,
			&v.LoginID, &v.FullName, &v.AccountStatus, &v.IsPrimaryAdmin); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Departments, _ = r.listMembershipDeptViews(ctx, out[i].MembershipID)
		out[i].Titles, _ = r.listMembershipTitleViews(ctx, out[i].MembershipID)
		out[i].Roles, _ = r.listMembershipRoleViews(ctx, out[i].MembershipID)
	}
	return out, nil
}

func (r *AdminRepository) listMembershipDeptViews(ctx context.Context, membershipID string) ([]caapp.DepartmentView, error) {
	rows, err := r.db.QueryContext(ctx, `
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
		v.Name = v.DepartmentName
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *AdminRepository) listMembershipTitleViews(ctx context.Context, membershipID string) ([]caapp.TitleView, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.title_id, t.title_name
		FROM membership_titles mt
		INNER JOIN titles t ON t.title_id = mt.title_id
		WHERE mt.membership_id = ? AND mt.status = 'active' AND t.status = 'active'
		ORDER BY t.title_name
	`, membershipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []caapp.TitleView
	for rows.Next() {
		var v caapp.TitleView
		if err := rows.Scan(&v.TitleID, &v.TitleName); err != nil {
			return nil, err
		}
		v.Name = v.TitleName
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *AdminRepository) listMembershipRoleViews(ctx context.Context, membershipID string) ([]caapp.RoleView, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.role_id, r.role_code, r.role_name
		FROM membership_roles mr
		INNER JOIN roles r ON r.role_id = mr.role_id
		WHERE mr.membership_id = ? AND mr.status = 'active' AND r.status = 'active'
		ORDER BY r.role_name
	`, membershipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []caapp.RoleView
	for rows.Next() {
		var v caapp.RoleView
		if err := rows.Scan(&v.RoleID, &v.RoleCode, &v.RoleName); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *AdminRepository) SetMembershipPrimaryAdmin(ctx context.Context, membershipID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE memberships SET is_primary_admin = TRUE WHERE membership_id = ?`, membershipID)
	return err
}

func (r *AdminRepository) ClearMembershipPrimaryAdmin(ctx context.Context, membershipID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE memberships SET is_primary_admin = FALSE WHERE membership_id = ?`, membershipID)
	return err
}

func (r *AdminRepository) GetMembershipByID(ctx context.Context, membershipID string) (*caapp.MembershipView, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT m.membership_id, m.user_id, m.company_id, c.company_name,
		       m.membership_status, u.login_id, u.full_name, u.account_status, m.is_primary_admin
		FROM memberships m
		INNER JOIN users u ON u.user_id = m.user_id
		INNER JOIN companies c ON c.company_id = m.company_id
		WHERE m.membership_id = ?
	`, membershipID)
	var mv caapp.MembershipView
	var isPrimaryAdmin bool
	err := row.Scan(&mv.MembershipID, &mv.UserID, &mv.CompanyID, &mv.CompanyName,
		&mv.Status, &mv.LoginID, &mv.FullName, &mv.AccountStatus, &isPrimaryAdmin)
	if err != nil {
		return nil, err
	}
	mv.IsPrimaryAdmin = isPrimaryAdmin
	return &mv, nil
}

func (r *AdminRepository) CountAdminsInCompany(ctx context.Context, companyID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT mr.membership_id)
		FROM membership_roles mr
		INNER JOIN memberships m ON m.membership_id = mr.membership_id
		INNER JOIN roles ro ON ro.role_id = mr.role_id
		WHERE m.company_id = ? AND m.membership_status = 'active'
		  AND ro.role_code = 'company_admin' AND mr.status = 'active'
	`, companyID).Scan(&count)
	return count, err
}

func (r *AdminRepository) ListUsersWithNoMembership(ctx context.Context) ([]caapp.MembershipView, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT '' AS membership_id,
		       u.user_id,
		       '' AS company_id,
		       '' AS company_name,
		       '' AS membership_status,
		       u.login_id, u.full_name, u.account_status
		FROM users u
		WHERE NOT EXISTS (SELECT 1 FROM memberships m WHERE m.user_id = u.user_id)
		ORDER BY u.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []caapp.MembershipView
	for rows.Next() {
		var v caapp.MembershipView
		if err := rows.Scan(&v.MembershipID, &v.UserID, &v.CompanyID, &v.CompanyName, &v.Status,
			&v.LoginID, &v.FullName, &v.AccountStatus); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *AdminRepository) CountMembershipsForUser(ctx context.Context, userID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memberships WHERE user_id = ?`, userID).Scan(&n)
	return n, err
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
		SELECT permission_code FROM permissions WHERE status = 'active' ORDER BY permission_code
	`)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
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
		SELECT m.membership_id, m.user_id, m.company_id, c.company_name, m.membership_status,
		       u.login_id, u.full_name, u.account_status
		FROM memberships m
		INNER JOIN companies c ON c.company_id = m.company_id
		INNER JOIN users u ON u.user_id = m.user_id
		WHERE m.membership_id = ?
	`, membershipID)
	var v caapp.MembershipView
	if err := row.Scan(&v.MembershipID, &v.UserID, &v.CompanyID, &v.CompanyName, &v.Status,
		&v.LoginID, &v.FullName, &v.AccountStatus); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeMembershipNotFound, "membership not found", nil)
		}
		return nil, err
	}
	return &v, nil
}

func (r *AdminRepository) getUserView(ctx context.Context, userID string) (*caapp.UserView, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT user_id, login_id, full_name, COALESCE(email, ''), COALESCE(phone, ''), account_status
		FROM users
		WHERE user_id = ?
	`, userID)
	var v caapp.UserView
	if err := row.Scan(&v.UserID, &v.LoginID, &v.FullName, &v.Email, &v.Phone, &v.AccountStatus); err != nil {
		if err == sql.ErrNoRows {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "user not found", nil)
		}
		return nil, err
	}
	return &v, nil
}

func (r *AdminRepository) LookupRoleIDForInvite(ctx context.Context, companyID, preferRoleID, preferRoleCode, defaultRoleCode string) (string, error) {
	companyID = strings.TrimSpace(companyID)
	preferRoleID = strings.TrimSpace(preferRoleID)
	preferRoleCode = strings.TrimSpace(strings.ToLower(preferRoleCode))
	defaultRoleCode = strings.TrimSpace(strings.ToLower(defaultRoleCode))

	if preferRoleID != "" {
		var rCompany sql.NullString
		err := r.db.QueryRowContext(ctx, `SELECT company_id FROM roles WHERE role_id = ? AND status = 'active'`, preferRoleID).Scan(&rCompany)
		if err == sql.ErrNoRows {
			return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "role not found", nil)
		}
		if err != nil {
			return "", err
		}
		if rCompany.Valid && rCompany.String != "" && rCompany.String != companyID {
			return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "role does not belong to target company", nil)
		}
		return preferRoleID, nil
	}

	if preferRoleCode != "" {
		var rid string
		err := r.db.QueryRowContext(ctx, `
			SELECT role_id FROM roles
			WHERE status = 'active' AND LOWER(TRIM(role_code)) = ?
			  AND (company_id IS NULL OR company_id = ?)
			ORDER BY CASE WHEN company_id = ? THEN 0 WHEN company_id IS NULL THEN 1 ELSE 2 END
			LIMIT 1
		`, preferRoleCode, companyID, companyID).Scan(&rid)
		if err == sql.ErrNoRows {
			return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "role_code not found for this company", nil)
		}
		if err != nil {
			return "", err
		}
		return rid, nil
	}

	tryCodes := []string{}
	seen := map[string]struct{}{}
	add := func(c string) {
		c = strings.TrimSpace(strings.ToLower(c))
		if c == "" {
			return
		}
		if _, ok := seen[c]; ok {
			return
		}
		seen[c] = struct{}{}
		tryCodes = append(tryCodes, c)
	}
	add(defaultRoleCode)
	add("user_thuong")
	add("viewer")
	add("dashboard_only")

	for _, code := range tryCodes {
		var rid string
		err := r.db.QueryRowContext(ctx, `
			SELECT role_id FROM roles
			WHERE status = 'active' AND LOWER(TRIM(role_code)) = ?
			  AND (company_id IS NULL OR company_id = ?)
			ORDER BY CASE WHEN company_id = ? THEN 0 WHEN company_id IS NULL THEN 1 ELSE 2 END
			LIMIT 1
		`, code, companyID, companyID).Scan(&rid)
		if err == nil {
			return rid, nil
		}
		if err != sql.ErrNoRows {
			return "", err
		}
	}
	return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "no default member role found for company (add roles or pass role_id/role_code)", nil)
}

func (r *AdminRepository) ListInviteRolesForCompany(ctx context.Context, companyID string) ([]caapp.InviteRoleOption, error) {
	companyID = strings.TrimSpace(companyID)
	if companyID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id is required", nil)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.role_id, r.role_code, r.role_name,
		       COALESCE(GROUP_CONCAT(rdgp.permission_code ORDER BY rdgp.permission_code SEPARATOR ','), '') AS default_permissions
		FROM roles r
		LEFT JOIN role_default_grant_permissions rdgp ON rdgp.role_id = r.role_id
		WHERE r.status = 'active' AND (r.company_id IS NULL OR r.company_id = ?)
		GROUP BY r.role_id, r.role_code, r.role_name
		ORDER BY r.role_code
	`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list invite roles: %w", err)
	}
	defer rows.Close()
	var out []caapp.InviteRoleOption
	for rows.Next() {
		var o caapp.InviteRoleOption
		var defaultPermsCSV string
		if err := rows.Scan(&o.RoleID, &o.RoleCode, &o.RoleName, &defaultPermsCSV); err != nil {
			return nil, err
		}
		if defaultPermsCSV != "" {
			o.DefaultPermissions = strings.Split(defaultPermsCSV, ",")
		} else {
			o.DefaultPermissions = []string{}
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (r *AdminRepository) InsertDirectPermission(ctx context.Context, membershipID, companyID, permCode, grantedBy string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO membership_direct_permissions (membership_id, company_id, permission_code, granted_by)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE revoked_at = NULL, revoked_by = NULL, granted_by = VALUES(granted_by), granted_at = CURRENT_TIMESTAMP
	`, membershipID, companyID, permCode, grantedBy)
	if err != nil {
		return fmt.Errorf("insert direct permission: %w", err)
	}
	return nil
}

func (r *AdminRepository) RevokeDirectPermission(ctx context.Context, membershipID, permCode, revokedBy string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE membership_direct_permissions
		SET revoked_at = CURRENT_TIMESTAMP, revoked_by = ?
		WHERE membership_id = ? AND permission_code = ? AND revoked_at IS NULL
	`, revokedBy, membershipID, permCode)
	if err != nil {
		return fmt.Errorf("revoke direct permission: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "active direct permission grant not found", nil)
	}
	return nil
}

func (r *AdminRepository) ListActiveDirectPermissions(ctx context.Context, membershipID string) ([]caapp.DirectPermissionView, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT permission_code, granted_by, granted_at
		FROM membership_direct_permissions
		WHERE membership_id = ? AND revoked_at IS NULL
		ORDER BY permission_code
	`, membershipID)
	if err != nil {
		return nil, fmt.Errorf("list direct permissions: %w", err)
	}
	defer rows.Close()
	var out []caapp.DirectPermissionView
	for rows.Next() {
		var v caapp.DirectPermissionView
		var grantedAt []byte
		if err := rows.Scan(&v.PermissionCode, &v.GrantedBy, &grantedAt); err != nil {
			return nil, err
		}
		v.GrantedAt = string(grantedAt)
		out = append(out, v)
	}
	if out == nil {
		out = []caapp.DirectPermissionView{}
	}
	return out, rows.Err()
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

// mapMySQLSchemaErr turns schema drift (missing migration) into a clear API error instead of a generic 500.
func mapMySQLSchemaErr(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := perr.AsHTTPError(err); ok {
		return err
	}
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		switch me.Number {
		case 1054: // ER_BAD_FIELD_ERROR — Unknown column
			return perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInvalidRequest,
				"Database schema is missing columns required for company APIs (companies.tax_code, verification_status, etc.). Apply migrations through 0029_company_profile_fields, then retry.",
				err)
		case 1146: // ER_NO_SUCH_TABLE
			return perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeInvalidRequest,
				"Database schema is missing a required table. Apply latest migrations from cobo_iam_services/migrations and retry.",
				err)
		}
	}
	return err
}
