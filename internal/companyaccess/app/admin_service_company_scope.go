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
