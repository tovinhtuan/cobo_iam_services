package app

import (
	"context"
	"net/http"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type membershipAdminScope struct {
	inviteScope
	delegationPerms map[string]struct{}
	fromDelegation  bool
}

func (s *adminService) resolveMembershipAdminScope(ctx context.Context, sub AdminSubject) (membershipAdminScope, error) {
	ok, err := s.hasPermission(ctx, sub, "rbac.manage")
	if err != nil {
		return membershipAdminScope{}, err
	}
	if ok {
		return membershipAdminScope{inviteScope: inviteScope{Kind: inviteScopeCompany}}, nil
	}

	grants, err := s.repo.ListActiveDelegationsForDelegatee(ctx, sub.CompanyID, sub.MembershipID)
	if err != nil {
		return membershipAdminScope{}, err
	}
	if len(grants) > 0 {
		deptSet := make(map[string]struct{})
		permSet := make(map[string]struct{})
		for _, g := range grants {
			deptSet[g.ScopeID] = struct{}{}
			for _, p := range g.PermissionSet {
				permSet[p] = struct{}{}
			}
		}
		deptIDs := make([]string, 0, len(deptSet))
		for id := range deptSet {
			deptIDs = append(deptIDs, id)
		}
		return membershipAdminScope{
			inviteScope:     inviteScope{Kind: inviteScopeDepartment, DepartmentIDs: deptIDs},
			delegationPerms: permSet,
			fromDelegation:  true,
		}, nil
	}

	legacy, err := s.resolveInviteScopeLegacy(ctx, sub)
	if err != nil {
		return membershipAdminScope{}, err
	}
	return membershipAdminScope{inviteScope: legacy}, nil
}

func (s *adminService) resolveInviteScope(ctx context.Context, sub AdminSubject) (inviteScope, error) {
	scope, err := s.resolveMembershipAdminScope(ctx, sub)
	if err != nil {
		return inviteScope{}, err
	}
	return scope.inviteScope, nil
}

func (s *adminService) resolveInviteScopeLegacy(ctx context.Context, sub AdminSubject) (inviteScope, error) {
	fromRole, err := s.repo.MembershipHasPermissionFromRole(ctx, sub.MembershipID, sub.CompanyID, permissionInvite)
	if err != nil {
		return inviteScope{}, err
	}
	if fromRole {
		return inviteScope{Kind: inviteScopeCompany}, nil
	}
	hasDirect, err := s.repo.HasActiveDirectPermission(ctx, sub.MembershipID, permissionInvite)
	if err != nil {
		return inviteScope{}, err
	}
	if !hasDirect {
		return inviteScope{}, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "access denied", nil)
	}
	deptIDs, err := s.repo.ListDepartmentIDsByHeadMembership(ctx, sub.CompanyID, sub.MembershipID)
	if err != nil {
		return inviteScope{}, err
	}
	if len(deptIDs) == 0 {
		return inviteScope{}, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "dept-scoped invite requires head of at least one department", nil)
	}
	return inviteScope{Kind: inviteScopeDepartment, DepartmentIDs: deptIDs}, nil
}

func (s *adminService) assertDelegationOperationPerm(scope membershipAdminScope, permission string) error {
	if !scope.fromDelegation {
		return nil
	}
	if _, ok := scope.delegationPerms[permission]; !ok {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "operation not in delegated permission_set", nil)
	}
	return nil
}

func (s *adminService) assertMembershipInAdminScope(ctx context.Context, scope membershipAdminScope, membershipID string) error {
	if scope.Kind != inviteScopeDepartment {
		return nil
	}
	ok, err := s.repo.MembershipInAnyDepartment(ctx, membershipID, scope.DepartmentIDs)
	if err != nil {
		return err
	}
	if !ok {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "membership outside delegated department scope", nil)
	}
	return nil
}

func (s *adminService) assertDepartmentInAdminScope(scope membershipAdminScope, departmentID string) error {
	if scope.Kind != inviteScopeDepartment {
		return nil
	}
	for _, d := range scope.DepartmentIDs {
		if d == departmentID {
			return nil
		}
	}
	return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "department outside delegated scope", nil)
}

func (s *adminService) authorizeScopedMembershipMutation(ctx context.Context, sub AdminSubject, permission, membershipID string) error {
	scope, err := s.resolveMembershipAdminScope(ctx, sub)
	if err != nil {
		return err
	}
	if scope.Kind == inviteScopeCompany {
		return nil
	}
	if err := s.assertDelegationOperationPerm(scope, permission); err != nil {
		return err
	}
	return s.assertMembershipInAdminScope(ctx, scope, membershipID)
}

func (s *adminService) authorizeScopedDepartmentMutation(ctx context.Context, sub AdminSubject, permission, departmentID string) error {
	scope, err := s.resolveMembershipAdminScope(ctx, sub)
	if err != nil {
		return err
	}
	if scope.Kind == inviteScopeCompany {
		return nil
	}
	if err := s.assertDelegationOperationPerm(scope, permission); err != nil {
		return err
	}
	return s.assertDepartmentInAdminScope(scope, departmentID)
}

func (s *adminService) authorizeScopedInviteOrCreate(ctx context.Context, sub AdminSubject, permission string) (membershipAdminScope, error) {
	scope, err := s.resolveMembershipAdminScope(ctx, sub)
	if err != nil {
		return membershipAdminScope{}, err
	}
	if scope.Kind == inviteScopeCompany {
		return scope, nil
	}
	if err := s.assertDelegationOperationPerm(scope, permission); err != nil {
		return membershipAdminScope{}, err
	}
	return scope, nil
}
