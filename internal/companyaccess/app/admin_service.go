package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	"github.com/cobo/cobo_iam_services/internal/platform/refreshtoken"
	"golang.org/x/crypto/bcrypt"
)

type adminService struct {
	repo                  AdminRepository
	auth                  authapp.Service
	idg                   idgen.Generator
	invMailer             InvitationMailer
	inviteTTL             time.Duration
	inviteDefaultRoleCode string
	tierLookup            func(ctx context.Context, userID string) string
}

func NewAdminService(repo AdminRepository, auth authapp.Service, idg idgen.Generator, opts ...AdminOption) AdminService {
	s := &adminService{repo: repo, auth: auth, idg: idg, inviteTTL: 72 * time.Hour, inviteDefaultRoleCode: "user_thuong"}
	for _, o := range opts {
		if o != nil {
			o(s)
		}
	}
	return s
}

func inviteRawTokenAndHash() (raw string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, refreshtoken.Hash(raw), nil
}

func (s *adminService) CreateUser(ctx context.Context, req CreateUserRequest) (*UserView, error) {
	// Keep auth compatible with existing policy map in week-1 bootstrap.
	if err := s.authorize(ctx, req.Subject, "admin.membership.create", req.Subject.CompanyID); err != nil {
		return nil, err
	}
	isWebAdmin, err := s.hasPermission(ctx, req.Subject, "rbac.manage")
	if err != nil {
		return nil, err
	}

	req.LoginID = strings.ToLower(strings.TrimSpace(req.LoginID))
	req.FullName = strings.TrimSpace(req.FullName)
	req.Email = strings.TrimSpace(req.Email)
	req.Phone = strings.TrimSpace(req.Phone)
	req.AccountStatus = strings.TrimSpace(req.AccountStatus)
	req.CompanyID = strings.TrimSpace(req.CompanyID)
	req.MembershipStatus = strings.TrimSpace(req.MembershipStatus)
	req.RoleID = strings.TrimSpace(req.RoleID)
	req.RoleCode = strings.TrimSpace(req.RoleCode)
	req.DepartmentID = strings.TrimSpace(req.DepartmentID)
	req.TitleID = strings.TrimSpace(req.TitleID)
	if req.AccountStatus == "" {
		req.AccountStatus = "active"
	}
	// Enterprise admin can only add users into current company.
	if !isWebAdmin {
		if req.CompanyID != "" && req.CompanyID != req.Subject.CompanyID {
			return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "enterprise admin can only create users for current company", nil)
		}
		req.CompanyID = req.Subject.CompanyID
	}
	if req.CompanyID != "" && req.MembershipStatus == "" {
		req.MembershipStatus = "active"
	}
	if req.CompanyID != "" {
		scope, err := s.resolveInviteScope(ctx, req.Subject)
		if err != nil {
			return nil, err
		}
		deptID, err := s.pickInviteDepartmentID(scope, req.DepartmentID)
		if err != nil {
			return nil, err
		}
		req.DepartmentID = deptID
		for _, p := range req.Permissions {
			if !isGrantable(p) {
				return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "permission_code is not grantable: "+p, nil)
			}
		}
	}
	if req.LoginID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "login_id required", nil)
	}
	if req.FullName == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "full_name required", nil)
	}
	if len(req.Password) < 12 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "password must be at least 12 characters", nil)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	u := UserView{
		UserID:        s.idg.NewUUID(),
		LoginID:       req.LoginID,
		FullName:      req.FullName,
		Email:         req.Email,
		Phone:         req.Phone,
		AccountStatus: req.AccountStatus,
	}
	opts := CreateUserOptions{
		CompanyID:        req.CompanyID,
		MembershipStatus: req.MembershipStatus,
	}
	if req.CompanyID != "" {
		defRoleCode := strings.TrimSpace(s.inviteDefaultRoleCode)
		if defRoleCode == "" {
			defRoleCode = "user_thuong"
		}
		roleID, err := s.repo.LookupRoleIDForInvite(ctx, req.CompanyID, req.RoleID, req.RoleCode, defRoleCode)
		if err != nil {
			return nil, err
		}
		opts.MembershipID = s.idg.NewUUID()
		opts.InitialRoleID = roleID
	}
	out, err := s.repo.CreateUser(ctx, u, string(hash), opts)
	if err != nil {
		return nil, err
	}
	if out.MembershipID != "" {
		for _, p := range req.Permissions {
			if err := s.repo.InsertDirectPermission(ctx, out.MembershipID, out.CompanyID, p, req.Subject.UserID); err != nil {
				return nil, err
			}
		}
		if err := s.grantDefaultPermissions(ctx, out.MembershipID, out.CompanyID, req.Subject.UserID); err != nil {
			return nil, err
		}
		if err := s.assignInviteDepartment(ctx, out.MembershipID, req.DepartmentID); err != nil {
			return nil, err
		}
		if err := s.assignInviteTitle(ctx, out.MembershipID, req.TitleID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *adminService) CreateCompany(ctx context.Context, req CreateCompanyRequest) (*CreateCompanyResult, error) {
	if err := s.authorizePlatformCompanyAdmin(ctx, req.Subject); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.CompanyName)
	if name == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_name is required", nil)
	}
	boot := CreateCompanyBootstrap{
		VerificationStatus: "verified",
		TaxCode:            strings.TrimSpace(req.TaxCode),
		RegistrationNumber: strings.TrimSpace(req.RegistrationNumber),
		Address:            strings.TrimSpace(req.Address),
		Phone:              strings.TrimSpace(req.Phone),
		ContactEmail:       strings.TrimSpace(req.ContactEmail),
		RepresentativeName: strings.TrimSpace(req.RepresentativeName),
	}
	companyID, companyCode, err := s.repo.CreateStandaloneCompany(ctx, name, boot)
	if err != nil {
		return nil, err
	}
	return &CreateCompanyResult{
		CompanyID:   companyID,
		CompanyCode: companyCode,
		CompanyName: name,
	}, nil
}

func (s *adminService) authorizePlatformCompanyAdmin(ctx context.Context, sub AdminSubject) error {
	if err := s.authorize(ctx, sub, "admin.membership.create", sub.CompanyID); err != nil {
		return err
	}
	canRbac, err := s.hasPermission(ctx, sub, "rbac.manage")
	if err != nil {
		return err
	}
	canSettings, err := s.hasPermission(ctx, sub, "system.settings")
	if err != nil {
		return err
	}
	if !canRbac && !canSettings {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "only platform administrators can manage companies", nil)
	}
	return nil
}

