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

func (s *adminService) ReplaceMembershipPrimaryRole(ctx context.Context, req ReplaceMembershipPrimaryRoleRequest) error {
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

	member, err := s.repo.GetMembershipByID(ctx, req.MembershipID)
	if err != nil {
		return err
	}

	isPlatformCMS, err := s.isPlatformCMSOperator(ctx, req.Subject)
	if err != nil {
		return err
	}
	if !isPlatformCMS {
		if _, err := s.validateEnterpriseInviteRole(ctx, member.CompanyID, roleID, "", "user_thuong", false); err != nil {
			return err
		}
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
}
