package domain

// UserIdentity is a global identity across all companies.
type UserIdentity struct {
	UserID   string
	LoginID  string
	FullName string
	Status   string
}

// SessionContext binds authenticated identity to company context.
type SessionContext struct {
	SessionID    string
	UserID       string
	MembershipID string
	CompanyID    string
}