func (s *adminService) ListPlatformCompanies(ctx context.Context, req ListPlatformCompaniesRequest) (*ListPlatformCompaniesResult, error) {
	if err := s.authorizePlatformCompanyAdmin(ctx, req.Subject); err != nil {
		return nil, err
	}
	return s.repo.ListCompaniesPlatform(ctx, req)
}

func (s *adminService) GetPlatformCompany(ctx context.Context, req GetPlatformCompanyRequest) (*PlatformCompanyDetail, error) {
	if err := s.authorizePlatformCompanyAdmin(ctx, req.Subject); err != nil {
		return nil, err
	}
	return s.repo.GetCompanyPlatform(ctx, strings.TrimSpace(req.CompanyID))
}

func (s *adminService) UpdatePlatformCompany(ctx context.Context, req UpdatePlatformCompanyRequest) error {
	if err := s.authorizePlatformCompanyAdmin(ctx, req.Subject); err != nil {
		return err
	}
	return s.repo.UpdateCompanyPlatform(ctx, req)
}

func (s *adminService) SetPlatformCompanyStatus(ctx context.Context, req SetPlatformCompanyStatusRequest) error {
	if err := s.authorizePlatformCompanyAdmin(ctx, req.Subject); err != nil {
		return err
	}
	return s.repo.SetCompanyStatusPlatform(ctx, strings.TrimSpace(req.CompanyID), strings.TrimSpace(req.Status))
}

func (s *adminService) InviteUser(ctx context.Context, req InviteUserRequest) (*InviteUserResponse, error) {
	if err := s.authorizeMembershipInvite(ctx, req.Subject, req.Subject.CompanyID); err != nil {
		return nil, err
	}
	scope, err := s.resolveInviteScope(ctx, req.Subject)
	if err != nil {
		return nil, err
	}
	deptID, err := s.pickInviteDepartmentID(scope, req.DepartmentID)
	if err != nil {
		return nil, err
	}
	req.DepartmentID = deptID
	isWebAdmin, err := s.hasPermission(ctx, req.Subject, "rbac.manage")
	if err != nil {
		return nil, err
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.FullName = strings.TrimSpace(req.FullName)
	req.CompanyID = strings.TrimSpace(req.CompanyID)
	req.MembershipStatus = strings.TrimSpace(req.MembershipStatus)
	req.TitleID = strings.TrimSpace(req.TitleID)
	if req.Email == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "email is required", nil)
	}
	if req.FullName == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "full_name is required", nil)
	}
	if !isWebAdmin {
		if req.CompanyID != "" && req.CompanyID != req.Subject.CompanyID {
			return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "enterprise admin can only create users for current company", nil)
		}
		req.CompanyID = req.Subject.CompanyID
	}
	// Platform web admin (rbac.manage) may invite without a company; enterprise admin is always
	// scoped to their own company (forced above), so empty CompanyID is only possible for web admin.
	if req.CompanyID == "" {
		return s.inviteUserWithoutCompany(ctx, req)
	}
	return s.inviteUserWithCompany(ctx, req, scope)
}

// inviteUserWithoutCompany creates a user + invitation with no membership.
// Only reachable for platform web admins (rbac.manage) when CompanyID is omitted.
func (s *adminService) inviteUserWithoutCompany(ctx context.Context, req InviteUserRequest) (*InviteUserResponse, error) {
	existingUserID, existingStatus, found, err := s.repo.LookupUserByLoginID(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if found {
		status := strings.ToLower(strings.TrimSpace(existingStatus))
		if status == "active" {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "user already exists and is active", nil)
		}
		if status == "invited" {
			// Resend invitation without changing company assignment
			rawTok, tokHash, err := inviteRawTokenAndHash()
			if err != nil {
				return nil, fmt.Errorf("generate invitation token: %w", err)
			}
			expiresAt := time.Now().UTC().Add(s.inviteTTL)
			if err := s.repo.ReplaceUserInvitation(ctx, existingUserID, s.idg.NewUUID(), tokHash, strings.TrimSpace(req.CreatedByUserID), expiresAt); err != nil {
				return nil, err
			}
			loginID, emailAddr, fullName, _, err := s.repo.GetUserProfile(ctx, existingUserID)
			if err != nil {
				return nil, err
			}
			if s.invMailer != nil {
				to := strings.TrimSpace(emailAddr)
				if to == "" {
					to = loginID
				}
				if err := s.invMailer.SendInvitationEmail(ctx, InvitationMailPayload{
					UserID: existingUserID, ToEmail: to, FullName: fullName, LoginID: loginID, RawToken: rawTok,
				}); err != nil {
					return nil, err
				}
			}
			return &InviteUserResponse{
				UserID: existingUserID, LoginID: loginID, Email: emailAddr, FullName: fullName,
				AccountStatus: "invited", InvitationExpiresAt: expiresAt.UTC().Format(time.RFC3339),
			}, nil
		}
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "user exists with unknown status", nil)
	}

	rawTok, tokHash, err := inviteRawTokenAndHash()
	if err != nil {
		return nil, fmt.Errorf("generate invitation token: %w", err)
	}
	expiresAt := time.Now().UTC().Add(s.inviteTTL)
	userID := s.idg.NewUUID()
	u := UserView{UserID: userID, LoginID: req.Email, FullName: req.FullName, Email: req.Email, AccountStatus: "invited"}
	createdBy := strings.TrimSpace(req.CreatedByUserID)

	out, err := s.repo.InviteUserWithoutCompany(ctx, u, s.idg.NewUUID(), tokHash, createdBy, expiresAt)
	if err != nil {
		return nil, err
	}
	if s.invMailer != nil {
		to := strings.TrimSpace(out.Email)
		if to == "" {
			to = out.LoginID
		}
		if err := s.invMailer.SendInvitationEmail(ctx, InvitationMailPayload{
			UserID: out.UserID, ToEmail: to, FullName: out.FullName, LoginID: out.LoginID, RawToken: rawTok,
		}); err != nil {
			return nil, err
		}
	}
	return &InviteUserResponse{
		UserID: out.UserID, LoginID: out.LoginID, Email: out.Email, FullName: out.FullName,
		AccountStatus: out.AccountStatus, InvitationExpiresAt: expiresAt.UTC().Format(time.RFC3339),
	}, nil
}

