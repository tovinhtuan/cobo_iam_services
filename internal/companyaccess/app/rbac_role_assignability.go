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

// withCompanyAdminLock (ROLE-23) runs fn holding the company's admin-change lock, so the
// last-admin check in fn and its write cannot interleave with another such change.
func (s *adminService) withCompanyAdminLock(ctx context.Context, companyID string, fn func() error) error {
	release, err := s.repo.LockCompanyAdmins(ctx, companyID)
	if err != nil {
		return err
	}
	defer release()
	return fn()
}

// assertMemberLeavingKeepsAdmin (ROLE-23) guards deactivating or deleting a membership: the
// company keeps at least one active admin-capable member. Run it under withCompanyAdminLock.
func (s *adminService) assertMemberLeavingKeepsAdmin(ctx context.Context, companyID, membershipID string) error {
	m, err := s.repo.GetMembershipByID(ctx, membershipID)
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(m.Status), "active") {
		return nil // an inactive member is not counted as an admin
	}
	capable, err := s.isMembershipAdminCapable(ctx, membershipID, companyID)
	if err != nil || !capable {
		return err
	}
	count, err := s.countActiveAdminCapableMembers(ctx, companyID)
	if err != nil {
		return err
	}
	if count <= 1 {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeLastAdminRoleChangeBlocked,
			"cannot deactivate or remove the last admin-capable member", nil)
	}
	return nil
}

// assertRBACPlanKeepsAdmin (ROLE-23, role level) refuses an RBAC restore or approval plan that
// would leave a company with admin-capable members with none: every active member's roles and
// direct grants are re-evaluated with the plan's rbac.manage changes applied. Run it under
// withCompanyAdminLock when the plan is applied right after.
func (s *adminService) assertRBACPlanKeepsAdmin(ctx context.Context, companyID string, plan RBACRestorePlan, direct RBACDirectRestorePlan) error {
	roleAfter := map[string]bool{}   // role_id -> holds rbac.manage after the plan
	directAfter := map[string]bool{} // membership_id -> holds rbac.manage directly after the plan
	removes := false
	for _, op := range plan.Ops {
		if op.PermissionCode == adminCapablePermission {
			roleAfter[op.RoleID] = op.Add
			removes = removes || !op.Add
		}
	}
	for _, d := range direct.Revoke {
		if d.PermissionCode == adminCapablePermission {
			directAfter[d.MembershipID] = false
			removes = true
		}
	}
	for _, d := range direct.Grant {
		if d.PermissionCode == adminCapablePermission {
			directAfter[d.MembershipID] = true
		}
	}
	if !removes {
		return nil
	}
	members, err := s.repo.ListMembershipsByCompany(ctx, companyID)
	if err != nil {
		return err
	}
	now, after := 0, 0
	for _, m := range members {
		if !strings.EqualFold(strings.TrimSpace(m.Status), "active") {
			continue
		}
		capNow, capAfter, err := s.adminCapabilityAfter(ctx, companyID, m.MembershipID, roleAfter, directAfter)
		if err != nil {
			return err
		}
		if capNow {
			now++
		}
		if capAfter {
			after++
		}
	}
	if now > 0 && after == 0 {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeLastAdminRoleChangeBlocked,
			"the change would leave the company without an admin-capable member", nil)
	}
	return nil
}

// adminCapabilityAfter reports whether a member holds rbac.manage now and after the given role and
// direct-grant changes.
func (s *adminService) adminCapabilityAfter(ctx context.Context, companyID, membershipID string, roleAfter, directAfter map[string]bool) (now, after bool, err error) {
	roles, err := s.repo.ListMembershipRoles(ctx, membershipID)
	if err != nil {
		return false, false, err
	}
	for _, r := range roles {
		has, err := s.isRoleAdminCapable(ctx, companyID, r.RoleID)
		if err != nil {
			if he, ok := perr.AsHTTPError(err); ok && he.HTTPStatus == http.StatusNotFound {
				continue
			}
			return false, false, err
		}
		now = now || has
		if v, ok := roleAfter[r.RoleID]; ok {
			has = v
		}
		after = after || has
	}
	has, err := s.repo.HasActiveDirectPermission(ctx, membershipID, adminCapablePermission)
	if err != nil {
		return false, false, err
	}
	now = now || has
	if v, ok := directAfter[membershipID]; ok {
		has = v
	}
	return now, after || has, nil
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
