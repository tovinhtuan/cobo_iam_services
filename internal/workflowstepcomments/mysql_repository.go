package workflowstepcomments

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// MySQLRepository persists workflow_step_comments.
type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) ListAliveByStep(ctx context.Context, owner OwnerContext, page, pageSize int) ([]Comment, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	var total int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM workflow_step_comments
		WHERE company_id = ? AND workflow_instance_id = ? AND step_code = ?
		  AND disclosure_record_id = ? AND deleted_at IS NULL
	`, owner.CompanyID, owner.WorkflowInstanceID, owner.StepCode, owner.DisclosureRecordID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count workflow_step_comments: %w", err)
	}
	offset := (page - 1) * pageSize
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, company_id, disclosure_record_id, workflow_instance_id, step_code,
		       author_user_id, author_membership_id, body, created_at, updated_at,
		       deleted_at, deleted_by_membership_id
		FROM workflow_step_comments
		WHERE company_id = ? AND workflow_instance_id = ? AND step_code = ?
		  AND disclosure_record_id = ? AND deleted_at IS NULL
		ORDER BY created_at ASC, id ASC
		LIMIT ? OFFSET ?
	`, owner.CompanyID, owner.WorkflowInstanceID, owner.StepCode, owner.DisclosureRecordID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list workflow_step_comments: %w", err)
	}
	defer rows.Close()
	out := make([]Comment, 0)
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

func (r *MySQLRepository) GetByIDInContext(ctx context.Context, owner OwnerContext, commentID string) (*Comment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, company_id, disclosure_record_id, workflow_instance_id, step_code,
		       author_user_id, author_membership_id, body, created_at, updated_at,
		       deleted_at, deleted_by_membership_id
		FROM workflow_step_comments
		WHERE company_id = ? AND id = ?
		LIMIT 1
	`, owner.CompanyID, commentID)
	c, err := scanComment(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get workflow_step_comment: %w", err)
	}
	if !c.MatchesOwner(owner) {
		return nil, errPathMismatch{}
	}
	return &c, nil
}

func (r *MySQLRepository) Insert(ctx context.Context, c Comment) error {
	return r.InsertInTx(ctx, nil, c)
}

func (r *MySQLRepository) InsertInTx(ctx context.Context, tx DBTX, c Comment) error {
	exec := DBTX(r.db)
	if tx != nil {
		exec = tx
	}
	_, err := exec.ExecContext(ctx, `
		INSERT INTO workflow_step_comments (
			id, company_id, disclosure_record_id, workflow_instance_id, step_code,
			author_user_id, author_membership_id, body, created_at, updated_at,
			deleted_at, deleted_by_membership_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, NULL)
	`, c.ID, c.CompanyID, c.DisclosureRecordID, c.WorkflowInstanceID, c.StepCode,
		c.AuthorUserID, c.AuthorMembershipID, c.Body, c.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert workflow_step_comment: %w", err)
	}
	return nil
}

func (r *MySQLRepository) UpdateBody(ctx context.Context, owner OwnerContext, commentID, body string, updatedAt time.Time) error {
	return r.UpdateBodyInTx(ctx, nil, owner, commentID, body, updatedAt)
}

func (r *MySQLRepository) UpdateBodyInTx(ctx context.Context, tx DBTX, owner OwnerContext, commentID, body string, updatedAt time.Time) error {
	exec := DBTX(r.db)
	if tx != nil {
		exec = tx
	}
	res, err := exec.ExecContext(ctx, `
		UPDATE workflow_step_comments
		SET body = ?, updated_at = ?
		WHERE company_id = ? AND id = ?
		  AND disclosure_record_id = ? AND workflow_instance_id = ? AND step_code = ?
		  AND deleted_at IS NULL
	`, body, updatedAt, owner.CompanyID, commentID,
		owner.DisclosureRecordID, owner.WorkflowInstanceID, owner.StepCode)
	if err != nil {
		return fmt.Errorf("update workflow_step_comment: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errNotFound{}
	}
	return nil
}

// BeginTx starts a SQL transaction for comment+mention atomic writes.
func (r *MySQLRepository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}

func (r *MySQLRepository) SoftDelete(ctx context.Context, owner OwnerContext, commentID, deletedByMembershipID string, deletedAt time.Time) error {
	var existingDeleted sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT deleted_at FROM workflow_step_comments
		WHERE company_id = ? AND id = ?
		  AND disclosure_record_id = ? AND workflow_instance_id = ? AND step_code = ?
		LIMIT 1
	`, owner.CompanyID, commentID, owner.DisclosureRecordID, owner.WorkflowInstanceID, owner.StepCode).Scan(&existingDeleted)
	if err == sql.ErrNoRows {
		return errNotFound{}
	}
	if err != nil {
		return fmt.Errorf("lookup workflow_step_comment for delete: %w", err)
	}
	if existingDeleted.Valid {
		return errAlreadyDeleted{}
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE workflow_step_comments
		SET deleted_at = ?, deleted_by_membership_id = ?
		WHERE company_id = ? AND id = ?
		  AND disclosure_record_id = ? AND workflow_instance_id = ? AND step_code = ?
		  AND deleted_at IS NULL
	`, deletedAt, nullIfEmpty(deletedByMembershipID), owner.CompanyID, commentID,
		owner.DisclosureRecordID, owner.WorkflowInstanceID, owner.StepCode)
	if err != nil {
		return fmt.Errorf("soft delete workflow_step_comment: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errNotFound{}
	}
	return nil
}

// MySQLDisplayNames resolves users.full_name by user_id.
type MySQLDisplayNames struct {
	db *sql.DB
}

func NewMySQLDisplayNames(db *sql.DB) *MySQLDisplayNames {
	return &MySQLDisplayNames{db: db}
}

func (r *MySQLDisplayNames) ResolveDisplayNames(ctx context.Context, userIDs []string) (map[string]string, error) {
	out := make(map[string]string, len(userIDs))
	uniq := make([]string, 0, len(userIDs))
	seen := map[string]struct{}{}
	for _, id := range userIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}
	if len(uniq) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(uniq))
	args := make([]any, len(uniq))
	for i, id := range uniq {
		placeholders[i] = "?"
		args[i] = id
	}
	q := fmt.Sprintf(`SELECT user_id, COALESCE(NULLIF(TRIM(full_name), ''), login_id, user_id) FROM users WHERE user_id IN (%s)`, strings.Join(placeholders, ","))
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return out, fmt.Errorf("resolve display names: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var uid, name string
		if err := rows.Scan(&uid, &name); err != nil {
			return out, err
		}
		out[uid] = name
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanComment(row rowScanner) (Comment, error) {
	var c Comment
	var updatedAt, deletedAt sql.NullTime
	var deletedBy sql.NullString
	err := row.Scan(
		&c.ID, &c.CompanyID, &c.DisclosureRecordID, &c.WorkflowInstanceID, &c.StepCode,
		&c.AuthorUserID, &c.AuthorMembershipID, &c.Body, &c.CreatedAt, &updatedAt,
		&deletedAt, &deletedBy,
	)
	if err != nil {
		return c, err
	}
	if updatedAt.Valid {
		t := updatedAt.Time
		c.UpdatedAt = &t
	}
	if deletedAt.Valid {
		t := deletedAt.Time
		c.DeletedAt = &t
	}
	if deletedBy.Valid {
		c.DeletedByMembershipID = deletedBy.String
	}
	return c, nil
}

func nullIfEmpty(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}
