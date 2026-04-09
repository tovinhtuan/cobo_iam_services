package domain

import "time"

// Membership is the authorization principal inside a company.
type Membership struct {
	MembershipID  string
	UserID        string
	CompanyID     string
	Status        string
	EffectiveFrom *time.Time
	EffectiveTo   *time.Time
}

// CompanyCandidate is used in login/select-company flows.
type CompanyCandidate struct {
	CompanyID        string
	MembershipID     string
	CompanyName      string
	MembershipStatus string
}
