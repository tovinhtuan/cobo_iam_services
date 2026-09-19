package workflowstepcomments

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// MySQLMentionsRepository persists workflow_step_comment_mentions.
type MySQLMentionsRepository struct {
	db *sql.DB
}

func NewMySQLMentionsRepository(db *sql.DB) *MySQLMentionsRepository {
	return &MySQLMentionsRepository{db: db}
}

func (r *MySQLMentionsRepository) dbtx(tx DBTX) DBTX {
	if tx != nil {
		return tx
	}
	return r.db
}

// BeginTx starts a DB transaction for Phase 3 comment+mention atomic writes.
func (r *MySQLMentionsRepository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}

// RunInTx runs fn inside a transaction; commit on nil error, otherwise rollback.
// Phase 3 will use this (or equivalent) so comment insert + mention replace share one Tx.
func (r *MySQLMentionsRepository) RunInTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin mentions tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit mentions tx: %w", err)
	}
	return nil
}

func (r *MySQLMentionsRepository) ListByComment(ctx context.Context, companyID, commentID string) ([]Mention, error) {
	companyID = strings.TrimSpace(companyID)
	commentID = strings.TrimSpace(commentID)
	if companyID == "" || commentID == "" {
		return nil, ErrInvalidMentions{Reason: "company_id and comment_id required"}
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, company_id, comment_id, mentioned_membership_id, start_offset, end_offset, created_at
		FROM workflow_step_comment_mentions
		WHERE company_id = ? AND comment_id = ?
		ORDER BY start_offset ASC, end_offset ASC, mentioned_membership_id ASC
	`, companyID, commentID)
	if err != nil {
		return nil, fmt.Errorf("list workflow_step_comment_mentions: %w", err)
	}
	defer rows.Close()
	out := make([]Mention, 0)
	for rows.Next() {
		var m Mention
		if err := rows.Scan(
			&m.ID, &m.CompanyID, &m.CommentID, &m.MentionedMembershipID,
			&m.StartOffset, &m.EndOffset, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan workflow_step_comment_mention: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *MySQLMentionsRepository) ReplaceForComment(ctx context.Context, tx DBTX, companyID, commentID string, mentions []Mention) error {
	prepared, err := PrepareMentionsForPersist(companyID, commentID, mentions)
	if err != nil {
		return err
	}
	exec := r.dbtx(tx)
	if err := deleteMentionsByComment(ctx, exec, companyID, commentID); err != nil {
		return err
	}
	for _, m := range prepared {
		if err := insertMention(ctx, exec, m); err != nil {
			return err
		}
	}
	return nil
}

func (r *MySQLMentionsRepository) DeleteByComment(ctx context.Context, tx DBTX, companyID, commentID string) error {
	companyID = strings.TrimSpace(companyID)
	commentID = strings.TrimSpace(commentID)
	if companyID == "" || commentID == "" {
		return ErrInvalidMentions{Reason: "company_id and comment_id required"}
	}
	return deleteMentionsByComment(ctx, r.dbtx(tx), companyID, commentID)
}

func deleteMentionsByComment(ctx context.Context, exec DBTX, companyID, commentID string) error {
	_, err := exec.ExecContext(ctx, `
		DELETE FROM workflow_step_comment_mentions
		WHERE company_id = ? AND comment_id = ?
	`, companyID, commentID)
	if err != nil {
		return fmt.Errorf("delete workflow_step_comment_mentions: %w", err)
	}
	return nil
}

func insertMention(ctx context.Context, exec DBTX, m Mention) error {
	_, err := exec.ExecContext(ctx, `
		INSERT INTO workflow_step_comment_mentions (
			id, company_id, comment_id, mentioned_membership_id, start_offset, end_offset, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, m.ID, m.CompanyID, m.CommentID, m.MentionedMembershipID, m.StartOffset, m.EndOffset, m.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert workflow_step_comment_mention: %w", err)
	}
	return nil
}
