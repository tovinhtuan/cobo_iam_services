package workflowstepcomments

import (
	"context"
	"database/sql"
	"time"
)

// Repository persists step comments with tenant + path scoping.
type Repository interface {
	ListAliveByStep(ctx context.Context, owner OwnerContext, page, pageSize int) (items []Comment, total int, err error)
	GetByIDInContext(ctx context.Context, owner OwnerContext, commentID string) (*Comment, error)
	Insert(ctx context.Context, c Comment) error
	UpdateBody(ctx context.Context, owner OwnerContext, commentID, body string, updatedAt time.Time) error
	SoftDelete(ctx context.Context, owner OwnerContext, commentID, deletedByMembershipID string, deletedAt time.Time) error
	// InsertInTx / UpdateBodyInTx support Phase 3 comment+mention atomic writes.
	// When tx is nil, behavior matches Insert/UpdateBody (auto-commit / memory immediate).
	InsertInTx(ctx context.Context, tx DBTX, c Comment) error
	UpdateBodyInTx(ctx context.Context, tx DBTX, owner OwnerContext, commentID, body string, updatedAt time.Time) error
}

// DisplayNameResolver resolves author display names (best-effort).
type DisplayNameResolver interface {
	ResolveDisplayNames(ctx context.Context, userIDs []string) (map[string]string, error)
}

// DBTX is satisfied by *sql.DB and *sql.Tx so mention writes can join a Phase 3 comment transaction.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// MentionsRepository persists workflow_step_comment_mentions (V1.1 Phase 2).
// All methods require company_id; never query by comment_id alone.
//
// Transaction boundary (Phase 3):
//
//	comment insert/update + ReplaceForComment MUST share the same *sql.Tx.
//
// Soft-deleted parent: mention rows are NOT auto hard-deleted; LIST omits them via comment join.
// DeleteByComment is for rollback/cleanup callers only (not soft-delete of parent).
type MentionsRepository interface {
	ListByComment(ctx context.Context, companyID, commentID string) ([]Mention, error)
	ReplaceForComment(ctx context.Context, tx DBTX, companyID, commentID string, mentions []Mention) error
	DeleteByComment(ctx context.Context, tx DBTX, companyID, commentID string) error
}
