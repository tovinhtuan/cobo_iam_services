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
	MembershipID  string `json:"membership_id"`
	UserID        string `json:"user_id"`
	CompanyID     string `json:"company_id"`
	CompanyName   string `json:"company_name"`
	Status        string `json:"membership_status"`
	LoginID       string `json:"login_id,omitempty"`
	FullName      string `json:"full_name,omitempty"`
	AccountStatus string `json:"account_status,omitempty"`
}

type DepartmentView struct {
	DepartmentID   string
	DepartmentName string
}
