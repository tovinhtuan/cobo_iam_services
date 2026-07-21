package app

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const (
	maxCustomRoleNameLen        = 120
	maxCustomRoleDescriptionLen = 2000
)

// CreateCustomRoleRequest creates a company-scoped tenant_custom role with no permissions.
type CreateCustomRoleRequest struct {
	Subject     AdminSubject
	RoleName    string
	Description string
}

// UpdateCustomRoleRequest updates metadata only (role_name, description).
type UpdateCustomRoleRequest struct {
	Subject     AdminSubject
	RoleID      string
	RoleName    *string
	Description *string
}

// InactivateCustomRoleRequest soft-deletes (status=inactive) a tenant_custom role.
type InactivateCustomRoleRequest struct {
	Subject AdminSubject
	RoleID  string
}

// CloneRoleRequest clones grantable permissions from a visible source role into a new tenant_custom role.
type CloneRoleRequest struct {
	Subject      AdminSubject
	SourceRoleID string
	RoleName     string
	Description  string
}

// ClonePermissionEntry is one row in clone copy_summary.
type ClonePermissionEntry struct {
	PermissionCode string              `json:"permission_code"`
	GrantTier      PermissionGrantTier `json:"grant_tier"`
	Reason         string              `json:"reason,omitempty"`
}

// CloneCopySummary reports which permissions were copied vs skipped during clone.
type CloneCopySummary struct {
	CopiedCount         int                    `json:"copied_count"`
	SkippedCount        int                    `json:"skipped_count"`
	CopiedPermissions   []ClonePermissionEntry `json:"copied_permissions"`
	SkippedPermissions  []ClonePermissionEntry `json:"skipped_permissions"`
}

// CloneRoleResult is the response for POST .../roles/{id}/clone.
type CloneRoleResult struct {
	Role        RoleListItem     `json:"role"`
	CopySummary CloneCopySummary `json:"copy_summary"`
}

// CreateTenantCustomRoleInput is the repository insert payload.
type CreateTenantCustomRoleInput struct {
	RoleID      string
	CompanyID   string
	RoleCode    string
	RoleName    string
	Description string
	CreatedBy   string
}

func (s *adminService) CreateCustomRole(ctx context.Context, req CreateCustomRoleRequest) (*RoleListItem, error) {
	if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return nil, err
	}
	name, desc, err := validateCustomRoleNameDescription(req.RoleName, req.Description)
	if err != nil {
		return nil, err
	}
	companyID := strings.TrimSpace(req.Subject.CompanyID)
	if companyID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeCompanyContextRequired, "company context required", nil)
	}
	roleID := s.idg.NewUUID()
	roleCode := generateCustomRoleCode(name, roleID)
	item, err := s.repo.CreateTenantCustomRole(ctx, CreateTenantCustomRoleInput{
		RoleID:      roleID,
		CompanyID:   companyID,
		RoleCode:    roleCode,
		RoleName:    name,
		Description: desc,
		CreatedBy:   req.Subject.UserID,
	})
	if err != nil {
		return nil, err
	}
	FinalizeRoleListItem(item)
	return item, nil
}

func (s *adminService) UpdateCustomRole(ctx context.Context, req UpdateCustomRoleRequest) (*RoleListItem, error) {
	if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return nil, err
	}
	role, err := s.requireEditableTenantCustomRole(ctx, req.Subject, req.RoleID)
	if err != nil {
		return nil, err
	}
	name := role.RoleName
	desc := role.Description
	if req.RoleName != nil {
		n, _, err := validateCustomRoleNameDescription(*req.RoleName, "")
		if err != nil {
			return nil, err
		}
		name = n
	}
	if req.Description != nil {
		d := strings.TrimSpace(*req.Description)
		if len(d) > maxCustomRoleDescriptionLen {
			return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "description too long", nil)
		}
		desc = d
	}
	item, err := s.repo.UpdateTenantCustomRoleMetadata(ctx, req.Subject.CompanyID, role.RoleID, name, desc, req.Subject.UserID)
	if err != nil {
		return nil, err
	}
	FinalizeRoleListItem(item)
	return item, nil
}

func (s *adminService) InactivateCustomRole(ctx context.Context, req InactivateCustomRoleRequest) error {
	if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return err
	}
	role, err := s.requireEditableTenantCustomRole(ctx, req.Subject, req.RoleID)
	if err != nil {
		return err
	}
	n, err := s.repo.CountActiveMembershipsForRole(ctx, req.Subject.CompanyID, role.RoleID)
	if err != nil {
		return err
	}
	if n > 0 {
		return perr.NewHTTPError(
			http.StatusConflict,
			perr.CodeRoleInUse,
			"Vai trò đang được gán cho thành viên; không thể vô hiệu hóa.",
			nil,
		)
	}
	return s.repo.InactivateTenantCustomRole(ctx, req.Subject.CompanyID, role.RoleID, req.Subject.UserID)
}

