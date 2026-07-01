package app

import (
	"context"
	"net/http"
	"strings"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

func (s *adminService) CreateDelegation(ctx context.Context, req CreateDelegationRequest) (*DelegatedAdminGrant, error) {
	if err := s.requireDelegationGrantAuthority(ctx, req.Subject); err != nil {
		return nil, err
	}
	delegateeID := strings.TrimSpace(req.DelegateeMembershipID)
	scopeType := strings.TrimSpace(req.ScopeType)
	scopeID := strings.TrimSpace(req.ScopeID)
	if delegateeID == "" || scopeType == "" || scopeID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "delegatee_membership_id, scope_type, and scope_id are required", nil)
	}
	if scopeType != DelegationScopeTypeDepartment {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "unsupported scope_type", nil)
	}
	if delegateeID == req.Subject.MembershipID {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "self-grant is not allowed", nil)
	}
	if err := s.requireMembershipInCompany(ctx, delegateeID, req.Subject.CompanyID); err != nil {
		return nil, err
	}
	delegatee, err := s.repo.GetMembershipByID(ctx, delegateeID)
	if err != nil {
		return nil, err
	}
	if delegatee.IsPrimaryAdmin {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "cannot delegate to primary admin", nil)
	}
	if !strings.EqualFold(strings.TrimSpace(delegatee.Status), "active") {
		return nil, perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeInvalidRequest, "delegatee membership must be active", nil)
	}
	if !s.departmentInCompany(ctx, req.Subject.CompanyID, scopeID) {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "scope_id is not a valid department in company", nil)
	}
	normalized, err := s.normalizeDelegationPermissionSet(ctx, req.Subject, req.PermissionSet)
	if err != nil {
		return nil, err
	}
	if len(normalized) == 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "permission_set is required", nil)
	}
	id := s.idg.NewUUID()
	row, err := s.repo.InsertDelegationGrant(ctx, InsertDelegationGrantInput{
		ID:                      id,
		CompanyID:               req.Subject.CompanyID,
		DelegateeMembershipID:   delegateeID,
		DelegatorMembershipID:   req.Subject.MembershipID,
		ScopeType:               scopeType,
		ScopeID:                 scopeID,
		PermissionSet:           normalized,
		CreatedBy:               req.Subject.MembershipID,
	})
	if err != nil {
		return nil, err
	}
	s.appendDelegationAudit(ctx, req.Subject, "delegated.admin.granted", row, nil)
	return row, nil
}

func (s *adminService) ListDelegations(ctx context.Context, req ListDelegationsRequest) (*DelegationListView, error) {
	if err := s.requireDelegationGrantAuthority(ctx, req.Subject); err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	items, err := s.repo.ListDelegationGrants(ctx, req.Subject.CompanyID, strings.TrimSpace(req.Status), strings.TrimSpace(req.DelegateeMembershipID), strings.TrimSpace(req.ScopeID), limit)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []DelegatedAdminGrant{}
	}
	return &DelegationListView{Items: items, Total: len(items)}, nil
}

func (s *adminService) GetDelegation(ctx context.Context, req GetDelegationRequest) (*DelegatedAdminGrant, error) {
	if err := s.requireDelegationGrantAuthority(ctx, req.Subject); err != nil {
		return nil, err
	}
	return s.repo.GetDelegationGrant(ctx, req.Subject.CompanyID, strings.TrimSpace(req.DelegationID))
}

func (s *adminService) PatchDelegation(ctx context.Context, req PatchDelegationRequest) (*DelegatedAdminGrant, error) {
	if err := s.requireDelegationGrantAuthority(ctx, req.Subject); err != nil {
		return nil, err
	}
	normalized, err := s.normalizeDelegationPermissionSet(ctx, req.Subject, req.PermissionSet)
	if err != nil {
		return nil, err
	}
	if len(normalized) == 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "permission_set is required", nil)
	}
	row, err := s.repo.UpdateDelegationGrantPermissions(ctx, req.Subject.CompanyID, strings.TrimSpace(req.DelegationID), normalized, req.Subject.MembershipID)
	if err != nil {
		return nil, err
	}
	s.appendDelegationAudit(ctx, req.Subject, "delegated.admin.updated", row, nil)
	return row, nil
}

