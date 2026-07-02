package app

import "context"

// MembershipQueryService provides read operations used by IAM and authorization.
type MembershipQueryService interface {
	GetMembershipsByUser(ctx context.Context, userID string) ([]MembershipView, error)
	GetActiveMembership(ctx context.Context, userID, companyID string) (*MembershipView, error)
	GetMembershipRoles(ctx context.Context, membershipID string) ([]string, error)
	GetMembershipDepartments(ctx context.Context, membershipID string) ([]DepartmentView, error)
	GetMembershipTitles(ctx context.Context, membershipID string) ([]string, error)
}

// MembershipRepository is the persistence port for membership reads/writes.
type MembershipRepository interface {
	ListByUserID(ctx context.Context, userID string) ([]MembershipView, error)
	FindActiveByUserAndCompany(ctx context.Context, userID, companyID string) (*MembershipView, error)
	ListRoleCodes(ctx context.Context, membershipID string) ([]string, error)
	ListDepartments(ctx context.Context, membershipID string) ([]DepartmentView, error)
	ListTitleNames(ctx context.Context, membershipID string) ([]string, error)
}

type MembershipView struct {
	MembershipID   string           `json:"membership_id"`
	UserID         string           `json:"user_id"`
	CompanyID      string           `json:"company_id"`
	CompanyCode    string           `json:"company_code,omitempty"`
	CompanyName    string           `json:"company_name"`
	Status         string           `json:"membership_status"`
	LoginID        string           `json:"login_id,omitempty"`
	FullName       string           `json:"full_name,omitempty"`
	AccountStatus  string           `json:"account_status,omitempty"`
	IsPrimaryAdmin bool             `json:"is_primary_admin,omitempty"`
	Departments    []DepartmentView `json:"departments,omitempty"`
	Titles         []TitleView      `json:"titles,omitempty"`
	Roles          []RoleView       `json:"roles,omitempty"`
}

type RoleView struct {
	RoleID   string `json:"role_id"`
	RoleCode string `json:"role_code,omitempty"`
	RoleName string `json:"role_name,omitempty"`
}

type TitleView struct {
	TitleID     string `json:"title_id"`
	TitleName   string `json:"title_name,omitempty"` // legacy field used by MembershipQueryService
	Name        string `json:"name,omitempty"`        // admin API field (maps from title_name)
	MemberCount int    `json:"member_count,omitempty"`
	Status      string `json:"status,omitempty"`
	SortOrder   int    `json:"sort_order,omitempty"`
}

type DepartmentView struct {
	DepartmentID     string  `json:"department_id"`
	DepartmentName   string  `json:"department_name,omitempty"` // legacy field used by MembershipQueryService
	Name             string  `json:"name,omitempty"`            // admin API field (maps from department_name)
	HeadMembershipID *string `json:"head_membership_id,omitempty"`
	HeadFullName     *string `json:"head_full_name,omitempty"`
	MemberCount      int     `json:"member_count,omitempty"`
	Status           string  `json:"status,omitempty"`
	SortOrder        int     `json:"sort_order,omitempty"`
}

type TeamView struct {
	TeamID       string `json:"team_id"`
	DepartmentID string `json:"department_id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	MemberCount  int    `json:"member_count,omitempty"`
}
