package app

import (
	"context"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const adminCapablePermission = "rbac.manage"

// IsRoleTypeAssignableForMembership reports whether role_type may be used as a
// primary enterprise membership role (Phase E / D6 primary-only).
// system_global remains assignable when already selectable today (denylist applied separately).
func IsRoleTypeAssignableForMembership(roleType string) bool {
	switch strings.TrimSpace(roleType) {
	case RoleTypeTenantDefault, RoleTypeTenantCustom, RoleTypeSystemGlobal:
		return true
	default:
		return false
	}
}

// assertRoleAssignableForMembership validates same-company eligibility for assign/invite.
// Returns the loaded role on success.
func (s *adminService) assertRoleAssignableForMembership(ctx context.Context, companyID, roleID string) (*RoleListItem, error) {
	companyID = strings.TrimSpace(companyID)
	roleID = strings.TrimSpace(roleID)
	if companyID == "" || roleID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "role_id required", nil)
	}

	role, err := s.repo.GetCompanyRoleByID(ctx, companyID, roleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "role not found for company", nil)
	}
	if !strings.EqualFold(strings.TrimSpace(role.Status), "active") {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeRoleInactive, "role is inactive", nil)
	}
	FinalizeRoleListItem(role)
	if !IsRoleTypeAssignableForMembership(role.RoleType) {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeRoleNotAssignable, "role type is not assignable to memberships", nil)
	}
	if IsEnterpriseInviteRoleDenied(role.RoleCode) {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "department focal must be assigned via focal_department_ids, not dept_lead system role", nil)
	}
	return role, nil
}

func (s *adminService) isRoleAdminCapable(ctx context.Context, companyID, roleID string) (bool, error) {
	view, err := s.repo.ListRolePermissions(ctx, companyID, roleID)
	if err != nil {
		return false, err
	}
	if view == nil {
		return false, nil
	}
	for _, p := range view.Permissions {
		if strings.TrimSpace(p.PermissionCode) == adminCapablePermission {
			return true, nil
		}
	}
	return false, nil
}

func (s *adminService) isMembershipAdminCapable(ctx context.Context, membershipID, companyID string) (bool, error) {
	fromRole, err := s.repo.MembershipHasPermissionFromRole(ctx, membershipID, companyID, adminCapablePermission)
	if err != nil {
		return false, err
	}
	if fromRole {
		return true, nil
	}
	return s.repo.HasActiveDirectPermission(ctx, membershipID, adminCapablePermission)
}

func (s *adminService) countActiveAdminCapableMembers(ctx context.Context, companyID string) (int, error) {
	members, err := s.repo.ListMembershipsByCompany(ctx, companyID)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, m := range members {
		if !strings.EqualFold(strings.TrimSpace(m.Status), "active") {
			continue
		}
		ok, err := s.isMembershipAdminCapable(ctx, m.MembershipID, companyID)
		if err != nil {
			return 0, err
		}
		if ok {
			n++
		}
	}
	return n, nil
}

// assertRoleRemovalKeepsAdmin (ROLE-05) guards removing one role from a membership: the
// primary admin keeps its admin role, and the company keeps at least one admin-capable member.
func (s *adminService) assertRoleRemovalKeepsAdmin(ctx context.Context, companyID, membershipID, roleID string) error {
	roles, err := s.repo.ListMembershipRoles(ctx, membershipID)
	if err != nil {
		return err
	}
	held := false
	for _, r := range roles {
		if r.RoleID == roleID {
			held = true
			break
		}
	}
	if !held {
		return nil // nothing is removed; the removal itself reports a missing binding
	}
	adminRole, err := s.isRoleAdminCapable(ctx, companyID, roleID)
	if err != nil {
		if he, ok := perr.AsHTTPError(err); ok && he.HTTPStatus == http.StatusNotFound {
			return nil // unknown role: the removal itself reports it
		}
		return err
	}
	if !adminRole {
		return nil
	}
	m, err := s.repo.GetMembershipByID(ctx, membershipID)
	if err != nil {
		return err
	}
	if m.IsPrimaryAdmin {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "CANNOT_REMOVE_PRIMARY_ADMIN_ROLE", nil)
	}
	if !strings.EqualFold(strings.TrimSpace(m.Status), "active") {
		return nil // an inactive member is not counted as an admin
	}
	for _, r := range roles {
		if r.RoleID == roleID {
			continue
		}
		other, err := s.isRoleAdminCapable(ctx, companyID, r.RoleID)
		if err != nil {
			if he, ok := perr.AsHTTPError(err); ok && he.HTTPStatus == http.StatusNotFound {
				continue
			}
			return err
		}
		if other {
			return nil // still an admin through another role
		}
	}
	direct, err := s.repo.HasActiveDirectPermission(ctx, membershipID, adminCapablePermission)
	if err != nil || direct {
		return err
	}
	count, err := s.countActiveAdminCapableMembers(ctx, companyID)
	if err != nil {
		return err
	}
	if count <= 1 {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeLastAdminRoleChangeBlocked,
			"cannot remove the admin role of the last admin-capable member", nil)
	}
	return nil
}

// assertPrimaryRoleChangeLockout applies Phase E assignment-local safety (not full R10).
func (s *adminService) assertPrimaryRoleChangeLockout(
	ctx context.Context,
	actor MembershipActor,
	targetMembershipID, companyID, targetRoleID string,
) error {
	targetAdminCapable, err := s.isRoleAdminCapable(ctx, companyID, targetRoleID)
	if err != nil {
		return err
	}

	actorMID := strings.TrimSpace(actor.MembershipID)
	targetMID := strings.TrimSpace(targetMembershipID)

	if actorMID != "" && actorMID == targetMID && !targetAdminCapable {
		return perr.NewHTTPError(
			http.StatusConflict,
			perr.CodeSelfRoleChangeBlocked,
			"cannot change your own role to one without admin capability",
			nil,
		)
	}

	currentAdminCapable, err := s.isMembershipAdminCapable(ctx, targetMID, companyID)
	if err != nil {
		return err
	}
	if !currentAdminCapable || targetAdminCapable {
		return nil
	}

	count, err := s.countActiveAdminCapableMembers(ctx, companyID)
	if err != nil {
		return err
	}
	if count <= 1 {
		return perr.NewHTTPError(
			http.StatusConflict,
			perr.CodeLastAdminRoleChangeBlocked,
			"cannot demote the last admin-capable membership in the company",
			nil,
		)
	}
	return nil
}

// MembershipActor is the minimal actor identity for lockout checks.
type MembershipActor struct {
	MembershipID string
}
