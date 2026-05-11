package app

import (
	"context"
	"time"
)

type AdminService interface {
	CreateUser(ctx context.Context, req CreateUserRequest) (*UserView, error)
	// CreateCompany provisions an empty tenant (companies row + default member roles); platform rbac.manage only.
	CreateCompany(ctx context.Context, req CreateCompanyRequest) (*CreateCompanyResult, error)
	ListPlatformCompanies(ctx context.Context, req ListPlatformCompaniesRequest) (*ListPlatformCompaniesResult, error)
	GetPlatformCompany(ctx context.Context, req GetPlatformCompanyRequest) (*PlatformCompanyDetail, error)
	UpdatePlatformCompany(ctx context.Context, req UpdatePlatformCompanyRequest) error
	SetPlatformCompanyStatus(ctx context.Context, req SetPlatformCompanyStatusRequest) error
	InviteUser(ctx context.Context, req InviteUserRequest) (*InviteUserResponse, error)
	// ListInviteRoles returns assignable roles for a target company (global + company-scoped), same resolution as invite.
	ListInviteRoles(ctx context.Context, req ListInviteRolesRequest) ([]InviteRoleOption, error)
	ResendUserInvitation(ctx context.Context, req ResendUserInvitationRequest) error
	// AssignUserToCompany links an existing user (active or invited) to a company.
	// For invited users it also re-issues the invitation with company context.
	AssignUserToCompany(ctx context.Context, req AssignUserToCompanyRequest) (*AssignUserToCompanyResponse, error)
	CreateMembership(ctx context.Context, req CreateMembershipRequest) (*MembershipView, error)
	UpdateMembership(ctx context.Context, req UpdateMembershipRequest) (*MembershipView, error)
	DeleteMembership(ctx context.Context, req DeleteMembershipRequest) error
	ListCompanyMemberships(ctx context.Context, req ListCompanyMembershipsRequest) ([]MembershipView, error)

	AssignRole(ctx context.Context, req AssignRoleRequest) error
	RemoveRole(ctx context.Context, req RemoveRoleRequest) error
	AssignDepartment(ctx context.Context, req AssignDepartmentRequest) error
	RemoveDepartment(ctx context.Context, req RemoveDepartmentRequest) error
	AssignTitle(ctx context.Context, req AssignTitleRequest) error
	RemoveTitle(ctx context.Context, req RemoveTitleRequest) error

	ListPermissions(ctx context.Context, req AdminSubjectRequest) ([]string, error)
	ListRoles(ctx context.Context, req AdminSubjectRequest) ([]string, error)
	AssignRolePermission(ctx context.Context, req AssignRolePermissionRequest) error
	RemoveRolePermission(ctx context.Context, req RemoveRolePermissionRequest) error

	CreateResourceScopeRule(ctx context.Context, req CreateResourceScopeRuleRequest) error
	CreateWorkflowAssigneeRule(ctx context.Context, req CreateWorkflowAssigneeRuleRequest) error
	CreateNotificationRule(ctx context.Context, req CreateNotificationRuleRequest) error
}

