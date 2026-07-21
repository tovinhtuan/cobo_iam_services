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
	GetInviteScope(ctx context.Context, req GetInviteScopeRequest) (*InviteScopeView, error)
	ResendUserInvitation(ctx context.Context, req ResendUserInvitationRequest) error
	// AssignUserToCompany links an existing user (active or invited) to a company.
	// For invited users it also re-issues the invitation with company context.
	AssignUserToCompany(ctx context.Context, req AssignUserToCompanyRequest) (*AssignUserToCompanyResponse, error)
	CreateMembership(ctx context.Context, req CreateMembershipRequest) (*MembershipView, error)
	UpdateMembership(ctx context.Context, req UpdateMembershipRequest) (*MembershipView, error)
	DeleteMembership(ctx context.Context, req DeleteMembershipRequest) error
	ListCompanyMemberships(ctx context.Context, req ListCompanyMembershipsRequest) (ListCompanyMembershipsResult, error)

	AssignRole(ctx context.Context, req AssignRoleRequest) error
	RemoveRole(ctx context.Context, req RemoveRoleRequest) error
	ReplaceMembershipPrimaryRole(ctx context.Context, req ReplaceMembershipPrimaryRoleRequest) error
	AssignDepartment(ctx context.Context, req AssignDepartmentRequest) error
	RemoveDepartment(ctx context.Context, req RemoveDepartmentRequest) error
	AssignTitle(ctx context.Context, req AssignTitleRequest) error
	RemoveTitle(ctx context.Context, req RemoveTitleRequest) error
	UpdateMembershipOrgAssignments(ctx context.Context, req UpdateMembershipOrgRequest) error

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

	ListPermissions(ctx context.Context, req AdminSubjectRequest) ([]PermissionListItem, error)
	ListGrantablePermissions(ctx context.Context, req AdminSubjectRequest) ([]GrantablePermissionItem, error)
	ListRoles(ctx context.Context, req AdminSubjectRequest) ([]RoleListItem, error)
	ListRolePermissions(ctx context.Context, req ListRolePermissionsRequest) (*RolePermissionsView, error)
	AssignRolePermission(ctx context.Context, req AssignRolePermissionRequest) error
	RemoveRolePermission(ctx context.Context, req RemoveRolePermissionRequest) error

	CreateResourceScopeRule(ctx context.Context, req CreateResourceScopeRuleRequest) error
	CreateWorkflowAssigneeRule(ctx context.Context, req CreateWorkflowAssigneeRuleRequest) error
	CreateNotificationRule(ctx context.Context, req CreateNotificationRuleRequest) error
	ListNotificationRules(ctx context.Context, req ListNotificationRulesRequest) ([]NotificationRuleView, error)
	GetNotificationRuleStatus(ctx context.Context, req GetNotificationRuleStatusRequest) (*NotificationRuleStatusView, error)
	UpdateNotificationRule(ctx context.Context, req UpdateNotificationRuleRequest) error
	DeleteNotificationRule(ctx context.Context, req DeleteNotificationRuleRequest) error
	SimulateNotificationRule(ctx context.Context, req SimulateNotificationRuleRequest, rawForbidden map[string]any) (*NotificationDispatchSimulateResult, error)
	GetConfigurationHealth(ctx context.Context, req GetConfigurationHealthRequest) (*ConfigurationHealthView, error)
	GetRuleRecommendations(ctx context.Context, req GetRuleRecommendationsRequest) (*RuleRecommendationsView, error)
	ValidateConfiguration(ctx context.Context, req ValidateConfigurationRequest) (*ConfigurationValidateView, error)
	GetObjectDependencies(ctx context.Context, req GetObjectDependenciesRequest) (*ObjectDependenciesView, error)
	GetOperationalDashboard(ctx context.Context, req GetOperationalDashboardRequest) (*OperationalDashboardView, error)
	ListAuditLogs(ctx context.Context, req ListAuditLogsRequest) (*AuditLogsListView, error)
	ListChangeTimeline(ctx context.Context, req ListChangeTimelineRequest) (*ChangeTimelineView, error)
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
	// CreateSelfServiceCompany creates an additional company for a user with at least one eligible membership (within quota).
	CreateSelfServiceCompany(ctx context.Context, req CreateSelfServiceCompanyRequest) (*InitializeCompanyResult, error)
	// RollbackSelfServiceBootstrap removes a freshly created self-service company after session failure.
	RollbackSelfServiceBootstrap(ctx context.Context, companyID, userID string) error

	// Configuration versioning (Sprint 5 Batch 1B).
	ListNotificationRuleVersions(ctx context.Context, req ListNotificationRuleVersionsRequest) (*ConfigVersionListView, error)
	GetNotificationRuleVersion(ctx context.Context, req GetNotificationRuleVersionRequest) (*ConfigVersionDetail, error)
	CompareNotificationRuleVersions(ctx context.Context, req CompareNotificationRuleVersionsRequest) (*CompareVersionsView, error)
	RollbackNotificationRuleVersion(ctx context.Context, req RollbackNotificationRuleVersionRequest) (*ConfigVersionRow, error)
	ListRBACMatrixVersions(ctx context.Context, req ListRBACMatrixVersionsRequest) (*ConfigVersionListView, error)
	GetRBACMatrixVersion(ctx context.Context, req GetRBACMatrixVersionRequest) (*ConfigVersionDetail, error)
	CompareRBACMatrixVersions(ctx context.Context, req CompareRBACMatrixVersionsRequest) (*CompareVersionsView, error)
	RollbackRBACMatrixVersion(ctx context.Context, req RollbackRBACMatrixVersionRequest) (*ConfigVersionRow, error)

	// Configuration approval (Sprint 5 Batch 2B).
	SubmitConfigApproval(ctx context.Context, req SubmitConfigApprovalRequest) (*PendingAdminChangeSummary, error)
	ListConfigApprovals(ctx context.Context, req ListConfigApprovalsRequest) (*ConfigApprovalListView, error)
	GetConfigApproval(ctx context.Context, req GetConfigApprovalRequest) (*PendingAdminChangeSummary, error)
	ApproveConfigApproval(ctx context.Context, req ApproveConfigApprovalRequest) (*PendingAdminChangeSummary, error)
	RejectConfigApproval(ctx context.Context, req RejectConfigApprovalRequest) (*PendingAdminChangeSummary, error)
	CancelConfigApproval(ctx context.Context, req CancelConfigApprovalRequest) (*PendingAdminChangeSummary, error)
	CompareConfigApproval(ctx context.Context, req CompareConfigApprovalRequest) (*CompareConfigApprovalView, error)

	// Delegated administration (Sprint 5 Batch 3B).
	CreateDelegation(ctx context.Context, req CreateDelegationRequest) (*DelegatedAdminGrant, error)
	ListDelegations(ctx context.Context, req ListDelegationsRequest) (*DelegationListView, error)
	GetDelegation(ctx context.Context, req GetDelegationRequest) (*DelegatedAdminGrant, error)
	PatchDelegation(ctx context.Context, req PatchDelegationRequest) (*DelegatedAdminGrant, error)
	RevokeDelegation(ctx context.Context, req RevokeDelegationRequest) (*DelegatedAdminGrant, error)

	// Break glass emergency access (Sprint 5 Batch 4B).
	CreateEmergencyAccessRequest(ctx context.Context, req CreateEmergencyAccessRequest) (*EmergencyAccessGrant, error)
	ListEmergencyAccessRequests(ctx context.Context, req ListEmergencyAccessRequests) (*EmergencyAccessListView, error)
	GetEmergencyAccessRequest(ctx context.Context, req GetEmergencyAccessRequest) (*EmergencyAccessGrant, error)
	ApproveEmergencyAccessRequest(ctx context.Context, req ApproveEmergencyAccessRequest) (*EmergencyAccessGrant, error)
	DenyEmergencyAccessRequest(ctx context.Context, req DenyEmergencyAccessRequest) (*EmergencyAccessGrant, error)
	CancelEmergencyAccessRequest(ctx context.Context, req CancelEmergencyAccessRequest) (*EmergencyAccessGrant, error)
	RevokeEmergencyAccessRequest(ctx context.Context, req RevokeEmergencyAccessRequest) (*EmergencyAccessGrant, error)
	GetEmergencyAccessTimeline(ctx context.Context, req GetEmergencyAccessTimelineRequest) (*ChangeTimelineView, error)

	// Configuration export (Sprint 5 Batch 5B).
	CreateConfigExport(ctx context.Context, req CreateConfigExportRequest) (*ConfigExportJobView, error)
	GetConfigExport(ctx context.Context, req GetConfigExportRequest) (*ConfigExportJobView, error)
	DownloadConfigExport(ctx context.Context, req DownloadConfigExportRequest) ([]byte, error)
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
	GetMembershipIDForUserCompany(ctx context.Context, userID, companyID string) (string, error)
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
	// CountEligibleMembershipsForUser counts memberships with status active or invited.
	CountEligibleMembershipsForUser(ctx context.Context, userID string) (int, error)
	// GetUserProvisioningGate returns account_status and whether email is verified.
	GetUserProvisioningGate(ctx context.Context, userID string) (accountStatus string, emailVerified bool, err error)
	// BootstrapSelfServiceCompanyTx provisions company + membership in one transaction (initialize or create mode).
	BootstrapSelfServiceCompanyTx(ctx context.Context, in BootstrapSelfServiceInput) (*BootstrapSelfServiceResult, error)
	// RollbackBootstrapSelfServiceCompany removes a freshly provisioned tenant when session update fails.
	RollbackBootstrapSelfServiceCompany(ctx context.Context, companyID, userID string) error

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
	// MembershipHasPermissionFromRole is true when any assigned role grants permissionCode.
	MembershipHasPermissionFromRole(ctx context.Context, membershipID, companyID, permissionCode string) (bool, error)
	// HasActiveDirectPermission is true when membership has a non-revoked direct grant for permissionCode.
	HasActiveDirectPermission(ctx context.Context, membershipID, permissionCode string) (bool, error)
	// ListDepartmentIDsByHeadMembership returns department IDs where head_membership_id matches.
	ListDepartmentIDsByHeadMembership(ctx context.Context, companyID, headMembershipID string) ([]string, error)
	// MembershipInAnyDepartment is true when membership belongs to at least one of departmentIDs.
	MembershipInAnyDepartment(ctx context.Context, membershipID string, departmentIDs []string) (bool, error)

	AddRole(ctx context.Context, membershipID, roleID string) error
	RemoveRole(ctx context.Context, membershipID, roleID string) error
	// ListMembershipRoles returns active roles assigned to a membership.
	ListMembershipRoles(ctx context.Context, membershipID string) ([]RoleView, error)
	AddDepartment(ctx context.Context, membershipID, departmentID string) error
	// UpsertDepartmentMembership links membership to department; isFocal marks department focal assignment.
	UpsertDepartmentMembership(ctx context.Context, membershipID, departmentID string, isFocal bool) error
	// ListFocalDepartmentIDs returns department IDs where membership is an active focal point.
	ListFocalDepartmentIDs(ctx context.Context, membershipID string) ([]string, error)
	// ListActiveMembershipDepartmentIDs returns active department IDs for a membership.
	ListActiveMembershipDepartmentIDs(ctx context.Context, membershipID string) ([]string, error)
	// ListActiveMembershipTitleIDs returns active title IDs for a membership.
	ListActiveMembershipTitleIDs(ctx context.Context, membershipID string) ([]string, error)
	// SetDepartmentMembershipFocal sets is_department_focal explicitly (membership must belong to department).
	SetDepartmentMembershipFocal(ctx context.Context, membershipID, departmentID string, isFocal bool) error
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

	ListPermissions(ctx context.Context) ([]PermissionListItem, error)
	// ListRoles returns roles for the given company: global roles (company_id NULL) plus roles scoped to that company.
	ListRoles(ctx context.Context, companyID string) ([]RoleListItem, error)
	ListRolePermissions(ctx context.Context, companyID, roleID string) (*RolePermissionsView, error)
	RoleAccessibleByCompany(ctx context.Context, companyID, roleID string) (bool, error)
	AddRolePermission(ctx context.Context, roleID, permissionID string) error
	RemoveRolePermission(ctx context.Context, roleID, permissionID string) error

	AddResourceScopeRule(ctx context.Context, rule map[string]any) error
	AddWorkflowAssigneeRule(ctx context.Context, rule map[string]any) error
	AddNotificationRule(ctx context.Context, rule map[string]any) error
	ListNotificationRules(ctx context.Context, companyID string) ([]NotificationRuleView, error)
	GetNotificationRuleByCode(ctx context.Context, companyID, ruleCode string) (*NotificationRuleView, error)
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

	// Configuration versioning (Sprint 5 Batch 1B).
	BuildNotificationRuleSnapshotJSON(ctx context.Context, companyID, ruleID string) ([]byte, error)
	BuildRBACMatrixSnapshotJSON(ctx context.Context, companyID string) ([]byte, error)
	InsertNotificationRuleVersion(ctx context.Context, in InsertNotificationRuleVersionInput) (*ConfigVersionRow, error)
	InsertRBACMatrixSnapshot(ctx context.Context, in InsertRBACMatrixSnapshotInput) (*ConfigVersionRow, error)
	ListNotificationRuleVersions(ctx context.Context, companyID, ruleID string, limit int) ([]ConfigVersionRow, error)
	GetNotificationRuleVersion(ctx context.Context, companyID, ruleID string, versionNo int) (*ConfigVersionDetail, error)
	ListRBACMatrixVersions(ctx context.Context, companyID string, limit int) ([]ConfigVersionRow, error)
	GetRBACMatrixVersion(ctx context.Context, companyID string, versionNo int) (*ConfigVersionDetail, error)
	RestoreNotificationRuleFromSnapshot(ctx context.Context, companyID string, raw []byte) error
	RestoreRBACMatrixFromSnapshot(ctx context.Context, companyID, actorUserID string, raw []byte) error

	// Configuration approval (Sprint 5 Batch 2B).
	InsertPendingAdminChange(ctx context.Context, in InsertPendingAdminChangeInput) (*PendingAdminChange, error)
	GetPendingAdminChange(ctx context.Context, companyID, approvalID string) (*PendingAdminChange, error)
	ListPendingAdminChanges(ctx context.Context, companyID, status, aggregateType string, limit int) ([]PendingAdminChange, error)
	HasPendingForAggregateStream(ctx context.Context, companyID, aggregateType, aggregateID string) (bool, error)
	UpdatePendingAdminChangeDecision(ctx context.Context, companyID, approvalID, status, reviewedBy, rejectReason string) (*PendingAdminChange, error)
	GetMaxNotificationRuleVersionNo(ctx context.Context, companyID, ruleID string) (int, error)
	GetMaxRBACMatrixVersionNo(ctx context.Context, companyID string) (int, error)
	ApplyPendingApprovalInTx(ctx context.Context, in ApplyPendingApprovalInput, row PendingAdminChange) (*ApplyPendingApprovalResult, error)

	// Delegated administration (Sprint 5 Batch 3B).
	InsertDelegationGrant(ctx context.Context, in InsertDelegationGrantInput) (*DelegatedAdminGrant, error)
	GetDelegationGrant(ctx context.Context, companyID, delegationID string) (*DelegatedAdminGrant, error)
	ListDelegationGrants(ctx context.Context, companyID, status, delegateeMembershipID, scopeID string, limit int) ([]DelegatedAdminGrant, error)
	ListActiveDelegationsForDelegatee(ctx context.Context, companyID, delegateeMembershipID string) ([]DelegatedAdminGrant, error)
	HasActiveDelegationGrant(ctx context.Context, companyID, delegateeMembershipID, scopeType, scopeID string) (bool, error)
	UpdateDelegationGrantPermissions(ctx context.Context, companyID, delegationID string, permissionSet []string, updatedBy string) (*DelegatedAdminGrant, error)
	RevokeDelegationGrant(ctx context.Context, companyID, delegationID, updatedBy string) (*DelegatedAdminGrant, error)

	// Break glass (M4).
	InsertEmergencyAccessGrant(ctx context.Context, in InsertEmergencyAccessGrantInput) (*EmergencyAccessGrant, error)
	GetEmergencyAccessGrant(ctx context.Context, companyID, sessionID string) (*EmergencyAccessGrant, error)
	ListEmergencyAccessGrants(ctx context.Context, companyID, status, targetMembershipID string, limit int) ([]EmergencyAccessGrant, error)
	GetActiveEmergencyGrantForTarget(ctx context.Context, companyID, targetMembershipID string) (*EmergencyAccessGrant, error)
	HasActiveEmergencyGrantForTarget(ctx context.Context, companyID, targetMembershipID string) (bool, error)
	RecordEmergencyFirstApproval(ctx context.Context, companyID, sessionID, approverMembershipID string) (*EmergencyAccessGrant, error)
	ActivateEmergencyGrant(ctx context.Context, companyID, sessionID, approverMembershipID string, expiresAt time.Time) (*EmergencyAccessGrant, error)
	DenyEmergencyGrant(ctx context.Context, companyID, sessionID string) (*EmergencyAccessGrant, error)
	CancelEmergencyGrant(ctx context.Context, companyID, sessionID string) (*EmergencyAccessGrant, error)
	RevokeEmergencyGrant(ctx context.Context, companyID, sessionID string) (*EmergencyAccessGrant, error)
	ExpireEmergencyGrant(ctx context.Context, companyID, sessionID string) (*EmergencyAccessGrant, error)
	ExpireDueEmergencyGrants(ctx context.Context, companyID string) (int, error)
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