// inviteUserWithCompany is the original invite flow (with company + membership creation).
func (s *adminService) inviteUserWithCompany(ctx context.Context, req InviteUserRequest, scope inviteScope) (*InviteUserResponse, error) {
	_ = scope
	if req.MembershipStatus == "" {
		req.MembershipStatus = "active"
	}
	for _, p := range req.Permissions {
		if !isGrantable(p) {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "permission_code is not grantable: "+p, nil)
		}
	}

	existingUserID, existingStatus, found, err := s.repo.LookupUserByLoginID(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if found && strings.EqualFold(strings.TrimSpace(existingStatus), "active") {
		already, err := s.repo.MembershipExistsForUserCompany(ctx, existingUserID, req.CompanyID)
		if err != nil {
			return nil, err
		}
		if already {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "user is already a member of this company", nil)
		}
		defRoleCode := strings.TrimSpace(s.inviteDefaultRoleCode)
		if defRoleCode == "" {
			defRoleCode = "user_thuong"
		}
		roleID, err := s.repo.LookupRoleIDForInvite(ctx, req.CompanyID, strings.TrimSpace(req.RoleID), strings.TrimSpace(req.RoleCode), defRoleCode)
		if err != nil {
			return nil, err
		}
		membershipID := s.idg.NewUUID()
		m, err := s.repo.CreateMembership(ctx, MembershipView{
			MembershipID: membershipID,
			UserID:       existingUserID,
			CompanyID:    req.CompanyID,
			Status:       req.MembershipStatus,
		})
		if err != nil {
			return nil, err
		}
		if err := s.repo.AddRole(ctx, m.MembershipID, roleID); err != nil {
			_ = s.repo.DeleteMembership(ctx, m.MembershipID)
			return nil, err
		}
		for _, p := range req.Permissions {
			if err := s.repo.InsertDirectPermission(ctx, m.MembershipID, req.CompanyID, p, req.CreatedByUserID); err != nil {
				return nil, err
			}
		}
		if err := s.grantDefaultPermissions(ctx, m.MembershipID, req.CompanyID, req.CreatedByUserID); err != nil {
			return nil, err
		}
		if err := s.assignInviteDepartment(ctx, m.MembershipID, req.DepartmentID); err != nil {
			return nil, err
		}
		if err := s.assignInviteTitle(ctx, m.MembershipID, req.TitleID); err != nil {
			return nil, err
		}
		companyDisplay := ""
		if cn, err := s.repo.GetCompanyName(ctx, req.CompanyID); err == nil {
			companyDisplay = cn
		}
		loginID, emailAddr, fullName, _, err := s.repo.GetUserProfile(ctx, existingUserID)
		if err != nil {
			return nil, err
		}
		if s.invMailer != nil {
			to := strings.TrimSpace(emailAddr)
			if to == "" {
				to = strings.TrimSpace(loginID)
			}
			if err := s.invMailer.SendInvitationEmail(ctx, InvitationMailPayload{
				UserID: existingUserID, ToEmail: to, FullName: fullName, LoginID: loginID,
				RawToken: "", CompanyName: companyDisplay,
			}); err != nil {
				return nil, err
			}
		}
		return &InviteUserResponse{
			UserID:        existingUserID,
			LoginID:       loginID,
			Email:         emailAddr,
			FullName:      fullName,
			AccountStatus: "active",
			MembershipID:  m.MembershipID,
			CompanyID:     req.CompanyID,
		}, nil
	}
	if found {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "user exists; use resend invitation if account is invited", nil)
	}

	rawTok, tokHash, err := inviteRawTokenAndHash()
	if err != nil {
		return nil, fmt.Errorf("generate invitation token: %w", err)
	}
	expiresAt := time.Now().UTC().Add(s.inviteTTL)
	userID := s.idg.NewUUID()
	defRoleCode := strings.TrimSpace(s.inviteDefaultRoleCode)
	if defRoleCode == "" {
		defRoleCode = "user_thuong"
	}
	roleID, err := s.repo.LookupRoleIDForInvite(ctx, req.CompanyID, strings.TrimSpace(req.RoleID), strings.TrimSpace(req.RoleCode), defRoleCode)
	if err != nil {
		return nil, err
	}

	opts := CreateUserOptions{
		CompanyID:        req.CompanyID,
		MembershipStatus: req.MembershipStatus,
		MembershipID:     s.idg.NewUUID(),
		InitialRoleID:    roleID,
	}
	invitationID := s.idg.NewUUID()
	u := UserView{
		UserID:        userID,
		LoginID:       req.Email,
		FullName:      req.FullName,
		Email:         req.Email,
		AccountStatus: "invited",
	}
	createdBy := strings.TrimSpace(req.CreatedByUserID)

	out, err := s.repo.InviteUserWithMembership(ctx, u, opts, invitationID, tokHash, createdBy, expiresAt)
	if err != nil {
		return nil, err
	}
	for _, p := range req.Permissions {
		if err := s.repo.InsertDirectPermission(ctx, opts.MembershipID, req.CompanyID, p, createdBy); err != nil {
			return nil, err
		}
	}
	if err := s.grantDefaultPermissions(ctx, opts.MembershipID, req.CompanyID, createdBy); err != nil {
		return nil, err
	}
	if err := s.assignInviteDepartment(ctx, opts.MembershipID, req.DepartmentID); err != nil {
		return nil, err
	}
	if err := s.assignInviteTitle(ctx, opts.MembershipID, req.TitleID); err != nil {
		return nil, err
	}
	if s.invMailer != nil {
		if err := s.invMailer.SendInvitationEmail(ctx, InvitationMailPayload{
			UserID: out.UserID, ToEmail: out.Email, FullName: out.FullName, LoginID: out.LoginID, RawToken: rawTok,
			CompanyName: out.CompanyName,
		}); err != nil {
			return nil, err
		}
	}

	return &InviteUserResponse{
		UserID:              out.UserID,
		LoginID:             out.LoginID,
		Email:               out.Email,
		FullName:            out.FullName,
		AccountStatus:       out.AccountStatus,
		MembershipID:        out.MembershipID,
		CompanyID:           out.CompanyID,
		InvitationExpiresAt: expiresAt.UTC().Format(time.RFC3339),
	}, nil
}

