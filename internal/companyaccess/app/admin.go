package app

import "context"

type AdminService interface {
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
	CreateMembership(ctx context.Context, m MembershipView) (*MembershipView, error)
	UpdateMembershipStatus(ctx context.Context, membershipID, status string) (*MembershipView, error)
	DeleteMembership(ctx context.Context, membershipID string) error
	ListMembershipsByCompany(ctx context.Context, companyID string) ([]MembershipView, error)

	AddRole(ctx context.Context, membershipID, roleID string) error
	RemoveRole(ctx context.Context, membershipID, roleID string) error
	AddDepartment(ctx context.Context, membershipID, departmentID string) error
	RemoveDepartment(ctx context.Context, membershipID, departmentID string) error
	AddTitle(ctx context.Context, membershipID, titleID string) error
	RemoveTitle(ctx context.Context, membershipID, titleID string) error

	ListPermissions(ctx context.Context) ([]string, error)
	ListRoles(ctx context.Context) ([]string, error)
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
