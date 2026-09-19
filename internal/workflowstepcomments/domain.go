package workflowstepcomments

import "time"

const (
	MaxBodyLength = 4000

	EditWindow = 24 * time.Hour

	IdempotencyScope = "workflow_step_comment.create.v1"

	DefaultPageSize = 20
	MaxPageSize     = 100

	// CommentIDLength is the DB column width (VARCHAR(36)).
	// IDs are plain UUID strings with no prefix (COMMENT_ID_PREFIX=NONE).
	CommentIDLength = 36

	// Mention candidate search (V1.1 Phase 1).
	CandidateQMinRunes   = 2
	CandidateQMaxRunes   = 100
	CandidatePageDefault = 20
	CandidatePageMax     = 50

	// Mention persistence (V1.1 Phase 2).
	MaxMentionsPerComment = 20
	MentionIDLength       = 36
)

// OwnerContext binds a comment to a disclosure workflow step within a tenant.
type OwnerContext struct {
	CompanyID          string
	DisclosureRecordID string
	WorkflowInstanceID string
	StepCode           string
}

// Comment is the persisted step discussion row (soft-delete aware).
// ID is a plain UUID string (36 chars); never prefixed (e.g. no "wsc_").
type Comment struct {
	ID                    string
	CompanyID             string
	DisclosureRecordID    string
	WorkflowInstanceID    string
	StepCode              string
	AuthorUserID          string
	AuthorMembershipID    string
	Body                  string
	CreatedAt             time.Time
	UpdatedAt             *time.Time
	DeletedAt             *time.Time
	DeletedByMembershipID string
}

// IsAlive reports whether the comment is not soft-deleted.
func (c Comment) IsAlive() bool {
	return c.DeletedAt == nil
}

// MatchesOwner reports full path binding.
func (c Comment) MatchesOwner(owner OwnerContext) bool {
	return c.CompanyID == owner.CompanyID &&
		c.DisclosureRecordID == owner.DisclosureRecordID &&
		c.WorkflowInstanceID == owner.WorkflowInstanceID &&
		c.StepCode == owner.StepCode
}

// Mention is a persisted @mention row for a step comment.
// Offsets are UTF-8 rune positions on the trimmed comment body [start, end).
// Display names are never stored — resolve at read time in Phase 3+.
type Mention struct {
	ID                    string
	CompanyID             string
	CommentID             string
	MentionedMembershipID string
	StartOffset           int
	EndOffset             int
	CreatedAt             time.Time
}
