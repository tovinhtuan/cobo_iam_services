package app

// ListFilter is a bounded audit list query (Batch 5B).
type ListFilter struct {
	CompanyID          string
	ActorUserID        string
	Action             string
	ActionPrefix       bool
	RequireAdminPrefix bool
	ResourceType       string
	ResourceID         string
	FromOccurredAt     string
	ToOccurredAt       string
	Cursor             string
	Limit              int
}
