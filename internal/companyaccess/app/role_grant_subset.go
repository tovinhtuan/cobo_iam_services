package app

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// grantLimit describes what a caller may hand out (ROLE-03). A caller without rbac.manage (for
// example a department head who was given admin.membership.invite) may only grant roles and
// direct permissions made of permissions it holds; without this rule the invite permission is
// enough to create a company administrator. Company admins and platform operators, who
// provision users for client companies through the platform CMS, are unrestricted.
type grantLimit struct {
	unrestricted bool
	held         map[string]struct{}
}

func (s *adminService) grantLimitFor(ctx context.Context, sub AdminSubject) (*grantLimit, error) {
	isAdmin, err := s.hasPermission(ctx, sub, "rbac.manage")
	if err != nil {
		return nil, err
	}
	if isAdmin {
		return &grantLimit{unrestricted: true}, nil
	}
	operator, err := s.isPlatformCompanyOperator(ctx, sub)
	if err != nil {
		return nil, err
	}
	if operator {
		return &grantLimit{unrestricted: true}, nil
	}
	eff, err := s.auth.GetEffectiveAccess(ctx, sub.MembershipID, sub.CompanyID)
	if err != nil {
		return nil, fmt.Errorf("load effective access: %w", err)
	}
	held := make(map[string]struct{}, len(eff.Permissions))
	for _, p := range eff.Permissions {
		held[p] = struct{}{}
	}
	return &grantLimit{held: held}, nil
}

// missingGrantPermissions returns, sorted, the permissions of the role and the direct codes
// the caller does not hold.
func (s *adminService) missingGrantPermissions(ctx context.Context, limit *grantLimit, companyID, roleID string, directCodes []string) ([]string, error) {
	if limit.unrestricted {
		return nil, nil
	}
	missing := map[string]struct{}{}
	if strings.TrimSpace(roleID) != "" {
		view, err := s.repo.ListRolePermissions(ctx, companyID, strings.TrimSpace(roleID))
		if err != nil {
			return nil, err
		}
		if view == nil {
			return nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "role not found", nil)
		}
		for _, p := range view.Permissions {
			if _, ok := limit.held[p.PermissionCode]; !ok {
				missing[p.PermissionCode] = struct{}{}
			}
		}
	}
	for _, code := range directCodes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if _, ok := limit.held[code]; !ok {
			missing[code] = struct{}{}
		}
	}
	codes := make([]string, 0, len(missing))
	for c := range missing {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	return codes, nil
}

// assertCanGrant enforces ROLE-03 for one role and a set of direct permissions.
func (s *adminService) assertCanGrant(ctx context.Context, sub AdminSubject, companyID, roleID string, directCodes []string) error {
	limit, err := s.grantLimitFor(ctx, sub)
	if err != nil {
		return err
	}
	codes, err := s.missingGrantPermissions(ctx, limit, companyID, roleID, directCodes)
	if err != nil {
		return err
	}
	if len(codes) == 0 {
		return nil
	}
	return &perr.HTTPError{
		Code:       perr.CodePermissionDenied,
		Message:    "you can only grant permissions you hold yourself",
		HTTPStatus: http.StatusForbidden,
		Details:    map[string]any{"permission_codes": codes},
	}
}

// filterGrantableInviteRoles keeps the invite roles the caller may grant, so the picker does
// not offer roles the invite would then refuse (API-17).
func (s *adminService) filterGrantableInviteRoles(ctx context.Context, sub AdminSubject, companyID string, items []InviteRoleOption) ([]InviteRoleOption, error) {
	limit, err := s.grantLimitFor(ctx, sub)
	if err != nil || limit.unrestricted {
		return items, err
	}
	out := make([]InviteRoleOption, 0, len(items))
	for _, item := range items {
		codes, err := s.missingGrantPermissions(ctx, limit, companyID, item.RoleID, nil)
		if err != nil {
			if he, ok := perr.AsHTTPError(err); ok && he.HTTPStatus == http.StatusNotFound {
				continue
			}
			return nil, err
		}
		if len(codes) == 0 {
			out = append(out, item)
		}
	}
	return out, nil
}
