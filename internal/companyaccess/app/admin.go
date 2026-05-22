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

	// Department CRUD (enterprise admin, guard: rbac.manage)
	ListDepartments(ctx context.Context, req ListDepartmentsRequest) ([]DepartmentView, error)
	CreateDepartment(ctx context.Context, req CreateDepartmentRequest) (*DepartmentView, error)
	UpdateDepartment(ctx context.Context, req UpdateDepartmentRequest) (*DepartmentView, error)
	DeleteDepartment(ctx context.Context, req DeleteDepartmentRequest) error
	AddDeptMember(ctx context.Context, req AddDeptMemberRequest) error
	RemoveDeptMember(ctx context.Context, req RemoveDeptMemberRequest) error

	// Title CRUD (enterprise admin, guard: rbac.manage)
	ListTitles(ctx context.Context, req ListTitlesRequest) ([]TitleView, error)
	CreateTitle(ctx context.Context, req CreateTitleRequest) (*TitleView, error)
	UpdateTitle(ctx context.Context, req UpdateTitleRequest) (*TitleView, error)
	DeleteTitle(ctx context.Context, req DeleteTitleRequest) error
	AddTitleMember(ctx context.Context, req AddTitleMemberRequest) error
	RemoveTitleMember(ctx context.Context, req RemoveTitleMemberRequest) error

	ListPermissions(ctx context.Context, req AdminSubjectRequest) ([]string, error)
	ListRoles(ctx context.Context, req AdminSubjectRequest) ([]string, error)
	AssignRolePermission(ctx context.Context, req AssignRolePermissionRequest) error
	RemoveRolePermission(ctx context.Context, req RemoveRolePermissionRequest) error

	CreateResourceScopeRule(ctx context.Context, req CreateResourceScopeRuleRequest) error
	CreateWorkflowAssigneeRule(ctx context.Context, req CreateWorkflowAssigneeRuleRequest) error
	CreateNotificationRule(ctx context.Context, req CreateNotificationRuleRequest) error
	ListNotificationRules(ctx context.Context, req ListNotificationRulesRequest) ([]NotificationRuleView, error)
	UpdateNotificationRule(ctx context.Context, req UpdateNotificationRuleRequest) error
	DeleteNotificationRule(ctx context.Context, req DeleteNotificationRuleRequest) error
	GetAdminAccountSettings(ctx context.Context, req GetAdminAccountSettingsRequest) (*AdminAccountSettingsView, error)
	PatchAdminAccountSettings(ctx context.Context, req PatchAdminAccountSettingsRequest) error

	// GetOwnCompany returns the profile of the enterprise admin's own company (scoped to sub.CompanyID).
	GetOwnCompany(ctx context.Context, req GetOwnCompanyRequest) (*PlatformCompanyDetail, error)
	// PatchOwnCompany updates editable profile fields of the enterprise admin's own company.
	// verification_status and status are intentionally excluded — only platform admins may change those.
	PatchOwnCompany(ctx context.Context, req PatchOwnCompanyRequest) (*PlatformCompanyDetail, error)

	// AddDirectPermission grants a grantable permission directly to a membership (not via role).
	AddDirectPermission(ctx context.Context, req AddDirectPermissionRequest) error
	// RemoveDirectPermission revokes a previously granted direct permission.
	RemoveDirectPermission(ctx context.Context, req RemoveDirectPermissionRequest) error
	// ListDirectPermissions returns active direct permission grants for a membership.
	ListDirectPermissions(ctx context.Context, req ListDirectPermissionsRequest) ([]DirectPermissionView, error)

	// Team CRUD (org_units)
	ListDepartmentTeams(ctx context.Context, req ListDepartmentTeamsRequest) ([]TeamView, error)
	CreateTeam(ctx context.Context, req CreateTeamRequest) (*TeamView, error)
	UpdateTeam(ctx context.Context, req UpdateTeamRequest) (*TeamView, error)
	DeleteTeam(ctx context.Context, req DeleteTeamRequest) error
	AddTeamMember(ctx context.Context, req AddTeamMemberRequest) error
	RemoveTeamMember(ctx context.Context, req RemoveTeamMemberRequest) error

	// AssignCompanyAdmin grants company_admin role to a membership (max 5 total admins).
	AssignCompanyAdmin(ctx context.Context, req AssignCompanyAdminRequest) error
	// RevokeCompanyAdmin removes company_admin role from a membership (primary admin cannot be revoked).
	RevokeCompanyAdmin(ctx context.Context, req RevokeCompanyAdminRequest) error
	// TransferOwnership atomically changes the primary admin from caller to target membership.
	TransferOwnership(ctx context.Context, req TransferOwnershipRequest) error

	// InitializeCompany creates a new company for a user with no existing membership (self-service onboarding).
	// Caller is responsible for issuing new session tokens after this returns.
	InitializeCompany(ctx context.Context, req InitializeCompanyRequest) (*InitializeCompanyResult, error)
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
	// ListUsersWithNoMembership returns users that have zero rows in memberships.
	ListUsersWithNoMembership(ctx context.Context) ([]MembershipView, error)
	// CountMembershipsForUser counts memberships for a user (any company).
	CountMembershipsForUser(ctx context.Context, userID string) (int, error)

	// LookupRoleIDForInvite resolves an assignable role for a new membership: explicit role_id,
	// or role_code / defaultRoleCode against companies.roles (company-specific overrides global).
	LookupRoleIDForInvite(ctx context.Context, companyID, preferRoleID, preferRoleCode, defaultRoleCode string) (string, error)
	// ListInviteRolesForCompany lists active roles assignable to memberships in companyID (global roles + that company),
	// with DefaultPermissions pre-populated from role_default_grant_permissions.
	ListInviteRolesForCompany(ctx context.Context, companyID string) ([]InviteRoleOption, error)

	// InsertDirectPermission inserts a membership_direct_permissions row (idempotent via INSERT IGNORE).
	InsertDirectPermission(ctx context.Context, membershipID, companyID, permCode, grantedBy string) error
	// RevokeDirectPermission sets revoked_at/revoked_by for an active direct grant.
	RevokeDirectPermission(ctx context.Context, membershipID, permCode, revokedBy string) error
	// ListActiveDirectPermissions returns all non-revoked direct grants for a membership.
	ListActiveDirectPermissions(ctx context.Context, membershipID string) ([]DirectPermissionView, error)

	AddRole(ctx context.Context, membershipID, roleID string) error
	RemoveRole(ctx context.Context, membershipID, roleID string) error
	AddDepartment(ctx context.Context, membershipID, departmentID string) error
	RemoveDepartment(ctx context.Context, membershipID, departmentID string) error
	AddTitle(ctx context.Context, membershipID, titleID string) error
	RemoveTitle(ctx context.Context, membershipID, titleID string) error

	// MembershipBelongsToCompany checks that a membership exists and its company_id matches.
	MembershipBelongsToCompany(ctx context.Context, membershipID, companyID string) (bool, error)

	// Department CRUD repository methods
	ListCompanyDepartments(ctx context.Context, companyID string) ([]DepartmentView, error)
	CreateDepartmentRow(ctx context.Context, companyID, deptID, deptCode, name string, headMembershipID *string, sortOrder int) (*DepartmentView, error)
	PatchDepartmentRow(ctx context.Context, companyID, deptID string, name *string, headMembershipID *string, clearHead bool, sortOrder *int, status *string) (*DepartmentView, error)
	SoftDeleteDepartment(ctx context.Context, deptID, companyID string) error
	CountDepartmentMembers(ctx context.Context, deptID string) (int, error)

	// Title CRUD repository methods
	ListCompanyTitles(ctx context.Context, companyID string) ([]TitleView, error)
	CreateTitleRow(ctx context.Context, companyID, titleID, titleCode, name string, sortOrder int) (*TitleView, error)
	PatchTitleRow(ctx context.Context, companyID, titleID string, name *string, sortOrder *int, status *string) (*TitleView, error)
	SoftDeleteTitle(ctx context.Context, titleID, companyID string) error
	CountTitleMembers(ctx context.Context, titleID string) (int, error)

	// Team CRUD repository methods (using org_units table)
	ListDepartmentTeams(ctx context.Context, companyID, departmentID string) ([]TeamView, error)
	CreateTeamRow(ctx context.Context, companyID, departmentID, teamID, name string) (*TeamView, error)
	PatchTeamRow(ctx context.Context, companyID, teamID string, name *string, status *string) (*TeamView, error)
	DeleteTeamRow(ctx context.Context, companyID, teamID string) error
	CountTeamsInDepartment(ctx context.Context, departmentID string) (int, error)
	AddTeamMember(ctx context.Context, companyID, teamID, membershipID string) error
	RemoveTeamMember(ctx context.Context, companyID, teamID, membershipID string) error
	MemberBelongsToDepartment(ctx context.Context, membershipID, departmentID string) (bool, error)

	ListPermissions(ctx context.Context) ([]string, error)
	// ListRoles returns role_id values for the given company: global roles (company_id NULL) plus roles scoped to that company.
	ListRoles(ctx context.Context, companyID string) ([]string, error)
	AddRolePermission(ctx context.Context, roleID, permissionID string) error
	RemoveRolePermission(ctx context.Context, roleID, permissionID string) error

	AddResourceScopeRule(ctx context.Context, rule map[string]any) error
	AddWorkflowAssigneeRule(ctx context.Context, rule map[string]any) error
	AddNotificationRule(ctx context.Context, rule map[string]any) error
	ListNotificationRules(ctx context.Context, companyID string) ([]NotificationRuleView, error)
	UpdateNotificationRuleMerged(ctx context.Context, companyID, ruleID string, payloadPatch map[string]any, status *string) error
	DeleteNotificationRule(ctx context.Context, companyID, ruleID string) error
	GetAdminAccountSettings(ctx context.Context, userID string) (*AdminAccountSettingsView, error)
	PatchAdminAccountSettings(ctx context.Context, userID string, fullName, email, phone *string) error

	// SetMembershipPrimaryAdmin sets is_primary_admin = true for the given membership.
	SetMembershipPrimaryAdmin(ctx context.Context, membershipID string) error
	// ClearMembershipPrimaryAdmin sets is_primary_admin = false for the given membership.
	ClearMembershipPrimaryAdmin(ctx context.Context, membershipID string) error
	// GetMembershipByID returns the MembershipView for a given membership_id, or an error if not found.
	GetMembershipByID(ctx context.Context, membershipID string) (*MembershipView, error)
	// CountAdminsInCompany returns the number of memberships with the company_admin role in the company.
	CountAdminsInCompany(ctx context.Context, companyID string) (int, error)
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
	// Permissions is a subset of GrantablePermissions to apply as direct grants after membership creation.
	Permissions []string `json:"permissions,omitempty"`
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

