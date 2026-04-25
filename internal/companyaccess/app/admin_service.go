package app

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
	"golang.org/x/crypto/bcrypt"
)

type adminService struct {
	repo AdminRepository
	auth authapp.Service
	idg  idgen.Generator
}

func NewAdminService(repo AdminRepository, auth authapp.Service, idg idgen.Generator) AdminService {
	return &adminService{repo: repo, auth: auth, idg: idg}
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
	if req.LoginID == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "login_id required", nil)
	}
	if req.FullName == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "full_name required", nil)
	}
	if len(req.Password) < 8 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "password must be at least 8 characters", nil)
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
		opts.MembershipID = s.idg.NewUUID()
	}
	return s.repo.CreateUser(ctx, u, string(hash), opts)
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

func (s *adminService) CreateMembership(ctx context.Context, req CreateMembershipRequest) (*MembershipView, error) {
	if err := s.authorize(ctx, req.Subject, "admin.membership.create", req.CompanyID); err != nil {
		return nil, err
	}
	m := MembershipView{MembershipID: s.idg.NewUUID(), UserID: req.UserID, CompanyID: req.CompanyID, CompanyName: req.CompanyID, Status: req.Status}
	if m.Status == "" {
		m.Status = "active"
	}
	return s.repo.CreateMembership(ctx, m)
}
func (s *adminService) UpdateMembership(ctx context.Context, req UpdateMembershipRequest) (*MembershipView, error) {
	if err := s.authorize(ctx, req.Subject, "admin.membership.update", req.MembershipID); err != nil {
		return nil, err
	}
	return s.repo.UpdateMembershipStatus(ctx, req.MembershipID, req.Status)
}
func (s *adminService) DeleteMembership(ctx context.Context, req DeleteMembershipRequest) error {
	if err := s.authorize(ctx, req.Subject, "admin.membership.delete", req.MembershipID); err != nil {
		return err
	}
	return s.repo.DeleteMembership(ctx, req.MembershipID)
}
func (s *adminService) ListCompanyMemberships(ctx context.Context, req ListCompanyMembershipsRequest) ([]MembershipView, error) {
	if err := s.authorize(ctx, req.Subject, "admin.membership.list", req.CompanyID); err != nil {
		return nil, err
	}
	return s.repo.ListMembershipsByCompany(ctx, req.CompanyID)
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