func (s *adminService) CloneRole(ctx context.Context, req CloneRoleRequest) (*CloneRoleResult, error) {
	if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return nil, err
	}
	name, desc, err := validateCustomRoleNameDescription(req.RoleName, req.Description)
	if err != nil {
		return nil, err
	}
	sourceID := strings.TrimSpace(req.SourceRoleID)
	if sourceID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "source_role_id required", nil)
	}
	ok, err := s.repo.RoleAccessibleByCompany(ctx, req.Subject.CompanyID, sourceID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "role not found", nil)
	}
	sourcePerms, err := s.repo.ListRolePermissions(ctx, req.Subject.CompanyID, sourceID)
	if err != nil {
		return nil, err
	}

	summary := CloneCopySummary{
		CopiedPermissions:  make([]ClonePermissionEntry, 0),
		SkippedPermissions: make([]ClonePermissionEntry, 0),
	}
	toCopy := make([]PermissionListItem, 0)
	for _, p := range sourcePerms.Permissions {
		policy := LookupGrantPolicy(p.PermissionCode)
		if policy.GrantTier == GrantTierGrantable && policy.AllowedOnCustomRole {
			summary.CopiedPermissions = append(summary.CopiedPermissions, ClonePermissionEntry{
				PermissionCode: p.PermissionCode,
				GrantTier:      policy.GrantTier,
			})
			toCopy = append(toCopy, p)
			continue
		}
		reason := cloneSkipReason(policy)
		summary.SkippedPermissions = append(summary.SkippedPermissions, ClonePermissionEntry{
			PermissionCode: p.PermissionCode,
			GrantTier:      policy.GrantTier,
			Reason:         reason,
		})
	}
	summary.CopiedCount = len(summary.CopiedPermissions)
	summary.SkippedCount = len(summary.SkippedPermissions)

	roleID := s.idg.NewUUID()
	roleCode := generateCustomRoleCode(name, roleID)
	item, err := s.repo.CreateTenantCustomRole(ctx, CreateTenantCustomRoleInput{
		RoleID:      roleID,
		CompanyID:   req.Subject.CompanyID,
		RoleCode:    roleCode,
		RoleName:    name,
		Description: desc,
		CreatedBy:   req.Subject.UserID,
	})
	if err != nil {
		return nil, err
	}
	for _, p := range toCopy {
		pid := strings.TrimSpace(p.PermissionID)
		if pid == "" {
			pid = p.PermissionCode
		}
		if err := s.repo.AddRolePermission(ctx, roleID, pid); err != nil {
			return nil, fmt.Errorf("clone add permission %s: %w", p.PermissionCode, err)
		}
	}
	// Reload for accurate permission_count.
	reloaded, err := s.repo.GetCompanyRoleByID(ctx, req.Subject.CompanyID, roleID)
	if err != nil {
		return nil, err
	}
	if reloaded != nil {
		item = reloaded
	}
	FinalizeRoleListItem(item)
	item.PermissionCount = summary.CopiedCount
	return &CloneRoleResult{Role: *item, CopySummary: summary}, nil
}

func (s *adminService) requireEditableTenantCustomRole(ctx context.Context, sub AdminSubject, roleID string) (*RoleListItem, error) {
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "role_id required", nil)
	}
	role, err := s.repo.GetCompanyRoleByID(ctx, sub.CompanyID, roleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "role not found", nil)
	}
	FinalizeRoleListItem(role)
	if IsRoleProtectedForMutation(role) || role.RoleType != RoleTypeTenantCustom {
		return nil, ErrProtectedRoleReadOnly()
	}
	if !strings.EqualFold(strings.TrimSpace(role.Status), "active") {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "role not found", nil)
	}
	return role, nil
}

func validateCustomRoleNameDescription(roleName, description string) (string, string, error) {
	name := strings.TrimSpace(roleName)
	if name == "" {
		return "", "", perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRoleName, "Tên vai trò là bắt buộc.", nil)
	}
	if len(name) > maxCustomRoleNameLen {
		return "", "", perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRoleName, "Tên vai trò quá dài.", nil)
	}
	desc := strings.TrimSpace(description)
	if len(desc) > maxCustomRoleDescriptionLen {
		return "", "", perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "description too long", nil)
	}
	return name, desc, nil
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func generateCustomRoleCode(roleName, roleID string) string {
	slug := strings.ToLower(roleName)
	var b strings.Builder
	for _, r := range slug {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if unicode.IsSpace(r) || r == '-' || r == '_' {
			b.WriteByte('_')
		}
	}
	slug = nonSlug.ReplaceAllString(b.String(), "_")
	slug = strings.Trim(slug, "_")
	if slug == "" {
		slug = "role"
	}
	if len(slug) > 40 {
		slug = slug[:40]
		slug = strings.Trim(slug, "_")
	}
	suffix := strings.ReplaceAll(roleID, "-", "")
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	return fmt.Sprintf("custom_%s_%s", slug, suffix)
}

func cloneSkipReason(policy PermissionGrantPolicy) string {
	switch policy.GrantTier {
	case GrantTierHighRisk:
		return "high_risk_permission_requires_approval"
	case GrantTierTenantAdminOnly:
		return "permission_not_grantable"
	case GrantTierSystemOnly:
		return "permission_not_grantable"
	default:
		return "permission_not_grantable"
	}
}
