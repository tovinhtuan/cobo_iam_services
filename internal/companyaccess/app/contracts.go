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
	MembershipID string
	UserID       string
	CompanyID    string
	CompanyName  string
	Status       string
}

type DepartmentView struct {
	DepartmentID   string
	DepartmentName string
}
