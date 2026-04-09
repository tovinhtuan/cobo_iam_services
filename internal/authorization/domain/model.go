package domain

// Subject identifies the principal in a tenant context.
type Subject struct {
	UserID       string `json:"user_id"`
	MembershipID string `json:"membership_id"`
	CompanyID    string `json:"company_id"`
}

// Resource points to target data for authorization.
type Resource struct {
	Type       string         `json:"type"`
	ID         string         `json:"id"`
	Attributes map[string]any `json:"attributes,omitempty"`
}
