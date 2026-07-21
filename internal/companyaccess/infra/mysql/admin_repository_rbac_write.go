package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func (r *AdminRepository) CreateTenantCustomRole(ctx context.Context, in caapp.CreateTenantCustomRoleInput) (*caapp.RoleListItem, error) {
	companyID := strings.TrimSpace(in.CompanyID)
	roleID := strings.TrimSpace(in.RoleID)
	roleCode := strings.TrimSpace(in.RoleCode)
	roleName := strings.TrimSpace(in.RoleName)
	if companyID == "" || roleID == "" || roleCode == "" || roleName == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "role fields required", nil)
	}
	now := time.Now().UTC()
	var desc any
	if strings.TrimSpace(in.Description) != "" {
		desc = in.Description
	}
	var createdBy any
	if strings.TrimSpace(in.CreatedBy) != "" {
		createdBy = in.CreatedBy
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO roles (
			role_id, company_id, role_code, role_name, status,
			role_type, is_protected, description, created_by, updated_by, created_at, updated_at
		) VALUES (?, ?, ?, ?, 'active', 'tenant_custom', 0, ?, ?, ?, ?, ?)
	`, roleID, companyID, roleCode, roleName, desc, createdBy, createdBy, now, now)
	if err != nil {
		if isMySQLDuplicate(err) {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "role_code already exists", nil)
		}
		return nil, fmt.Errorf("insert tenant_custom role: %w", err)
	}
	return r.GetCompanyRoleByID(ctx, companyID, roleID)
}

func (r *AdminRepository) UpdateTenantCustomRoleMetadata(ctx context.Context, companyID, roleID, roleName, description, updatedBy string) (*caapp.RoleListItem, error) {
	companyID = strings.TrimSpace(companyID)
	roleID = strings.TrimSpace(roleID)
	roleName = strings.TrimSpace(roleName)
	if companyID == "" || roleID == "" || roleName == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "role fields required", nil)
	}
	var desc any
	if strings.TrimSpace(description) != "" {
		desc = description
	}
	var updatedByVal any
	if strings.TrimSpace(updatedBy) != "" {
		updatedByVal = updatedBy
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE roles
		SET role_name = ?, description = ?, updated_by = ?, updated_at = UTC_TIMESTAMP()
		WHERE role_id = ?
		  AND company_id = ?
		  AND role_type = 'tenant_custom'
		  AND is_protected = 0
		  AND status = 'active'
	`, roleName, desc, updatedByVal, roleID, companyID)
	if err != nil {
		return nil, fmt.Errorf("update tenant_custom role: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "role not found", nil)
	}
	return r.GetCompanyRoleByID(ctx, companyID, roleID)
}

func (r *AdminRepository) InactivateTenantCustomRole(ctx context.Context, companyID, roleID, updatedBy string) error {
	companyID = strings.TrimSpace(companyID)
	roleID = strings.TrimSpace(roleID)
	if companyID == "" || roleID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "role fields required", nil)
	}
	var updatedByVal any
	if strings.TrimSpace(updatedBy) != "" {
		updatedByVal = updatedBy
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE roles
		SET status = 'inactive', updated_by = ?, updated_at = UTC_TIMESTAMP()
		WHERE role_id = ?
		  AND company_id = ?
		  AND role_type = 'tenant_custom'
		  AND is_protected = 0
		  AND status = 'active'
	`, updatedByVal, roleID, companyID)
	if err != nil {
		return fmt.Errorf("inactivate tenant_custom role: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "role not found", nil)
	}
	return nil
}

func (r *AdminRepository) CountActiveMembershipsForRole(ctx context.Context, companyID, roleID string) (int, error) {
	companyID = strings.TrimSpace(companyID)
	roleID = strings.TrimSpace(roleID)
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT mr.membership_id)
		FROM membership_roles mr
		INNER JOIN memberships m ON m.membership_id = mr.membership_id
			AND m.company_id = ?
			AND LOWER(m.membership_status) != 'deleted'
		WHERE mr.role_id = ? AND mr.status = 'active'
	`, companyID, roleID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count memberships for role: %w", err)
	}
	return n, nil
}

func (r *AdminRepository) GetCompanyRoleByID(ctx context.Context, companyID, roleID string) (*caapp.RoleListItem, error) {
	companyID = strings.TrimSpace(companyID)
	roleID = strings.TrimSpace(roleID)
	if companyID == "" || roleID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id and role_id required", nil)
	}
	var item caapp.RoleListItem
	var companyIDNull sql.NullString
	var descriptionNull sql.NullString
	var isProtected int
	err := r.db.QueryRowContext(ctx, `
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
		WHERE r.role_id = ?
		  AND (r.company_id IS NULL OR r.company_id = ?)
		LIMIT 1
	`, companyID, roleID, companyID).Scan(
		&item.RoleID, &item.RoleCode, &item.RoleName, &item.Status, &companyIDNull,
		&item.RoleType, &isProtected, &descriptionNull,
		&item.CreatedAt, &item.UpdatedAt,
		&item.PermissionCount, &item.MemberCount,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get company role: %w", err)
	}
	item.IsProtected = isProtected != 0
	if descriptionNull.Valid {
		item.Description = descriptionNull.String
	}
	if companyIDNull.Valid && strings.TrimSpace(companyIDNull.String) != "" {
		item.Scope = "company"
		item.IsBuiltin = false
		// Enforce company ownership for company-scoped roles.
		if strings.TrimSpace(companyIDNull.String) != companyID {
			return nil, nil
		}
	} else {
		item.Scope = "global"
		item.IsBuiltin = true
	}
	item.CreatedAt = item.CreatedAt.UTC()
	item.UpdatedAt = item.UpdatedAt.UTC()
	caapp.FinalizeRoleListItem(&item)
	return &item, nil
}