func (s *adminService) ListInviteRoles(ctx context.Context, req ListInviteRolesRequest) ([]InviteRoleOption, error) {
	if err := s.authorizeMembershipInvite(ctx, req.Subject, req.Subject.CompanyID); err != nil {
		return nil, err
	}
	isWebAdmin, err := s.hasPermission(ctx, req.Subject, "rbac.manage")
	if err != nil {
		return nil, err
	}
	target := strings.TrimSpace(req.CompanyID)
	if !isWebAdmin {
		if target != "" && target != req.Subject.CompanyID {
			return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "enterprise admin can only list invite roles for current company", nil)
		}
		target = req.Subject.CompanyID
	}
	if target == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_id is required", nil)
	}
	return s.repo.ListInviteRolesForCompany(ctx, target)
}

func (s *adminService) ResendUserInvitation(ctx context.Context, req ResendUserInvitationRequest) error {
	if err := s.authorizeMembershipInvite(ctx, req.Subject, req.Subject.CompanyID); err != nil {
		return err
	}
	scope, err := s.resolveInviteScope(ctx, req.Subject)
	if err != nil {
		return err
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "user_id is required", nil)
	}
	if req.ResendNoCompanyScope {
		isWebAdmin, err := s.hasPermission(ctx, req.Subject, "rbac.manage")
		if err != nil {
			return err
		}
		if !isWebAdmin {
			return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "resend without company requires rbac.manage", nil)
		}
		n, err := s.repo.CountMembershipsForUser(ctx, userID)
		if err != nil {
			return err
		}
		if n > 0 {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "user has memberships; pass company_id to resend in company context", nil)
		}
		loginID, email, fullName, acctStatus, err := s.repo.GetUserProfile(ctx, userID)
		if err != nil {
			return err
		}
		if !strings.EqualFold(strings.TrimSpace(acctStatus), "invited") {
			return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "user is not in invited state", nil)
		}
		rawTok, tokHash, err := inviteRawTokenAndHash()
		if err != nil {
			return fmt.Errorf("generate invitation token: %w", err)
		}
		expiresAt := time.Now().UTC().Add(s.inviteTTL)
		if err := s.repo.ReplaceUserInvitation(ctx, userID, s.idg.NewUUID(), tokHash, strings.TrimSpace(req.Subject.UserID), expiresAt); err != nil {
			return err
		}
		if s.invMailer != nil {
			to := strings.TrimSpace(email)
			if to == "" {
				to = strings.TrimSpace(loginID)
			}
			return s.invMailer.SendInvitationEmail(ctx, InvitationMailPayload{
				UserID: userID, ToEmail: to, FullName: fullName, LoginID: loginID, RawToken: rawTok,
			})
		}
		return nil
	}

	cid := strings.TrimSpace(req.CompanyID)
	if cid == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "user_id and company_id are required", nil)
	}
	ok, err := s.repo.MembershipExistsForUserCompany(ctx, userID, cid)
	if err != nil {
		return err
	}
	if !ok {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeMembershipNotFound, "membership not found for user in company", nil)
	}
	if err := s.assertResendInInviteScope(ctx, scope, userID, cid); err != nil {
		return err
	}
	loginID, email, fullName, acctStatus, err := s.repo.GetUserProfile(ctx, userID)
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(acctStatus), "invited") {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "user is not in invited state", nil)
	}
	rawTok, tokHash, err := inviteRawTokenAndHash()
	if err != nil {
		return fmt.Errorf("generate invitation token: %w", err)
	}
	expiresAt := time.Now().UTC().Add(s.inviteTTL)
	if err := s.repo.ReplaceUserInvitation(ctx, userID, s.idg.NewUUID(), tokHash, strings.TrimSpace(req.Subject.UserID), expiresAt); err != nil {
		return err
	}
	if s.invMailer != nil {
		to := strings.TrimSpace(email)
		if to == "" {
			to = strings.TrimSpace(loginID)
		}
		companyDisplay := ""
		if cn, err := s.repo.GetCompanyName(ctx, cid); err == nil {
			companyDisplay = cn
		}
		return s.invMailer.SendInvitationEmail(ctx, InvitationMailPayload{
			UserID: userID, ToEmail: to, FullName: fullName, LoginID: loginID, RawToken: rawTok,
			CompanyName: companyDisplay,
		})
	}
	return nil
}

func (s *adminService) hasPermission(ctx context.Context, sub AdminSubject, permission string) (bool, error) {
	eff, err := s.auth.GetEffectiveAccess(ctx, sub.MembershipID, sub.CompanyID)
	if err != nil {
		return false, fmt.Errorf("load effective access: %w", err)
	}
	for _, p := range eff.Permissions {
		if p == permission {
			return true, nil
		}
	}
	return false, nil
}

// AssignUserToCompany links an existing user to a company. If the user is still in "invited" state
// it also creates membership + re-issues the invitation email with company context.
// If the user is already active it creates the membership directly.
func (s *adminService) AssignUserToCompany(ctx context.Context, req AssignUserToCompanyRequest) (*AssignUserToCompanyResponse, error) {
	if err := s.authorize(ctx, req.Subject, "admin.membership.create", req.CompanyID); err != nil {
		return nil, err
	}
	userID := strings.TrimSpace(req.UserID)
	companyID := strings.TrimSpace(req.CompanyID)
	if userID == "" || companyID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "user_id and company_id are required", nil)
	}

	loginID, emailAddr, fullName, acctStatus, err := s.repo.GetUserProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	already, err := s.repo.MembershipExistsForUserCompany(ctx, userID, companyID)
	if err != nil {
		return nil, err
	}
	if already {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "user is already a member of this company", nil)
	}

	membershipStatus := strings.TrimSpace(req.MembershipStatus)
	if membershipStatus == "" {
		membershipStatus = "active"
	}
	membershipID := s.idg.NewUUID()
	m, err := s.repo.CreateMembership(ctx, MembershipView{
		MembershipID: membershipID, UserID: userID, CompanyID: companyID, Status: membershipStatus,
	})
	if err != nil {
		return nil, err
	}

	// Assign role if requested.
	if roleID := strings.TrimSpace(req.RoleID); roleID != "" {
		if err := s.repo.AddRole(ctx, m.MembershipID, roleID); err != nil {
			_ = s.repo.DeleteMembership(ctx, m.MembershipID)
			return nil, err
		}
	} else if roleCode := strings.TrimSpace(req.RoleCode); roleCode != "" {
		defRoleCode := strings.TrimSpace(s.inviteDefaultRoleCode)
		if defRoleCode == "" {
			defRoleCode = "user_thuong"
		}
		resolvedRoleID, err := s.repo.LookupRoleIDForInvite(ctx, companyID, "", roleCode, defRoleCode)
		if err != nil {
			_ = s.repo.DeleteMembership(ctx, m.MembershipID)
			return nil, err
		}
		if err := s.repo.AddRole(ctx, m.MembershipID, resolvedRoleID); err != nil {
			_ = s.repo.DeleteMembership(ctx, m.MembershipID)
			return nil, err
		}
	}

	createdBy := strings.TrimSpace(req.Subject.UserID)
	if err := s.grantDefaultPermissions(ctx, m.MembershipID, companyID, createdBy); err != nil {
		return nil, err
	}

	resp := &AssignUserToCompanyResponse{
		UserID: userID, CompanyID: companyID, MembershipID: m.MembershipID,
	}

	// For invited users: re-issue invitation with company context so the email includes company info.
	if strings.EqualFold(strings.TrimSpace(acctStatus), "invited") && s.invMailer != nil {
		rawTok, tokHash, err := inviteRawTokenAndHash()
		if err == nil {
			expiresAt := time.Now().UTC().Add(s.inviteTTL)
			if replErr := s.repo.ReplaceUserInvitation(ctx, userID, s.idg.NewUUID(), tokHash, req.Subject.UserID, expiresAt); replErr == nil {
				companyDisplay := ""
				if cn, err := s.repo.GetCompanyName(ctx, companyID); err == nil {
					companyDisplay = cn
				}
				to := strings.TrimSpace(emailAddr)
				if to == "" {
					to = loginID
				}
				_ = s.invMailer.SendInvitationEmail(ctx, InvitationMailPayload{
					UserID: userID, ToEmail: to, FullName: fullName, LoginID: loginID,
					RawToken: rawTok, CompanyName: companyDisplay,
				})
				resp.ResendInvitation = true
			}
		}
	}

	return resp, nil
}