// EmailVerificationIssuer stores a one-time email-verification token and sends the
// verify-link email for a freshly created (pending) user. Wired to iam.Service in
// production; nil in bootstrap/inmemory modes (staff-create then skips email issuance).
type EmailVerificationIssuer interface {
	IssueEmailVerificationLink(ctx context.Context, userID string) error
}

type InviteUserRequest struct {
	Subject          AdminSubject
	Email            string
	FullName         string
	CompanyID        string
	MembershipStatus string
	CreatedByUserID  string
	// Optional role for the new membership. If both empty, InviteDefaultRoleCode (e.g. user_thuong) is used.
	RoleID   string   `json:"role_id,omitempty"`
	RoleCode string   `json:"role_code,omitempty"`
	RoleIDs  []string `json:"role_ids,omitempty"`
	// Permissions is a subset of GrantablePermissions to apply as direct grants after membership creation.
	Permissions []string `json:"permissions,omitempty"`
	// DepartmentID is required when inviter is dept-scoped (direct grant only) and heads multiple departments.
	DepartmentID string `json:"department_id,omitempty"`
	// DepartmentIDs optionally assigns multiple departments during invite.
	DepartmentIDs []string `json:"department_ids,omitempty"`
	// TitleID optionally assigns one active company title to the created membership during invite.
	TitleID string `json:"title_id,omitempty"`
	// TitleIDs optionally assigns multiple titles during invite.
	TitleIDs []string `json:"title_ids,omitempty"`
	// IsDepartmentFocal marks the membership as focal for FocalDepartmentIDs.
	IsDepartmentFocal bool `json:"is_department_focal,omitempty"`
	// FocalDepartmentIDs is required when IsDepartmentFocal is true.
	FocalDepartmentIDs []string `json:"focal_department_ids,omitempty"`
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
	"admin.membership.invite",
	"template.workflow.override.write",
	"template.workflow.override.read",
	"template.workflow.override.approve",
	"template.workflow.override.reset",
	"ad_hoc_alert.propose",
	// ad_hoc_alert.focal_review: "Duyệt cảnh báo bất thường" — allows a membership to be
	// assigned as a reviewer on ad-hoc proposals (v3 D8, plan §7 Phase 3).
	"ad_hoc_alert.focal_review",
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

type GetInviteScopeRequest struct {
	Subject AdminSubject
}

// InviteScopeView describes company-wide vs department-scoped invite for the current subject.
type InviteScopeView struct {
	Scope       string            `json:"scope"` // company | department
	Departments []InviteScopeDept `json:"departments,omitempty"`
}

type InviteScopeDept struct {
	DepartmentID   string `json:"department_id"`
	DepartmentName string `json:"department_name"`
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
	// Optional membership bootstrap fields when company_id is set.
	RoleID       string   `json:"role_id,omitempty"`
	RoleCode     string   `json:"role_code,omitempty"`
	RoleIDs      []string `json:"role_ids,omitempty"`
	Permissions  []string `json:"permissions,omitempty"`
	DepartmentID  string   `json:"department_id,omitempty"`
	DepartmentIDs []string `json:"department_ids,omitempty"`
	TitleID       string   `json:"title_id,omitempty"`
	TitleIDs      []string `json:"title_ids,omitempty"`
	// IsDepartmentFocal marks the membership as focal for FocalDepartmentIDs.
	IsDepartmentFocal bool `json:"is_department_focal,omitempty"`
	// FocalDepartmentIDs is required when IsDepartmentFocal is true.
	FocalDepartmentIDs []string `json:"focal_department_ids,omitempty"`
}

type UpdateMembershipOrgRequest struct {
	Subject            AdminSubject
	MembershipID       string
	DepartmentIDs      []string `json:"department_ids"`
	TitleIDs           []string `json:"title_ids"`
	FocalDepartmentIDs []string `json:"focal_department_ids"`
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
	// Page is 1-indexed. Zero or negative values default to 1.
	Page int
	// PageSize is the number of items per page. Zero or negative values default to 20; max 100.
	PageSize int
}

// ListCompanyMembershipsResult holds a paginated membership list.
type ListCompanyMembershipsResult struct {
	Items    []MembershipView
	Total    int
	Page     int
	PageSize int
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
	Subject                       AdminSubject
	CompanyName                   *string `json:"company_name"`
	TaxCode                       *string `json:"tax_code"`
	RegistrationNumber            *string `json:"registration_number"`
	Address                       *string `json:"address"`
	Phone                         *string `json:"phone"`
	ContactEmail                  *string `json:"contact_email"`
	RepresentativeName            *string `json:"representative_name"`
	IsListed                      *bool   `json:"is_listed"`
	IsLargePublic                 *bool   `json:"is_large_public"`
	IsNonLargePublic              *bool   `json:"is_non_large_public"`
	HasSubsidiaries               *bool   `json:"has_subsidiaries"`
	HasSubordinateAccountingUnits *bool   `json:"has_subordinate_accounting_units"`
	BusinessSector                *string `json:"business_sector"`
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

// CreateSelfServiceCompanyRequest is the service input for POST /api/v1/company/create.
type CreateSelfServiceCompanyRequest struct {
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

const (
	ProvisioningSourceSelfServiceInitialize = "self_service_initialize"
	ProvisioningSourceSelfServiceCreate     = "self_service_create"
)

type BootstrapSelfServiceMode string

const (
	BootstrapModeInitialize BootstrapSelfServiceMode = "initialize"
	BootstrapModeCreate     BootstrapSelfServiceMode = "create"
)

type BootstrapSelfServiceInput struct {
	Mode               BootstrapSelfServiceMode
	UserID             string
	MembershipID       string
	CompanyName        string
	TaxCode            string
	RegistrationNumber string
	Address            string
	Phone              string
	ContactEmail       string
	// QuotaLimit is max self-provisioned companies allowed for create mode; 0 means unlimited (Enterprise).
	QuotaLimit int
	// QuotaTier is echoed in QUOTA_EXCEEDED details (empty tier is reported as Free).
	QuotaTier string
}

type BootstrapSelfServiceResult struct {
	CompanyID    string
	CompanyCode  string
	CompanyName  string
	MembershipID string
	// SelfProvisionedCount is the number of self-provisioned companies for the founder after this bootstrap (create mode logging).
	SelfProvisionedCount int
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
	Subject AdminSubject
	TitleID string
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
