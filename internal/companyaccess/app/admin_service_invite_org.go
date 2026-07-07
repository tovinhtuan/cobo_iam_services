package app

import (
	"context"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func normalizeFocalDepartmentIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (s *adminService) validateEnterpriseInviteRole(ctx context.Context, companyID, roleID, roleCode, defaultRoleCode string, isPlatformCMS bool) (string, error) {
	if isPlatformCMS {
		return s.repo.LookupRoleIDForInvite(ctx, companyID, roleID, roleCode, defaultRoleCode)
	}
	checkCode := strings.TrimSpace(strings.ToLower(roleCode))
	if checkCode != "" && IsEnterpriseInviteRoleDenied(checkCode) {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "department focal must be assigned via focal_department_ids, not dept_lead system role", nil)
	}
	if strings.TrimSpace(roleID) != "" {
		roles, err := s.repo.ListInviteRolesForCompany(ctx, companyID)
		if err != nil {
			return "", err
		}
		for _, role := range roles {
			if role.RoleID == strings.TrimSpace(roleID) && IsEnterpriseInviteRoleDenied(role.RoleCode) {
				return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "department focal must be assigned via focal_department_ids, not dept_lead system role", nil)
			}
		}
	}
	roleIDResolved, err := s.repo.LookupRoleIDForInvite(ctx, companyID, roleID, roleCode, defaultRoleCode)
	if err != nil {
		return "", err
	}
	roles, err := s.repo.ListInviteRolesForCompany(ctx, companyID)
	if err != nil {
		return "", err
	}
	for _, role := range roles {
		if role.RoleID == roleIDResolved && IsEnterpriseInviteRoleDenied(role.RoleCode) {
			return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "department focal must be assigned via focal_department_ids, not dept_lead system role", nil)
		}
	}
	return roleIDResolved, nil
}

func validateEnterpriseInvitePermissions(perms []string) error {
	deniedDirect := map[string]struct{}{
		"rbac.manage":                                       {},
		"system.settings":                                   {},
		"template.workflow.override.read":                   {},
		"admin.membership.invite":                           {},
		"template.workflow.override.approve":                {},
		"template.workflow.override.reset":                  {},
		"template.workflow.override.department_group.write": {},
	}
	for _, p := range perms {
		if !isGrantable(p) {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "permission_code is not grantable: "+p, nil)
		}
		if _, blocked := deniedDirect[p]; blocked {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "permission_code is not allowed for enterprise invite: "+p, nil)
		}
		if !isEnterpriseInvitePermission(p) {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "permission_code is not allowed for enterprise invite: "+p, nil)
		}
	}
	return nil
}

func (s *adminService) validateInviteOrgFields(isDepartmentFocal bool, focalDepartmentIDs []string) error {
	if !isDepartmentFocal {
		return nil
	}
	if len(focalDepartmentIDs) == 0 {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "focal_department_ids is required when is_department_focal is true", nil)
	}
	return nil
}

func (s *adminService) assertFocalDepartmentsInInviteScope(scope inviteScope, focalDepartmentIDs []string) error {
	if scope.Kind != inviteScopeDepartment {
		return nil
	}
	allowed := make(map[string]struct{}, len(scope.DepartmentIDs))
	for _, id := range scope.DepartmentIDs {
		allowed[id] = struct{}{}
	}
	for _, deptID := range focalDepartmentIDs {
		if _, ok := allowed[deptID]; !ok {
			return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "department_id not in invite scope", nil)
		}
	}
	return nil
}