func (s *adminService) CreateMembership(ctx context.Context, req CreateMembershipRequest) (*MembershipView, error) {
	if err := s.authorize(ctx, req.Subject, "admin.membership.create", req.CompanyID); err != nil {
		return nil, err
	}
	m := MembershipView{MembershipID: s.idg.NewUUID(), UserID: req.UserID, CompanyID: req.CompanyID, CompanyName: req.CompanyID, Status: req.Status}
	if m.Status == "" {
		m.Status = "active"
	}
	result, err := s.repo.CreateMembership(ctx, m)
	if err != nil {
		return nil, err
	}
	if err := s.grantDefaultPermissions(ctx, result.MembershipID, req.CompanyID, req.Subject.UserID); err != nil {
		return nil, err
	}
	return result, nil
}
func (s *adminService) UpdateMembership(ctx context.Context, req UpdateMembershipRequest) (*MembershipView, error) {
	if err := s.authorize(ctx, req.Subject, "admin.membership.update", req.MembershipID); err != nil {
		return nil, err
	}
	if req.Status == "inactive" {
		m, err := s.repo.GetMembershipByID(ctx, req.MembershipID)
		if err == nil && m.IsPrimaryAdmin {
			return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "CANNOT_DEACTIVATE_PRIMARY_ADMIN", nil)
		}
	}
	return s.repo.UpdateMembershipStatus(ctx, req.MembershipID, req.Status)
}

func (s *adminService) DeleteMembership(ctx context.Context, req DeleteMembershipRequest) error {
	if err := s.authorize(ctx, req.Subject, "admin.membership.delete", req.MembershipID); err != nil {
		return err
	}
	m, err := s.repo.GetMembershipByID(ctx, req.MembershipID)
	if err == nil && m.IsPrimaryAdmin {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "CANNOT_DELETE_PRIMARY_ADMIN", nil)
	}
	return s.repo.DeleteMembership(ctx, req.MembershipID)
}

func (s *adminService) ListDepartmentTeams(ctx context.Context, req ListDepartmentTeamsRequest) ([]TeamView, error) {
	if err := s.authorize(ctx, req.Subject, "rbac.manage", ""); err != nil {
		return nil, err
	}
	return s.repo.ListDepartmentTeams(ctx, req.Subject.CompanyID, req.DepartmentID)
}

func (s *adminService) CreateTeam(ctx context.Context, req CreateTeamRequest) (*TeamView, error) {
	if err := s.authorize(ctx, req.Subject, "rbac.manage", ""); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "name is required", nil)
	}
	count, err := s.repo.CountTeamsInDepartment(ctx, req.DepartmentID)
	if err != nil {
		return nil, err
	}
	if count >= 5 {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "TEAM_LIMIT_REACHED", nil)
	}
	teamID := s.idg.NewUUID()
	return s.repo.CreateTeamRow(ctx, req.Subject.CompanyID, req.DepartmentID, teamID, name)
}

func (s *adminService) UpdateTeam(ctx context.Context, req UpdateTeamRequest) (*TeamView, error) {
	if err := s.authorize(ctx, req.Subject, "rbac.manage", ""); err != nil {
		return nil, err
	}
	return s.repo.PatchTeamRow(ctx, req.Subject.CompanyID, req.TeamID, req.Name, req.Status)
}

func (s *adminService) DeleteTeam(ctx context.Context, req DeleteTeamRequest) error {
	if err := s.authorize(ctx, req.Subject, "rbac.manage", ""); err != nil {
		return err
	}
	return s.repo.DeleteTeamRow(ctx, req.Subject.CompanyID, req.TeamID)
}

func (s *adminService) AddTeamMember(ctx context.Context, req AddTeamMemberRequest) error {
	if err := s.authorize(ctx, req.Subject, "rbac.manage", ""); err != nil {
		return err
	}
	// Member must belong to the parent department
	ok, err := s.repo.MemberBelongsToDepartment(ctx, req.MembershipID, req.DepartmentID)
	if err != nil {
		return err
	}
	if !ok {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "MEMBER_NOT_IN_DEPARTMENT", nil)
	}
	return s.repo.AddTeamMember(ctx, req.Subject.CompanyID, req.TeamID, req.MembershipID)
}

func (s *adminService) RemoveTeamMember(ctx context.Context, req RemoveTeamMemberRequest) error {
	if err := s.authorize(ctx, req.Subject, "rbac.manage", ""); err != nil {
		return err
	}
	return s.repo.RemoveTeamMember(ctx, req.Subject.CompanyID, req.TeamID, req.MembershipID)
}

