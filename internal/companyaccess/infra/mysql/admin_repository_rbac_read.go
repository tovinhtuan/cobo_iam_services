package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func (r *AdminRepository) ListPermissions(ctx context.Context) ([]caapp.PermissionListItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT permission_id, permission_code, permission_name, module_name
		FROM permissions
		WHERE status = 'active'
		ORDER BY module_name, permission_code
	`)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	defer rows.Close()
	out := make([]caapp.PermissionListItem, 0)
	for rows.Next() {
		var item caapp.PermissionListItem
		if err := rows.Scan(&item.PermissionID, &item.PermissionCode, &item.PermissionName, &item.ModuleName); err != nil {
			return nil, err
		}
		item.Description = ""
		item.RiskLevel = caapp.PermissionRiskLevel(item.PermissionCode)
		item.IsGrantable = caapp.IsGrantablePermission(item.PermissionCode)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *AdminRepository) ListRoles(ctx context.Context, companyID string) ([]caapp.RoleListItem, error) {
	companyID = strings.TrimSpace(companyID)
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.role_id, r.role_code, r.role_name, r.status, r.company_id,
		       r.role_type, r.is_protected, r.description,
		       r.created_at, r.updated_at,
		       COALESCE(pc.cnt, 0) AS permission_count,
		       COALESCE(mc.cnt, 0) AS member_count
		FROM roles r
		LEFT JOIN (
			SELECT role_id, COUNT(*) AS cnt
			FROM role_permissions
			WHERE status = 'active'
			GROUP BY role_id
		) pc ON pc.role_id = r.role_id
		LEFT JOIN (
			SELECT mr.role_id, COUNT(DISTINCT mr.membership_id) AS cnt
			FROM membership_roles mr
			INNER JOIN memberships m ON m.membership_id = mr.membership_id
				AND m.company_id = ?
				AND LOWER(m.membership_status) != 'deleted'
			WHERE mr.status = 'active'
			GROUP BY mr.role_id
		) mc ON mc.role_id = r.role_id
		WHERE r.status = 'active' AND (r.company_id IS NULL OR r.company_id = ?)
		ORDER BY r.role_code
	`, companyID, companyID)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()
	out := make([]caapp.RoleListItem, 0)
	for rows.Next() {
		var item caapp.RoleListItem
		var companyIDNull sql.NullString
		var descriptionNull sql.NullString
		var isProtected int
		if err := rows.Scan(
			&item.RoleID, &item.RoleCode, &item.RoleName, &item.Status, &companyIDNull,
			&item.RoleType, &isProtected, &descriptionNull,
			&item.CreatedAt, &item.UpdatedAt,
			&item.PermissionCount, &item.MemberCount,
		); err != nil {
			return nil, err
		}
		item.IsProtected = isProtected != 0
		if descriptionNull.Valid {
			item.Description = descriptionNull.String
		} else {
			item.Description = ""
		}
		if companyIDNull.Valid && strings.TrimSpace(companyIDNull.String) != "" {
			item.Scope = "company"
			item.IsBuiltin = false
		} else {
			item.Scope = "global"
			item.IsBuiltin = true
		}
		item.CreatedAt = item.CreatedAt.UTC()
		item.UpdatedAt = item.UpdatedAt.UTC()
		caapp.FinalizeRoleListItem(&item)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *AdminRepository) RoleAccessibleByCompany(ctx context.Context, companyID, roleID string) (bool, error) {
	companyID = strings.TrimSpace(companyID)
	roleID = strings.TrimSpace(roleID)
	if companyID == "" || roleID == "" {
		return false, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id and role_id required", nil)
	}
	var roleCompany sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT company_id FROM roles WHERE role_id = ? AND status = 'active'
	`, roleID).Scan(&roleCompany)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !roleCompany.Valid || strings.TrimSpace(roleCompany.String) == "" {
		return true, nil
	}
	return strings.TrimSpace(roleCompany.String) == companyID, nil
}

func (r *AdminRepository) ListRolePermissions(ctx context.Context, companyID, roleID string) (*caapp.RolePermissionsView, error) {
	ok, err := r.RoleAccessibleByCompany(ctx, companyID, roleID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "role not found", nil)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.permission_id, p.permission_code, p.permission_name, p.module_name
		FROM role_permissions rp
		INNER JOIN permissions p ON p.permission_id = rp.permission_id AND p.status = 'active'
		WHERE rp.role_id = ? AND rp.status = 'active'
		ORDER BY p.module_name, p.permission_code
	`, roleID)
	if err != nil {
		return nil, fmt.Errorf("list role permissions: %w", err)
	}
	defer rows.Close()
	perms := make([]caapp.PermissionListItem, 0)
	for rows.Next() {
		var item caapp.PermissionListItem
		if err := rows.Scan(&item.PermissionID, &item.PermissionCode, &item.PermissionName, &item.ModuleName); err != nil {
			return nil, err
		}
		item.Description = ""
		item.RiskLevel = caapp.PermissionRiskLevel(item.PermissionCode)
		item.IsGrantable = caapp.IsGrantablePermission(item.PermissionCode)
		perms = append(perms, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &caapp.RolePermissionsView{RoleID: roleID, Permissions: perms}, nil
}

// GetNotificationRuleByCode loads a single notification rule for a company by rule_code.
func (r *AdminRepository) GetNotificationRuleByCode(ctx context.Context, companyID, ruleCode string) (*caapp.NotificationRuleView, error) {
	companyID = strings.TrimSpace(companyID)
	ruleCode = strings.TrimSpace(ruleCode)
	if companyID == "" || ruleCode == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id and rule_code required", nil)
	}
	var id, code, status string
	var raw []byte
	var updatedAt time.Time
	err := r.db.QueryRowContext(ctx, `
		SELECT notification_rule_id, rule_code, status, payload_json, updated_at
		FROM notification_rules
		WHERE company_id = ? AND rule_code = ?
		LIMIT 1
	`, companyID, ruleCode).Scan(&id, &code, &status, &raw, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get notification_rule: %w", err)
	}
	payload := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, fmt.Errorf("decode payload_json: %w", err)
		}
	}
	for _, k := range []string{"notification_rule_id", "company_id", "rule_code", "status"} {
		delete(payload, k)
	}
	return &caapp.NotificationRuleView{
		NotificationRuleID: id,
		RuleCode:           code,
		Status:             status,
		Payload:            payload,
		UpdatedAt:          updatedAt.UTC(),
	}, nil
}
