package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// Company scope for tenant admin APIs (risk review 2026-10-09, C4/H2).
//
// Acting on a company other than the one in the access token, or on users that belong to no
// company, is a platform capability. It is decided by isPlatformCompanyOperator and never by a
// tenant permission such as rbac.manage (every self-registered company owner holds it).

// errCompanyScopeMismatch is returned when a non-operator targets another company.
func errCompanyScopeMismatch() error {
	return perr.NewHTTPError(http.StatusForbidden, perr.CodeCompanyScopeMismatch,
		"company_id must match the current company", nil)
}

// isPlatformCompanyOperator reports whether the subject may act across companies:
// platform.cms.view AND (rbac.manage OR system.settings). This mirrors the gate of the
// /api/v1/platform/cms/admin/* routes.
//
// Only the subject's real effective permissions are considered. Break-glass overlays are
// deliberately ignored: they grant tenant-scoped, time-boxed capabilities and can never make a
// tenant member a platform operator.
func (s *adminService) isPlatformCompanyOperator(ctx context.Context, sub AdminSubject) (bool, error) {
	eff, err := s.auth.GetEffectiveAccess(ctx, sub.MembershipID, sub.CompanyID)
	if err != nil {
		return false, fmt.Errorf("load effective access: %w", err)
	}
	var cmsView, rbacManage, systemSettings bool
	for _, p := range eff.Permissions {
		switch strings.TrimSpace(p) {
		case "platform.cms.view":
			cmsView = true
		case "rbac.manage":
			rbacManage = true
		case "system.settings":
			systemSettings = true
		}
	}
	return cmsView && (rbacManage || systemSettings), nil
}

// resolveTargetCompany returns the company a tenant admin operation must act on.
//
//   - Platform operator: requested is used as given. An empty value means "no company" and is only
//     honoured when allowNoCompany is true; otherwise the subject's own company is used.
//   - Everyone else: requested must be empty or equal to the subject's company (403
//     COMPANY_SCOPE_MISMATCH otherwise). The result is always the subject's company; a subject
//     without a company is rejected (422 COMPANY_CONTEXT_REQUIRED).
func (s *adminService) resolveTargetCompany(ctx context.Context, sub AdminSubject, requested string, allowNoCompany bool) (string, error) {
	requested = strings.TrimSpace(requested)
	operator, err := s.isPlatformCompanyOperator(ctx, sub)
	if err != nil {
		return "", err
	}
	if operator {
		if requested == "" && !allowNoCompany {
			return sub.CompanyID, nil
		}
		return requested, nil
	}
	// Fail closed: a non-operator without a company in the token has no company to act on.
	if strings.TrimSpace(sub.CompanyID) == "" {
		return "", perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeCompanyContextRequired,
			"company context is required", nil)
	}
	if requested != "" && requested != sub.CompanyID {
		return "", errCompanyScopeMismatch()
	}
	return sub.CompanyID, nil
}

// assertRoleHasNoPlatformPermissions rejects a role that carries platform-tier permissions
// (platform.cms.view, cms.*, ... see IsEnterprisePermission). Handing such a role to a membership
// would make it a platform operator, so only platform operators may assign it.
func (s *adminService) assertRoleHasNoPlatformPermissions(ctx context.Context, companyID, roleID string) error {
	view, err := s.repo.ListRolePermissions(ctx, companyID, strings.TrimSpace(roleID))
	if err != nil {
		return err
	}
	if view == nil {
		return nil
	}
	for _, p := range view.Permissions {
		if !IsEnterprisePermission(strings.TrimSpace(p.PermissionCode), p.ModuleName) {
			return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied,
				"role carries platform permissions and can only be assigned by a platform operator", nil)
		}
	}
	return nil
}

// assertMayRemovePlatformRole (ROLE-13): a tenant admin cannot assign a role carrying
// platform-tier permissions, so it may not remove one either; only a platform operator may.
func (s *adminService) assertMayRemovePlatformRole(ctx context.Context, sub AdminSubject, roleID string) error {
	operator, err := s.isPlatformCompanyOperator(ctx, sub)
	if err != nil || operator {
		return err
	}
	// Judged by permission code (platform.*, cms.*, EnterpriseDenyCodes), not by catalog module:
	// tenant permissions such as disclosure_type.manage are tagged module "cms", and a company
	// admin must still be able to remove a role like admin_doanh_nghiep that carries them.
	view, err := s.repo.ListRolePermissions(ctx, sub.CompanyID, strings.TrimSpace(roleID))
	if err != nil {
		if he, ok := perr.AsHTTPError(err); ok && he.HTTPStatus == http.StatusNotFound {
			return nil // the removal itself reports a missing role
		}
		return err
	}
	if view == nil {
		return nil
	}
	for _, p := range view.Permissions {
		if isPlatformTierPermission(p.PermissionCode) {
			return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied,
				"role carries platform permissions and can only be removed by a platform operator", nil)
		}
	}
	return nil
}

// assertMayRemovePlatformPermission is the direct-permission counterpart of
// assertMayRemovePlatformRole.
func (s *adminService) assertMayRemovePlatformPermission(ctx context.Context, sub AdminSubject, code string) error {
	if !isPlatformTierPermission(code) {
		return nil
	}
	operator, err := s.isPlatformCompanyOperator(ctx, sub)
	if err != nil || operator {
		return err
	}
	return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied,
		"platform permissions can only be removed by a platform operator", nil)
}