func (s *adminService) AssignCompanyAdmin(ctx context.Context, req AssignCompanyAdminRequest) error {
	if err := s.authorize(ctx, req.Subject, "rbac.manage", ""); err != nil {
		return err
	}
	ok, err := s.repo.MembershipBelongsToCompany(ctx, req.MembershipID, req.Subject.CompanyID)
	if err != nil {
		return err
	}
	if !ok {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "membership not found in company", nil)
	}
	count, err := s.repo.CountAdminsInCompany(ctx, req.Subject.CompanyID)
	if err != nil {
		return err
	}
	if count >= 5 {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "ADMIN_LIMIT_REACHED", nil)
	}
	roleID, err := s.repo.LookupRoleIDForInvite(ctx, req.Subject.CompanyID, "", "company_admin", "company_admin")
	if err != nil || roleID == "" {
		return perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "company_admin role not found", nil)
	}
	return s.repo.AddRole(ctx, req.MembershipID, roleID)
}

func (s *adminService) RevokeCompanyAdmin(ctx context.Context, req RevokeCompanyAdminRequest) error {
	if err := s.authorize(ctx, req.Subject, "rbac.manage", ""); err != nil {
		return err
	}
	m, err := s.repo.GetMembershipByID(ctx, req.MembershipID)
	if err != nil {
		return err
	}
	if m.IsPrimaryAdmin {
		return perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "CANNOT_REVOKE_PRIMARY_ADMIN", nil)
	}
	roleID, err := s.repo.LookupRoleIDForInvite(ctx, req.Subject.CompanyID, "", "company_admin", "company_admin")
	if err != nil || roleID == "" {
		return perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "company_admin role not found", nil)
	}
	return s.repo.RemoveRole(ctx, req.MembershipID, roleID)
}

func (s *adminService) TransferOwnership(ctx context.Context, req TransferOwnershipRequest) error {
	if err := s.authorize(ctx, req.Subject, "rbac.manage", ""); err != nil {
		return err
	}
	caller, err := s.repo.GetMembershipByID(ctx, req.Subject.MembershipID)
	if err != nil {
		return err
	}
	if !caller.IsPrimaryAdmin {
		return perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "only the primary admin can transfer ownership", nil)
	}
	ok, err := s.repo.MembershipBelongsToCompany(ctx, req.TargetMembershipID, req.Subject.CompanyID)
	if err != nil {
		return err
	}
	if !ok {
		return perr.NewHTTPError(http.StatusNotFound, perr.CodeInvalidRequest, "target membership not found in company", nil)
	}
	if err := s.repo.ClearMembershipPrimaryAdmin(ctx, req.Subject.MembershipID); err != nil {
		return err
	}
	return s.repo.SetMembershipPrimaryAdmin(ctx, req.TargetMembershipID)
}
func (s *adminService) ListCompanyMemberships(ctx context.Context, req ListCompanyMembershipsRequest) (ListCompanyMembershipsResult, error) {
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	var all []MembershipView
	if req.ListWithoutCompany {
		isWebAdmin, err := s.hasPermission(ctx, req.Subject, "rbac.manage")
		if err != nil {
			return ListCompanyMembershipsResult{}, err
		}
		if !isWebAdmin {
			return ListCompanyMembershipsResult{}, perr.NewHTTPError(http.StatusForbidden, perr.CodePermissionDenied, "listing users without company requires rbac.manage", nil)
		}
		if err := s.authorizeMembershipInvite(ctx, req.Subject, req.Subject.CompanyID); err != nil {
			return ListCompanyMembershipsResult{}, err
		}
		items, err := s.repo.ListUsersWithNoMembership(ctx)
		if err != nil {
			return ListCompanyMembershipsResult{}, err
		}
		all = items
	} else {
		cid := strings.TrimSpace(req.CompanyID)
		if err := s.authorizeMembershipInvite(ctx, req.Subject, cid); err != nil {
			return ListCompanyMembershipsResult{}, err
		}
		scope, err := s.resolveInviteScope(ctx, req.Subject)
		if err != nil {
			return ListCompanyMembershipsResult{}, err
		}
		items, err := s.repo.ListMembershipsByCompany(ctx, cid)
		if err != nil {
			return ListCompanyMembershipsResult{}, err
		}
		filtered, err := s.filterMembershipsByInviteScope(items, scope)
		if err != nil {
			return ListCompanyMembershipsResult{}, err
		}
		all = filtered
	}

	total := len(all)
	start := (page - 1) * pageSize
	if start >= total {
		return ListCompanyMembershipsResult{Items: []MembershipView{}, Total: total, Page: page, PageSize: pageSize}, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return ListCompanyMembershipsResult{Items: all[start:end], Total: total, Page: page, PageSize: pageSize}, nil
}
func (s *adminService) AssignRole(ctx context.Context, req AssignRoleRequest) error {
	if err := s.authorize(ctx, req.Subject, "admin.membership.role.assign", req.MembershipID); err != nil {
		return err
	}
	return s.repo.AddRole(ctx, req.MembershipID, req.RoleID)
}
func (s *adminService) RemoveRole(ctx context.Context, req RemoveRoleRequest) error {
	if err := s.authorize(ctx, req.Subject, "admin.membership.role.remove", req.MembershipID); err != nil {
		return err
	}
	return s.repo.RemoveRole(ctx, req.MembershipID, req.RoleID)
}
func (s *adminService) AssignDepartment(ctx context.Context, req AssignDepartmentRequest) error {
	if err := s.authorize(ctx, req.Subject, "admin.membership.department.assign", req.MembershipID); err != nil {
		return err
	}
	return s.repo.AddDepartment(ctx, req.MembershipID, req.DepartmentID)
}
func (s *adminService) RemoveDepartment(ctx context.Context, req RemoveDepartmentRequest) error {
	if err := s.authorize(ctx, req.Subject, "admin.membership.department.remove", req.MembershipID); err != nil {
		return err
	}
	return s.repo.RemoveDepartment(ctx, req.MembershipID, req.DepartmentID)
}
func (s *adminService) AssignTitle(ctx context.Context, req AssignTitleRequest) error {
	if err := s.authorize(ctx, req.Subject, "admin.membership.title.assign", req.MembershipID); err != nil {
		return err
	}
	return s.repo.AddTitle(ctx, req.MembershipID, req.TitleID)
}
func (s *adminService) RemoveTitle(ctx context.Context, req RemoveTitleRequest) error {
	if err := s.authorize(ctx, req.Subject, "admin.membership.title.remove", req.MembershipID); err != nil {
		return err
	}
	return s.repo.RemoveTitle(ctx, req.MembershipID, req.TitleID)
}
func (s *adminService) ListPermissions(ctx context.Context, req AdminSubjectRequest) ([]string, error) {
	if err := s.authorize(ctx, req.Subject, "admin.permissions.list", ""); err != nil {
		return nil, err
	}
	return s.repo.ListPermissions(ctx)
}
func (s *adminService) ListRoles(ctx context.Context, req AdminSubjectRequest) ([]string, error) {
	if err := s.authorize(ctx, req.Subject, "admin.roles.list", ""); err != nil {
		return nil, err
	}
	return s.repo.ListRoles(ctx, req.Subject.CompanyID)
}
func (s *adminService) AssignRolePermission(ctx context.Context, req AssignRolePermissionRequest) error {
	if err := s.authorize(ctx, req.Subject, "admin.role.permission.assign", req.RoleID); err != nil {
		return err
	}
	return s.repo.AddRolePermission(ctx, req.RoleID, req.PermissionID)
}
func (s *adminService) RemoveRolePermission(ctx context.Context, req RemoveRolePermissionRequest) error {
	if err := s.authorize(ctx, req.Subject, "admin.role.permission.remove", req.RoleID); err != nil {
		return err
	}
	return s.repo.RemoveRolePermission(ctx, req.RoleID, req.PermissionID)
}
func (s *adminService) CreateResourceScopeRule(ctx context.Context, req CreateResourceScopeRuleRequest) error {
	if err := s.authorize(ctx, req.Subject, "admin.resource_scope_rule.create", ""); err != nil {
		return err
	}
	return s.repo.AddResourceScopeRule(ctx, req.Payload)
}
func (s *adminService) CreateWorkflowAssigneeRule(ctx context.Context, req CreateWorkflowAssigneeRuleRequest) error {
	if err := s.authorize(ctx, req.Subject, "admin.workflow_assignee_rule.create", ""); err != nil {
		return err
	}
	return s.repo.AddWorkflowAssigneeRule(ctx, req.Payload)
}
func (s *adminService) CreateNotificationRule(ctx context.Context, req CreateNotificationRuleRequest) error {
	if err := s.authorize(ctx, req.Subject, "admin.notification_rule.create", ""); err != nil {
		return err
	}
	return s.repo.AddNotificationRule(ctx, req.Payload)
}

func (s *adminService) ListNotificationRules(ctx context.Context, req ListNotificationRulesRequest) ([]NotificationRuleView, error) {
	if err := s.authorize(ctx, req.Subject, "admin.notification_rule.list", ""); err != nil {
		return nil, err
	}
	return s.repo.ListNotificationRules(ctx, req.Subject.CompanyID)
}

func (s *adminService) UpdateNotificationRule(ctx context.Context, req UpdateNotificationRuleRequest) error {
	if err := s.authorize(ctx, req.Subject, "admin.notification_rule.update", ""); err != nil {
		return err
	}
	return s.repo.UpdateNotificationRuleMerged(ctx, req.Subject.CompanyID, req.RuleID, req.PayloadPatch, req.Status)
}

func (s *adminService) DeleteNotificationRule(ctx context.Context, req DeleteNotificationRuleRequest) error {
	if err := s.authorize(ctx, req.Subject, "admin.notification_rule.delete", ""); err != nil {
		return err
	}
	return s.repo.DeleteNotificationRule(ctx, req.Subject.CompanyID, req.RuleID)
}

func (s *adminService) GetAdminAccountSettings(ctx context.Context, req GetAdminAccountSettingsRequest) (*AdminAccountSettingsView, error) {
	if err := s.authorize(ctx, req.Subject, "admin.account.settings.read", ""); err != nil {
		return nil, err
	}
	return s.repo.GetAdminAccountSettings(ctx, req.Subject.UserID)
}

func (s *adminService) PatchAdminAccountSettings(ctx context.Context, req PatchAdminAccountSettingsRequest) error {
	if err := s.authorize(ctx, req.Subject, "admin.account.settings.update", ""); err != nil {
		return err
	}
	return s.repo.PatchAdminAccountSettings(ctx, req.Subject.UserID, req.FullName, req.Email, req.Phone)
}

func (s *adminService) GetOwnCompany(ctx context.Context, req GetOwnCompanyRequest) (*PlatformCompanyDetail, error) {
	if req.Subject.CompanyID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company context required", nil)
	}
	if err := s.authorize(ctx, req.Subject, "company.view", req.Subject.CompanyID); err != nil {
		return nil, err
	}
	return s.repo.GetCompanyPlatform(ctx, req.Subject.CompanyID)
}