type AdminRepository interface {
	CreateUser(ctx context.Context, u UserView, passwordHash string, opts CreateUserOptions) (*UserView, error)
	InviteUserWithMembership(ctx context.Context, u UserView, opts CreateUserOptions, invitationID, tokenHash, createdByUserID string, expiresAt time.Time) (*UserView, error)
	// InviteUserWithoutCompany creates user + invitation with no membership (company-optional invite).
	InviteUserWithoutCompany(ctx context.Context, u UserView, invitationID, tokenHash, createdByUserID string, expiresAt time.Time) (*UserView, error)
	ReplaceUserInvitation(ctx context.Context, userID, invitationID, tokenHash, createdByUserID string, expiresAt time.Time) error
	LookupUserByLoginID(ctx context.Context, loginID string) (userID string, accountStatus string, found bool, err error)
	GetUserProfile(ctx context.Context, userID string) (loginID, email, fullName, accountStatus string, err error)
	MembershipExistsForUserCompany(ctx context.Context, userID, companyID string) (bool, error)
	// GetCompanyName returns companies.company_name for a valid company_id.
	GetCompanyName(ctx context.Context, companyID string) (string, error)
	// CreateStandaloneCompany inserts companies + seeded tenant roles (no users).
	CreateStandaloneCompany(ctx context.Context, displayName string, bootstrap CreateCompanyBootstrap) (companyID, companyCode string, err error)
	// CMS platform company directory (MySQL; no-ops / empty in in-memory repository).
	ListCompaniesPlatform(ctx context.Context, req ListPlatformCompaniesRequest) (*ListPlatformCompaniesResult, error)
	GetCompanyPlatform(ctx context.Context, companyID string) (*PlatformCompanyDetail, error)
	UpdateCompanyPlatform(ctx context.Context, req UpdatePlatformCompanyRequest) error
	SetCompanyStatusPlatform(ctx context.Context, companyID, status string) error
	CreateMembership(ctx context.Context, m MembershipView) (*MembershipView, error)
	UpdateMembershipStatus(ctx context.Context, membershipID, status string) (*MembershipView, error)
	DeleteMembership(ctx context.Context, membershipID string) error
	ListMembershipsByCompany(ctx context.Context, companyID string) ([]MembershipView, error)

	// LookupRoleIDForInvite resolves an assignable role for a new membership: explicit role_id,
	// or role_code / defaultRoleCode against companies.roles (company-specific overrides global).
	LookupRoleIDForInvite(ctx context.Context, companyID, preferRoleID, preferRoleCode, defaultRoleCode string) (string, error)
	// ListInviteRolesForCompany lists active roles assignable to memberships in companyID (global roles + that company).
	ListInviteRolesForCompany(ctx context.Context, companyID string) ([]InviteRoleOption, error)

	AddRole(ctx context.Context, membershipID, roleID string) error
	RemoveRole(ctx context.Context, membershipID, roleID string) error
	AddDepartment(ctx context.Context, membershipID, departmentID string) error
	RemoveDepartment(ctx context.Context, membershipID, departmentID string) error
	AddTitle(ctx context.Context, membershipID, titleID string) error
	RemoveTitle(ctx context.Context, membershipID, titleID string) error

	ListPermissions(ctx context.Context) ([]string, error)
	// ListRoles returns role_id values for the given company: global roles (company_id NULL) plus roles scoped to that company.
	ListRoles(ctx context.Context, companyID string) ([]string, error)
	AddRolePermission(ctx context.Context, roleID, permissionID string) error
	RemoveRolePermission(ctx context.Context, roleID, permissionID string) error

	AddResourceScopeRule(ctx context.Context, rule map[string]any) error
	AddWorkflowAssigneeRule(ctx context.Context, rule map[string]any) error
	AddNotificationRule(ctx context.Context, rule map[string]any) error
}

type AdminSubject struct {
	UserID       string
	MembershipID string
	CompanyID    string
}

type AdminSubjectRequest struct{ Subject AdminSubject }

type UserView struct {
	UserID        string `json:"user_id"`
	LoginID       string `json:"login_id"`
	FullName      string `json:"full_name"`
	Email         string `json:"email,omitempty"`
	Phone         string `json:"phone,omitempty"`
	AccountStatus string `json:"account_status"`
	// Optional output when create-user also creates membership in one call.
	MembershipID     string `json:"membership_id,omitempty"`
	MembershipStatus string `json:"membership_status,omitempty"`
	CompanyID        string `json:"company_id,omitempty"`
	CompanyName      string `json:"company_name,omitempty"`
}

type CreateUserOptions struct {
	MembershipID     string
	CompanyID        string
	MembershipStatus string
	// InitialRoleID optional: inserted into membership_roles when inviting/creating membership (invite flow).
	InitialRoleID string
}

// InvitationMailPayload is dispatched via outbox (IAM) after a user invitation is persisted.
type InvitationMailPayload struct {
	UserID, ToEmail, FullName, LoginID, RawToken string
	// CompanyName is shown in the email body (required by product). Empty skips the line.
	CompanyName string
}

// InvitationMailer sends invitation email payloads (wired to iam.Service in production).
type InvitationMailer interface {
	SendInvitationEmail(ctx context.Context, p InvitationMailPayload) error
}

type InviteUserRequest struct {
	Subject          AdminSubject
	Email            string
	FullName         string
	CompanyID        string
	MembershipStatus string
	CreatedByUserID  string
	// Optional role for the new membership. If both empty, InviteDefaultRoleCode (e.g. user_thuong) is used.
	RoleID   string `json:"role_id,omitempty"`
	RoleCode string `json:"role_code,omitempty"`
}