// assertKeepsPlatformOperator (ROLE-20): a caller who is not a platform operator may not make a
// member that is one stop being one, whatever is removed (a role without platform permissions
// can still be the one that gives the operator its rbac.manage / system.settings).
func (s *adminService) assertKeepsPlatformOperator(ctx context.Context, sub AdminSubject, membershipID, removedRoleID, removedDirectCode string) error {
	operator, err := s.isPlatformCompanyOperator(ctx, sub)
	if err != nil || operator {
		return err
	}
	before, err := s.membershipPermissionSet(ctx, sub.CompanyID, membershipID, "", "")
	if err != nil || !isOperatorPermissionSet(before) {
		return err
	}
	after, err := s.membershipPermissionSet(ctx, sub.CompanyID, membershipID, removedRoleID, removedDirectCode)
	if err != nil {
		return err
	}
	if isOperatorPermissionSet(after) {
		return nil
	}
	return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied,
		"the change would end the member's platform operator access; only a platform operator may do that", nil)
}

// membershipPermissionSet collects a membership's role and direct permissions, leaving out one
// role and/or one direct permission code.
func (s *adminService) membershipPermissionSet(ctx context.Context, companyID, membershipID, skipRoleID, skipDirectCode string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	roles, err := s.repo.ListMembershipRoles(ctx, membershipID)
	if err != nil {
		return nil, err
	}
	for _, r := range roles {
		if r.RoleID == skipRoleID {
			continue
		}
		view, err := s.repo.ListRolePermissions(ctx, companyID, r.RoleID)
		if err != nil {
			if he, ok := perr.AsHTTPError(err); ok && he.HTTPStatus == http.StatusNotFound {
				continue
			}
			return nil, err
		}
		if view == nil {
			continue
		}
		for _, p := range view.Permissions {
			out[p.PermissionCode] = struct{}{}
		}
	}
	direct, err := s.repo.ListActiveDirectPermissions(ctx, membershipID)
	if err != nil {
		return nil, err
	}
	for _, d := range direct {
		if d.PermissionCode != skipDirectCode {
			out[d.PermissionCode] = struct{}{}
		}
	}
	return out, nil
}

func isOperatorPermissionSet(perms map[string]struct{}) bool {
	_, cms := perms["platform.cms.view"]
	_, rbac := perms["rbac.manage"]
	_, settings := perms["system.settings"]
	return cms && (rbac || settings)
}

// normalizeNewMembershipStatus (ROLE-09) accepts the statuses a new membership may start in;
// the empty value means active.
func normalizeNewMembershipStatus(status string) (string, error) {
	switch s := strings.ToLower(strings.TrimSpace(status)); s {
	case "":
		return "active", nil
	case "active", "inactive", "invited":
		return s, nil
	default:
		return "", perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "status must be active, inactive or invited", nil)
	}
}

// isPlatformTierPermission reports a permission outside the enterprise (tenant) scope:
// EnterpriseDenyCodes plus anything under platform.* or cms.*.
func isPlatformTierPermission(code string) bool {
	code = strings.TrimSpace(code)
	if strings.HasPrefix(code, "platform.") || strings.HasPrefix(code, "cms.") {
		return true
	}
	return !IsEnterprisePermission(code, "")
}

// assertMayDeactivatePlatformMember (ROLE-13 / BES-21): deactivating or deleting a membership
// that holds platform-tier permissions removes that platform access as well, so only a
// platform operator may do it.
func (s *adminService) assertMayDeactivatePlatformMember(ctx context.Context, sub AdminSubject, membershipID string) error {
	operator, err := s.isPlatformCompanyOperator(ctx, sub)
	if err != nil || operator {
		return err
	}
	eff, err := s.auth.GetEffectiveAccess(ctx, membershipID, sub.CompanyID)
	if err != nil {
		return fmt.Errorf("load effective access: %w", err)
	}
	for _, p := range eff.Permissions {
		if isPlatformTierPermission(p) {
			return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied,
				"membership holds platform permissions and can only be deactivated by a platform operator", nil)
		}
	}
	return nil
}

// requireTargetMembership makes sure the membership a tenant route acts on belongs to the
// company of the access token. A membership of another company answers exactly like a missing
// one (404 MEMBERSHIP_NOT_FOUND) so ids of other companies cannot be probed. Tenant routes never
// act across companies, platform operators included; cross-company work goes through
// /api/v1/platform/cms/*.
func (s *adminService) requireTargetMembership(ctx context.Context, sub AdminSubject, membershipID string) error {
	if strings.TrimSpace(sub.CompanyID) == "" {
		return perr.NewHTTPError(http.StatusUnprocessableEntity, perr.CodeCompanyContextRequired,
			"company context is required", nil)
	}
	return s.requireMembershipInCompany(ctx, membershipID, sub.CompanyID)
}

// requireDepartmentInCompany makes sure a department belongs to the caller's company (404
// otherwise, the same answer as for a missing department). Storage errors are propagated.
func (s *adminService) requireDepartmentInCompany(ctx context.Context, companyID, departmentID string) error {
	ok, err := s.repo.DepartmentBelongsToCompany(ctx, companyID, strings.TrimSpace(departmentID))
	if err != nil {
		return err
	}
	if !ok {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "department not found", nil)
	}
	return nil
}

// requireTitleInCompany makes sure a title belongs to the caller's company (404 otherwise).
func (s *adminService) requireTitleInCompany(ctx context.Context, companyID, titleID string) error {
	ok, err := s.repo.TitleBelongsToCompany(ctx, companyID, strings.TrimSpace(titleID))
	if err != nil {
		return err
	}
	if !ok {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "title not found", nil)
	}
	return nil
}

// requireTeamInCompany makes sure a team belongs to the caller's company (404 otherwise, the same
// answer as for a missing team).
func (s *adminService) requireTeamInCompany(ctx context.Context, companyID, teamID string) error {
	ok, err := s.repo.TeamBelongsToCompany(ctx, companyID, strings.TrimSpace(teamID))
	if err != nil {
		return err
	}
	if !ok {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "team not found", nil)
	}
	return nil
}