func (s *adminService) PatchOwnCompany(ctx context.Context, req PatchOwnCompanyRequest) (*PlatformCompanyDetail, error) {
	if req.Subject.CompanyID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company context required", nil)
	}
	if err := s.authorize(ctx, req.Subject, "company.edit", req.Subject.CompanyID); err != nil {
		return nil, err
	}
	// VerificationStatus and Status are intentionally absent — only platform admins may change those.
	if req.BusinessSector != nil {
		v := strings.TrimSpace(*req.BusinessSector)
		if v != "" {
			if v != "commercial" && v != "service" && v != "manufacturing" {
				return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid business_sector", nil)
			}
		} else {
			empty := ""
			req.BusinessSector = &empty
		}
	}
	if err := s.repo.UpdateCompanyPlatform(ctx, UpdatePlatformCompanyRequest{
		Subject:                       req.Subject,
		CompanyID:                     req.Subject.CompanyID,
		CompanyName:                   req.CompanyName,
		TaxCode:                       req.TaxCode,
		RegistrationNumber:            req.RegistrationNumber,
		Address:                       req.Address,
		Phone:                         req.Phone,
		ContactEmail:                  req.ContactEmail,
		RepresentativeName:            req.RepresentativeName,
		IsListed:                      req.IsListed,
		IsLargePublic:                 req.IsLargePublic,
		IsNonLargePublic:              req.IsNonLargePublic,
		HasSubsidiaries:               req.HasSubsidiaries,
		HasSubordinateAccountingUnits: req.HasSubordinateAccountingUnits,
		BusinessSector:                req.BusinessSector,
	}); err != nil {
		return nil, err
	}
	return s.repo.GetCompanyPlatform(ctx, req.Subject.CompanyID)
}

func (s *adminService) AddDirectPermission(ctx context.Context, req AddDirectPermissionRequest) error {
	if strings.TrimSpace(req.PermissionCode) == permissionInvite {
		if err := s.assertCanGrantInvitePermission(ctx, req.Subject); err != nil {
			return err
		}
	} else if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return err
	}
	if !isGrantable(req.PermissionCode) {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "permission_code is not grantable", nil)
	}
	return s.repo.InsertDirectPermission(ctx, req.MembershipID, req.Subject.CompanyID, req.PermissionCode, req.Subject.UserID)
}

