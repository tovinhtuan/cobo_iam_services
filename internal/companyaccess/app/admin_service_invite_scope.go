package app

import (
	"context"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const permissionInvite = "admin.membership.invite"

type inviteScopeKind string

const (
	inviteScopeCompany    inviteScopeKind = "company"
	inviteScopeDepartment inviteScopeKind = "department"
)

type inviteScope struct {
	Kind          inviteScopeKind
	DepartmentIDs []string
}

func (s *adminService) authorizeMembershipInvite(ctx context.Context, sub AdminSubject, companyID string) error {
	ok, err := s.hasPermission(ctx, sub, "rbac.manage")
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return s.authorize(ctx, sub, "admin.membership.create", companyID)
}

func (s *adminService) pickInviteDepartmentID(scope inviteScope, requested string) (string, error) {
	if scope.Kind != inviteScopeDepartment {
		return strings.TrimSpace(requested), nil
	}
	req := strings.TrimSpace(requested)
	if len(scope.DepartmentIDs) == 1 {
		if req != "" && req != scope.DepartmentIDs[0] {
			return "", perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "department_id not in invite scope", nil)
		}
		return scope.DepartmentIDs[0], nil
	}
	if req == "" {
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "department_id is required when heading multiple departments", nil)
	}
	for _, d := range scope.DepartmentIDs {
		if d == req {
			return req, nil
		}
	}
	return "", perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "department_id not in invite scope", nil)
}

func (s *adminService) assignInviteDepartment(ctx context.Context, membershipID, departmentID string) error {
	if departmentID == "" {
		return nil
	}
	return s.repo.AddDepartment(ctx, membershipID, departmentID)
}

func (s *adminService) filterMembershipsByInviteScope(items []MembershipView, scope inviteScope) ([]MembershipView, error) {
	if scope.Kind != inviteScopeDepartment {
		return items, nil
	}
	allowed := make(map[string]struct{}, len(scope.DepartmentIDs))
	for _, d := range scope.DepartmentIDs {
		allowed[d] = struct{}{}
	}
	var out []MembershipView
	for _, m := range items {
		if m.MembershipID == "" {
			continue
		}
		inScope := false
		for _, dept := range m.Departments {
			if _, ok := allowed[dept.DepartmentID]; ok {
				inScope = true
				break
			}
		}
		if inScope {
			out = append(out, m)
		}
	}
	if out == nil {
		out = []MembershipView{}
	}
	return out, nil
}

func (s *adminService) assertResendInInviteScope(ctx context.Context, scope inviteScope, userID, companyID string) error {
	if scope.Kind != inviteScopeDepartment {
		return nil
	}
	mid, err := s.repo.GetMembershipIDForUserCompany(ctx, userID, companyID)
	if err != nil {
		return err
	}
	if mid == "" {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeMembershipNotFound, "membership not found for user in company", nil)
	}
	ok, err := s.repo.MembershipInAnyDepartment(ctx, mid, scope.DepartmentIDs)
	if err != nil {
		return err
	}
	if !ok {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "cannot resend invitation outside department scope", nil)
	}
	return nil
}

func (s *adminService) GetInviteScope(ctx context.Context, req GetInviteScopeRequest) (*InviteScopeView, error) {
	if err := s.authorizeMembershipInvite(ctx, req.Subject, req.Subject.CompanyID); err != nil {
		return nil, err
	}
	scope, err := s.resolveInviteScope(ctx, req.Subject)
	if err != nil {
		return nil, err
	}
	if scope.Kind == inviteScopeCompany {
		return &InviteScopeView{Scope: "company"}, nil
	}
	depts, err := s.repo.ListCompanyDepartments(ctx, req.Subject.CompanyID)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(scope.DepartmentIDs))
	for _, id := range scope.DepartmentIDs {
		allowed[id] = struct{}{}
	}
	var out []InviteScopeDept
	for _, d := range depts {
		if _, ok := allowed[d.DepartmentID]; !ok {
			continue
		}
		name := d.Name
		if name == "" {
			name = d.DepartmentName
		}
		out = append(out, InviteScopeDept{DepartmentID: d.DepartmentID, DepartmentName: name})
	}
	return &InviteScopeView{Scope: "department", Departments: out}, nil
}

func (s *adminService) assertCanGrantInvitePermission(ctx context.Context, sub AdminSubject) error {
	caller, err := s.repo.GetMembershipByID(ctx, sub.MembershipID)
	if err != nil {
		return err
	}
	if !caller.IsPrimaryAdmin {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "only primary admin can grant invite permission", nil)
	}
	ok, err := s.hasPermission(ctx, sub, "rbac.manage")
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	hasInvite, err := s.hasPermission(ctx, sub, permissionInvite)
	if err != nil {
		return err
	}
	if !hasInvite {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "access denied", nil)
	}
	return nil
}
