package app

import (
	"context"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type ReplaceMembershipPrimaryRoleRequest struct {
	Subject      AdminSubject
	MembershipID string
	RoleID       string `json:"role_id"`
}

func partitionMembershipRoles(roles []RoleView) (primary *RoleView, legacy []RoleView, extra []RoleView) {
	nonLegacy := make([]RoleView, 0, len(roles))
	for _, r := range roles {
		if IsEnterpriseInviteRoleDenied(r.RoleCode) {
			legacy = append(legacy, r)
			continue
		}
		nonLegacy = append(nonLegacy, r)
	}
	if len(nonLegacy) > 0 {
		cp := nonLegacy[0]
		primary = &cp
		extra = nonLegacy[1:]
	}
	return primary, legacy, extra
}

func rejectEnterpriseRoleIDsPayload(roleIDs []string) error {
	if len(roleIDs) == 0 {
		return nil
	}
	return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "role_ids is not allowed for enterprise staff management; use role_id", nil)
}

func (s *adminService) ReplaceMembershipPrimaryRole(ctx context.Context, req ReplaceMembershipPrimaryRoleRequest) (err error) {
	wrote := false
	defer s.invalidateEffectiveAccessIfWritten(ctx, req.Subject.CompanyID, &err, &wrote)
	if err := s.authorize(ctx, req.Subject, "admin.membership.role.assign", req.MembershipID); err != nil {
		return err
	}
	if err := s.authorizeScopedMembershipMutation(ctx, req.Subject, "admin.membership.update", req.MembershipID); err != nil {
		return err
	}

	roleID := strings.TrimSpace(req.RoleID)
	if roleID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "role_id is required", nil)
	}

	// The membership belongs to the caller's company (checked above); roles are validated against
	// that same company.
	isPlatformCMS, err := s.isPlatformCompanyOperator(ctx, req.Subject)
	if err != nil {
		return err
	}
	if !isPlatformCMS {
		if _, err := s.validateEnterpriseInviteRole(ctx, req.Subject.CompanyID, roleID, "", "user_thuong", false); err != nil {
			return err
		}
	} else {
		// Platform CMS still must not assign inactive / cross-tenant roles.
		if _, err := s.assertRoleAssignableForMembership(ctx, req.Subject.CompanyID, roleID); err != nil {
			return err
		}
	}

	// ROLE-23: the last-admin checks below and the writes run under the company's admin lock.
	return s.withCompanyAdminLock(ctx, req.Subject.CompanyID, func() error {
		if err := s.assertPrimaryRoleChangeLockout(ctx, MembershipActor{
			MembershipID: req.Subject.MembershipID,
		}, req.MembershipID, req.Subject.CompanyID, roleID); err != nil {
			return err
		}

		roles, err := s.repo.ListMembershipRoles(ctx, req.MembershipID)
		if err != nil {
			return err
		}
		primary, _, _ := partitionMembershipRoles(roles)
		if primary != nil && primary.RoleID == roleID {
			return nil
		}

		alreadyAssigned := false
		for _, r := range roles {
			if r.RoleID == roleID {
				alreadyAssigned = true
				break
			}
		}

		if err := s.assertCanGrant(ctx, req.Subject, req.Subject.CompanyID, roleID, nil); err != nil {
			return err
		}
		if primary != nil {
			if err := s.assertMayRemovePlatformRole(ctx, req.Subject, primary.RoleID); err != nil {
				return err
			}
			if err := s.assertKeepsPlatformOperator(ctx, req.Subject, req.MembershipID, primary.RoleID, ""); err != nil {
				return err
			}
			// ROLE-05 / BES-19: replacing the primary role must not demote the primary admin.
			newIsAdmin, err := s.isRoleAdminCapable(ctx, req.Subject.CompanyID, roleID)
			if err != nil {
				return err
			}
			if !newIsAdmin {
				if err := s.assertRoleRemovalKeepsAdmin(ctx, req.Subject.CompanyID, req.MembershipID, primary.RoleID); err != nil {
					return err
				}
			}
		}
		wrote = true
		if primary != nil {
			if err := s.repo.RemoveRole(ctx, req.MembershipID, primary.RoleID); err != nil {
				return err
			}
		}
		if !alreadyAssigned {
			if err := s.repo.AddRole(ctx, req.MembershipID, roleID); err != nil {
				return err
			}
		}
		return nil
	})
}