func (s *adminService) RemoveDirectPermission(ctx context.Context, req RemoveDirectPermissionRequest) error {
	if strings.TrimSpace(req.PermissionCode) == permissionInvite {
		if err := s.assertCanGrantInvitePermission(ctx, req.Subject); err != nil {
			return err
		}
	} else if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return err
	}
	return s.repo.RevokeDirectPermission(ctx, req.MembershipID, req.PermissionCode, req.Subject.UserID)
}

func (s *adminService) ListDirectPermissions(ctx context.Context, req ListDirectPermissionsRequest) ([]DirectPermissionView, error) {
	if err := s.requireRbacManage(ctx, req.Subject); err != nil {
		return nil, err
	}
	return s.repo.ListActiveDirectPermissions(ctx, req.MembershipID)
}

func isGrantable(permCode string) bool {
	for _, p := range GrantablePermissions {
		if p == permCode {
			return true
		}
	}
	return false
}

// defaultMembershipPermissions are granted to every new active membership automatically.
// Using InsertDirectPermission (upsert) makes this idempotent.
var defaultMembershipPermissions = []string{
	"template.workflow.override.read",
}

func (s *adminService) grantDefaultPermissions(ctx context.Context, membershipID, companyID, grantedBy string) error {
	for _, p := range defaultMembershipPermissions {
		if err := s.repo.InsertDirectPermission(ctx, membershipID, companyID, p, grantedBy); err != nil {
			return fmt.Errorf("grant default permission %s: %w", p, err)
		}
	}
	return nil
}

func (s *adminService) assignInviteTitle(ctx context.Context, membershipID, titleID string) error {
	titleID = strings.TrimSpace(titleID)
	if titleID == "" {
		return nil
	}
	if err := s.repo.AddTitle(ctx, membershipID, titleID); err != nil {
		return fmt.Errorf("assign invite title: %w", err)
	}
	return nil
}

func (s *adminService) InitializeCompany(ctx context.Context, req InitializeCompanyRequest) (*InitializeCompanyResult, error) {
	name := strings.TrimSpace(req.CompanyName)
	if len([]rune(name)) < 2 || len(name) > 255 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_name must be 2-255 characters", nil)
	}

	accountStatus, emailVerified, err := s.repo.GetUserProvisioningGate(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	if !emailVerified {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodeEmailVerificationRequired, "email verification required", nil)
	}
	if strings.TrimSpace(accountStatus) != "active" {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodeAccountLocked, "account is not active", nil)
	}

	membershipID := s.idg.NewUUID()
	boot, err := s.repo.BootstrapSelfServiceCompanyTx(ctx, BootstrapSelfServiceInput{
		Mode:               BootstrapModeInitialize,
		UserID:             req.UserID,
		MembershipID:       membershipID,
		CompanyName:        name,
		TaxCode:            req.TaxCode,
		RegistrationNumber: req.RegistrationNumber,
		Address:            req.Address,
		Phone:              req.Phone,
		ContactEmail:       req.ContactEmail,
	})
	if err != nil {
		return nil, err
	}
	return &InitializeCompanyResult{
		CompanyID:    boot.CompanyID,
		CompanyCode:  boot.CompanyCode,
		CompanyName:  boot.CompanyName,
		MembershipID: boot.MembershipID,
	}, nil
}

func (s *adminService) CreateSelfServiceCompany(ctx context.Context, req CreateSelfServiceCompanyRequest) (*InitializeCompanyResult, error) {
	name := strings.TrimSpace(req.CompanyName)
	if len([]rune(name)) < 2 || len(name) > 255 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "company_name must be 2-255 characters", nil)
	}

	accountStatus, emailVerified, err := s.repo.GetUserProvisioningGate(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	if !emailVerified {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodeEmailVerificationRequired, "email verification required", nil)
	}
	if strings.TrimSpace(accountStatus) != "active" {
		return nil, perr.NewHTTPError(http.StatusForbidden, perr.CodeAccountLocked, "account is not active", nil)
	}

	tier := s.subscriptionTier(ctx, req.UserID)
	limit := selfServiceCompanyQuotaLimit(tier)
	displayTier := displaySubscriptionTier(tier)

	membershipID := s.idg.NewUUID()
	boot, err := s.repo.BootstrapSelfServiceCompanyTx(ctx, BootstrapSelfServiceInput{
		Mode:               BootstrapModeCreate,
		UserID:             req.UserID,
		MembershipID:       membershipID,
		CompanyName:        name,
		TaxCode:            req.TaxCode,
		RegistrationNumber: req.RegistrationNumber,
		Address:            req.Address,
		Phone:              req.Phone,
		ContactEmail:       req.ContactEmail,
		QuotaLimit:         limit,
		QuotaTier:          displayTier,
	})
	if err != nil {
		return nil, err
	}

	logCreateSelfServiceSuccess(ctx, req.UserID, boot, limit, tier)

	return &InitializeCompanyResult{
		CompanyID:    boot.CompanyID,
		CompanyCode:  boot.CompanyCode,
		CompanyName:  boot.CompanyName,
		MembershipID: boot.MembershipID,
	}, nil
}

// RollbackSelfServiceBootstrap compensates a committed bootstrap when session/token update fails.
func (s *adminService) RollbackSelfServiceBootstrap(ctx context.Context, companyID, userID string) error {
	return s.repo.RollbackBootstrapSelfServiceCompany(ctx, companyID, userID)
}

func (s *adminService) authorize(ctx context.Context, sub AdminSubject, action, resourceID string) error {
	decision, err := s.auth.Authorize(ctx, authapp.AuthorizeRequest{Subject: authapp.SubjectRef{UserID: sub.UserID, MembershipID: sub.MembershipID, CompanyID: sub.CompanyID}, Action: action, Resource: authapp.ResourceRef{Type: "admin_access", ID: resourceID, Attributes: map[string]any{}}})
	if err != nil {
		return fmt.Errorf("authorize admin action: %w", err)
	}
	if decision.Decision != authapp.DecisionAllow {
		code := perr.CodePermissionDenied
		if decision.DenyReasonCode != nil {
			code = *decision.DenyReasonCode
		}
		return perr.NewHTTPError(http.StatusForbidden, code, "access denied", nil)
	}
	return nil
}
