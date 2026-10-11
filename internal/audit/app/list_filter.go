package app

// ListFilter is a bounded audit list query (Batch 5B).
type ListFilter struct {
	CompanyID          string
	ActorUserID        string
	Action             string
	ActionPrefix       bool
	RequireAdminPrefix bool
	// ExcludeActionPrefix drops actions starting with this prefix (e.g. "cms.company" for tenant views).
	ExcludeActionPrefix string
	ResourceType        string
	ResourceID          string
	FromOccurredAt      string
	ToOccurredAt        string
	Cursor              string
	// CursorEventID breaks ties at Cursor's second (keyset on occurred_at, event_id); needs KeysetByEventID.
	CursorEventID string
	// KeysetByEventID orders by (occurred_at DESC, event_id DESC) and pages strictly after (Cursor, CursorEventID).
	KeysetByEventID bool
	Limit           int
}
