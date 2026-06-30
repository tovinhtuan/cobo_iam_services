package timeline

// TimelineEvent is the normalized change timeline output (Batch 5A/5B).
type TimelineEvent struct {
	ID            string         `json:"id"`
	OccurredAt    string         `json:"occurred_at"`
	Actor         Actor          `json:"actor"`
	Action        string         `json:"action"`
	Domain        string         `json:"domain"`
	ResourceType  string         `json:"resource_type"`
	ResourceID    string         `json:"resource_id"`
	Summary       string         `json:"summary"`
	Source        string         `json:"source"`
	ResourceLabel string         `json:"resource_label,omitempty"`
	Category      string         `json:"category,omitempty"`
	MetadataSafe  map[string]any `json:"metadata_safe,omitempty"`
	ActionLink    string         `json:"action_link,omitempty"`
	CorrelationID string         `json:"correlation_id,omitempty"`
}

// Actor is the normalized actor on a timeline event.
type Actor struct {
	UserID       string `json:"user_id"`
	MembershipID string `json:"membership_id,omitempty"`
	Display      string `json:"display,omitempty"`
}

const SourceAuditLogV1 = "audit_log_v1"