func (s *adminService) RevokeDelegation(ctx context.Context, req RevokeDelegationRequest) (*DelegatedAdminGrant, error) {
	if err := s.requireDelegationGrantAuthority(ctx, req.Subject); err != nil {
		return nil, err
	}
	row, err := s.repo.RevokeDelegationGrant(ctx, req.Subject.CompanyID, strings.TrimSpace(req.DelegationID), req.Subject.MembershipID)
	if err != nil {
		return nil, err
	}
	s.appendDelegationAudit(ctx, req.Subject, "delegated.admin.revoked", row, nil)
	return row, nil
}

func (s *adminService) requireDelegationGrantAuthority(ctx context.Context, sub AdminSubject) error {
	ok, err := s.hasPermission(ctx, sub, "rbac.manage")
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	ok, err = s.hasPermission(ctx, sub, "system.settings")
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	caller, err := s.repo.GetMembershipByID(ctx, sub.MembershipID)
	if err != nil {
		return err
	}
	if caller.IsPrimaryAdmin {
		return nil
	}
	return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "delegation grant authority required", nil)
}

func (s *adminService) normalizeDelegationPermissionSet(ctx context.Context, sub AdminSubject, requested []string) ([]string, error) {
	if len(requested) == 0 {
		return nil, nil
	}
	bypassHolderCheck, err := s.canBypassDelegationPermissionHolderCheck(ctx, sub)
	if err != nil {
		return nil, err
	}
	var holder map[string]struct{}
	if !bypassHolderCheck {
		eff, err := s.auth.GetEffectiveAccess(ctx, sub.MembershipID, sub.CompanyID)
		if err != nil {
			return nil, err
		}
		holder = delegatorEffectivePermissions(eff)
	}
	seen := make(map[string]struct{})
	var out []string
	for _, raw := range requested {
		code := strings.TrimSpace(raw)
		if code == "" {
			continue
		}
		if isForbiddenDelegationPermission(code) {
			return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "permission is not delegatable: "+code, nil)
		}
		if !bypassHolderCheck {
			if _, ok := holder[code]; !ok {
				return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "delegator does not hold permission: "+code, nil)
			}
		}
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out, nil
}

func (s *adminService) canBypassDelegationPermissionHolderCheck(ctx context.Context, sub AdminSubject) (bool, error) {
	ok, err := s.hasPermission(ctx, sub, "rbac.manage")
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}
	ok, err = s.hasPermission(ctx, sub, "system.settings")
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}
	caller, err := s.repo.GetMembershipByID(ctx, sub.MembershipID)
	if err != nil {
		return false, err
	}
	return caller.IsPrimaryAdmin, nil
}

func isForbiddenDelegationPermission(code string) bool {
	if code == "rbac.manage" || code == "system.settings" {
		return true
	}
	allowed := false
	for _, d := range DelegatableMembershipPermissions {
		if d == code {
			allowed = true
			break
		}
	}
	if !allowed {
		return true
	}
	if _, ok := criticalPermissionCodes[code]; ok {
		for _, d := range DelegatableMembershipPermissions {
			if d == code {
				return false
			}
		}
		return true
	}
	return false
}

func (s *adminService) departmentInCompany(ctx context.Context, companyID, departmentID string) bool {
	depts, err := s.repo.ListCompanyDepartments(ctx, companyID)
	if err != nil {
		return false
	}
	for _, d := range depts {
		if d.DepartmentID == departmentID {
			return true
		}
	}
	return false
}

func (s *adminService) appendDelegationAudit(ctx context.Context, sub AdminSubject, action string, row *DelegatedAdminGrant, extra map[string]any) {
	if s.auditRepo == nil || row == nil {
		return
	}
	meta := map[string]any{
		"delegation_id":             row.DelegationID,
		"scope_type":                row.ScopeType,
		"scope_id":                  row.ScopeID,
		"delegatee_membership_id":   row.DelegateeMembershipID,
		"delegator_membership_id":   row.DelegatorMembershipID,
		"permission_set":            row.PermissionSet,
		"status":                    row.Status,
	}
	for k, v := range extra {
		meta[k] = v
	}
	_ = s.auditRepo.Append(ctx, auditapp.Entry{
		ActorUserID:       sub.UserID,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            action,
		ResourceType:      "delegation",
		ResourceID:        row.DelegationID,
		Decision:          "allow",
		Metadata:          meta,
	})
}

// delegatorEffectivePermissions is used in tests.
func delegatorEffectivePermissions(eff *authapp.EffectiveAccessSummary) map[string]struct{} {
	out := make(map[string]struct{})
	if eff == nil {
		return out
	}
	for _, p := range eff.Permissions {
		out[p] = struct{}{}
	}
	return out
}
