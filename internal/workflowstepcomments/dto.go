package workflowstepcomments

// AuthorDTO identifies the comment author in API responses.
type AuthorDTO struct {
	UserID       string `json:"user_id"`
	MembershipID string `json:"membership_id"`
	DisplayName  string `json:"display_name"`
}

// CommentCapabilities is BE-owned per-comment mutation authority.
type CommentCapabilities struct {
	CanEdit   bool `json:"can_edit"`
	CanDelete bool `json:"can_delete"`
}

// CommentDTO is one alive comment in list/mutate responses.
// CommentID is a plain UUID string (36 chars); COMMENT_ID_PREFIX=NONE.
type CommentDTO struct {
	CommentID    string              `json:"comment_id"`
	Body         string              `json:"body"`
	Mentions     []MentionDTO        `json:"mentions,omitempty"`
	CreatedAt    string              `json:"created_at"`
	UpdatedAt    *string             `json:"updated_at"`
	Author       AuthorDTO           `json:"author"`
	Capabilities CommentCapabilities `json:"capabilities"`
}

// MentionDTO is a read-side mention with server-resolved display_name.
type MentionDTO struct {
	MembershipID string `json:"membership_id"`
	DisplayName  string `json:"display_name"`
	Start        int    `json:"start"`
	End          int    `json:"end"`
}

// ListCapabilities is BE-owned collection-level authority.
type ListCapabilities struct {
	CanCreate  bool `json:"can_create"`
	CanMention bool `json:"can_mention"`
}

// MentionCandidateDTO is one mention search hit (no email/phone).
type MentionCandidateDTO struct {
	MembershipID string `json:"membership_id"`
	DisplayName  string `json:"display_name"`
}

// MentionCandidatesResponse is the flat candidate search payload.
type MentionCandidatesResponse struct {
	Items    []MentionCandidateDTO `json:"items"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
	Total    int                   `json:"total"`
}

// ListResponse is the GET comments payload (flat).
type ListResponse struct {
	RecordID           string           `json:"record_id"`
	WorkflowInstanceID string           `json:"workflow_instance_id"`
	StepCode           string           `json:"step_code"`
	Completed          bool             `json:"completed"`
	Capabilities       ListCapabilities `json:"capabilities"`
	Page               int              `json:"page"`
	PageSize           int              `json:"page_size"`
	Total              int              `json:"total"`
	Comments           []CommentDTO     `json:"comments"`
}

// MutateResponse wraps a single comment after create/edit.
type MutateResponse struct {
	Comment CommentDTO `json:"comment"`
}

// CreateRequest is the POST body.
type CreateRequest struct {
	Body     string         `json:"body"`
	Mentions []MentionInput `json:"mentions,omitempty"`
}

// UpdateRequest is the PATCH body.
// Mentions omitted or [] clears all mention rows (contract Phase 3).
type UpdateRequest struct {
	Body     string         `json:"body"`
	Mentions []MentionInput `json:"mentions,omitempty"`
}