// GrantablePermissions is the whitelist of permissions that enterprise admin can grant/revoke directly
// on individual memberships. Validated by AddDirectPermission and InviteUser.
var GrantablePermissions = []string{
	"template.workflow.override.write",
	"template.workflow.override.read",
	"template.workflow.override.approve",
	"template.workflow.override.reset",
	"ad_hoc_alert.propose",
	"disclosure_type.manage",
}

// InviteRoleOption is one row from roles for invite UI (matches LookupRoleIDForInvite eligibility).
type InviteRoleOption struct {
	RoleID   string `json:"role_id"`
	RoleCode string `json:"role_code"`
	RoleName string `json:"role_name"`
	// DefaultPermissions lists grantable permission codes pre-checked for this role at invite time.
	DefaultPermissions []string `json:"default_permissions"`
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
	// ResendNoCompanyScope is true when the client sent an explicit empty company_id (e.g. ?company_id=):
	// resend invitation for an invited user with no memberships (platform rbac.manage only).
	ResendNoCompanyScope bool
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
	// CompanyID optional: empty creates user + password only (no membership), for callers with rbac.manage.
	// Enterprise admins without rbac.manage still get CompanyID forced to their tenant in the service layer.
	// Semantics align with optional company_id on InviteUser.
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
	// ListWithoutCompany is true when the client sent an explicit empty company_id (e.g. ?company_id=):
	// list users that have no membership rows (platform rbac.manage only).
	ListWithoutCompany bool
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

// NotificationRuleView is the list-row shape for GET /api/v1/admin/notification-rules.
type NotificationRuleView struct {
	NotificationRuleID string         `json:"notification_rule_id"`
	RuleCode           string         `json:"rule_code"`
	Status             string         `json:"status"`
	Payload            map[string]any `json:"payload"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type ListNotificationRulesRequest struct {
	Subject AdminSubject
}

type UpdateNotificationRuleRequest struct {
	Subject      AdminSubject
	RuleID       string
	PayloadPatch map[string]any
	Status       *string
}

type DeleteNotificationRuleRequest struct {
	Subject AdminSubject
	RuleID  string
}

type AdminAccountSettingsView struct {
	UserID        string `json:"user_id"`
	LoginID       string `json:"login_id"`
	FullName      string `json:"full_name"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	AccountStatus string `json:"account_status"`
}

type GetAdminAccountSettingsRequest struct {
	Subject AdminSubject
}

type PatchAdminAccountSettingsRequest struct {
	Subject  AdminSubject
	FullName *string `json:"full_name"`
	Email    *string `json:"email"`
	Phone    *string `json:"phone"`
}

type GetOwnCompanyRequest struct {
	Subject AdminSubject
}

type PatchOwnCompanyRequest struct {
	Subject            AdminSubject
	CompanyName        *string `json:"company_name"`
	TaxCode            *string `json:"tax_code"`
	RegistrationNumber *string `json:"registration_number"`
	Address            *string `json:"address"`
	Phone              *string `json:"phone"`
	ContactEmail       *string `json:"contact_email"`
	RepresentativeName *string `json:"representative_name"`
}

// DirectPermissionView is one active direct grant row returned by ListDirectPermissions.
type DirectPermissionView struct {
	PermissionCode string `json:"permission_code"`
	GrantedBy      string `json:"granted_by"`
	GrantedAt      string `json:"granted_at"`
}

type AddDirectPermissionRequest struct {
	Subject        AdminSubject
	MembershipID   string
	PermissionCode string `json:"permission_code"`
}

type RemoveDirectPermissionRequest struct {
	Subject        AdminSubject
	MembershipID   string
	PermissionCode string
}

type ListDirectPermissionsRequest struct {
	Subject      AdminSubject
	MembershipID string
}

type ListDepartmentTeamsRequest struct {
	Subject      AdminSubject
	DepartmentID string
}

type CreateTeamRequest struct {
	Subject      AdminSubject
	DepartmentID string
	Name         string
}

type UpdateTeamRequest struct {
	Subject AdminSubject
	TeamID  string
	Name    *string
	Status  *string
}

type DeleteTeamRequest struct {
	Subject AdminSubject
	TeamID  string
}

type AddTeamMemberRequest struct {
	Subject      AdminSubject
	TeamID       string
	DepartmentID string
	MembershipID string
}

type RemoveTeamMemberRequest struct {
	Subject      AdminSubject
	TeamID       string
	MembershipID string
}

type AssignCompanyAdminRequest struct {
	Subject      AdminSubject
	MembershipID string
}

type RevokeCompanyAdminRequest struct {
	Subject      AdminSubject
	MembershipID string
}

type TransferOwnershipRequest struct {
	Subject            AdminSubject
	TargetMembershipID string
}

type InitializeCompanyRequest struct {
	UserID             string
	CompanyName        string
	TaxCode            string
	RegistrationNumber string
	Address            string
	Phone              string
	ContactEmail       string
}

type InitializeCompanyResult struct {
	CompanyID    string `json:"company_id"`
	CompanyCode  string `json:"company_code"`
	CompanyName  string `json:"company_name"`
	MembershipID string `json:"membership_id"`
}

type ListDepartmentsRequest struct {
	Subject AdminSubject
}

type CreateDepartmentRequest struct {
	Subject          AdminSubject
	Name             string  `json:"name"`
	HeadMembershipID *string `json:"head_membership_id"`
	SortOrder        int     `json:"sort_order"`
}

type UpdateDepartmentRequest struct {
	Subject      AdminSubject
	DepartmentID string
	// nil = not provided (no change). For HeadMembershipID: "" = clear, non-empty = set.
	Name             *string `json:"name"`
	HeadMembershipID *string `json:"head_membership_id"`
	SortOrder        *int    `json:"sort_order"`
	Status           *string `json:"status"`
}

type DeleteDepartmentRequest struct {
	Subject      AdminSubject
	DepartmentID string
}

type AddDeptMemberRequest struct {
	Subject      AdminSubject
	DepartmentID string
	MembershipID string `json:"membership_id"`
}

type RemoveDeptMemberRequest struct {
	Subject      AdminSubject
	DepartmentID string
	MembershipID string
}

type ListTitlesRequest struct {
	Subject AdminSubject
}

type CreateTitleRequest struct {
	Subject   AdminSubject
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type UpdateTitleRequest struct {
	Subject   AdminSubject
	TitleID   string
	// nil = not provided (no change).
	Name      *string `json:"name"`
	SortOrder *int    `json:"sort_order"`
	Status    *string `json:"status"`
}

type DeleteTitleRequest struct {
	Subject AdminSubject
	TitleID string
}

type AddTitleMemberRequest struct {
	Subject      AdminSubject
	TitleID      string
	MembershipID string `json:"membership_id"`
}

type RemoveTitleMemberRequest struct {
	Subject      AdminSubject
	TitleID      string
	MembershipID string
}