type InviteUserResponse struct {
	UserID              string `json:"user_id"`
	LoginID             string `json:"login_id"`
	Email               string `json:"email"`
	FullName            string `json:"full_name"`
	AccountStatus       string `json:"account_status"`
	MembershipID        string `json:"membership_id,omitempty"`
	CompanyID           string `json:"company_id,omitempty"`
	InvitationExpiresAt string `json:"invitation_expires_at,omitempty"`
}

// InviteRoleOption is one row from roles for invite UI (matches LookupRoleIDForInvite eligibility).
type InviteRoleOption struct {
	RoleID   string `json:"role_id"`
	RoleCode string `json:"role_code"`
	RoleName string `json:"role_name"`
}

// ListInviteRolesRequest scopes invite-role listing to a target company (same rules as InviteUser).
type ListInviteRolesRequest struct {
	Subject   AdminSubject
	CompanyID string
}

type ResendUserInvitationRequest struct {
	Subject   AdminSubject
	UserID    string
	CompanyID string
}

// AssignUserToCompanyRequest links an existing user (by login_id/email or user_id) to a company.
type AssignUserToCompanyRequest struct {
	Subject          AdminSubject
	UserID           string `json:"user_id"`
	CompanyID        string `json:"company_id"`
	MembershipStatus string `json:"membership_status"`
	RoleID           string `json:"role_id"`
	RoleCode         string `json:"role_code"`
}

type AssignUserToCompanyResponse struct {
	UserID       string `json:"user_id"`
	CompanyID    string `json:"company_id"`
	MembershipID string `json:"membership_id"`
	// ResendInvitation is true when the user was in "invited" state and a new invitation email was sent.
	ResendInvitation bool `json:"resend_invitation"`
}

type CreateCompanyRequest struct {
	Subject            AdminSubject
	CompanyName        string `json:"company_name"`
	TaxCode            string `json:"tax_code,omitempty"`
	RegistrationNumber string `json:"registration_number,omitempty"`
	Address            string `json:"address,omitempty"`
	Phone              string `json:"phone,omitempty"`
	ContactEmail       string `json:"contact_email,omitempty"`
	RepresentativeName string `json:"representative_name,omitempty"`
}

type CreateCompanyResult struct {
	CompanyID   string `json:"company_id"`
	CompanyCode string `json:"company_code"`
	CompanyName string `json:"company_name"`
}

type CreateUserRequest struct {
	Subject       AdminSubject
	LoginID       string `json:"login_id"`
	Password      string `json:"password"`
	FullName      string `json:"full_name"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	AccountStatus string `json:"account_status"`
	// Optional: if provided, user + membership are created atomically in one call.
	CompanyID        string `json:"company_id"`
	MembershipStatus string `json:"membership_status"`
}

type CreateMembershipRequest struct {
	Subject   AdminSubject
	UserID    string `json:"user_id"`
	CompanyID string `json:"company_id"`
	Status    string `json:"status"`
}

type UpdateMembershipRequest struct {
	Subject      AdminSubject
	MembershipID string
	Status       string `json:"status"`
}

type DeleteMembershipRequest struct {
	Subject      AdminSubject
	MembershipID string
}
type ListCompanyMembershipsRequest struct {
	Subject   AdminSubject
	CompanyID string
}

type AssignRoleRequest struct {
	Subject      AdminSubject
	MembershipID string
	RoleID       string `json:"role_id"`
}
type RemoveRoleRequest struct {
	Subject      AdminSubject
	MembershipID string
	RoleID       string
}
type AssignDepartmentRequest struct {
	Subject      AdminSubject
	MembershipID string
	DepartmentID string `json:"department_id"`
}
type RemoveDepartmentRequest struct {
	Subject      AdminSubject
	MembershipID string
	DepartmentID string
}
type AssignTitleRequest struct {
	Subject      AdminSubject
	MembershipID string
	TitleID      string `json:"title_id"`
}
type RemoveTitleRequest struct {
	Subject      AdminSubject
	MembershipID string
	TitleID      string
}

type AssignRolePermissionRequest struct {
	Subject      AdminSubject
	RoleID       string
	PermissionID string `json:"permission_id"`
}
type RemoveRolePermissionRequest struct {
	Subject      AdminSubject
	RoleID       string
	PermissionID string
}

type CreateResourceScopeRuleRequest struct {
	Subject AdminSubject
	Payload map[string]any
}
type CreateWorkflowAssigneeRuleRequest struct {
	Subject AdminSubject
	Payload map[string]any
}
type CreateNotificationRuleRequest struct {
	Subject AdminSubject
	Payload map[string]any
}
