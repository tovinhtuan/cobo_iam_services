package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	inappapp "github.com/cobo/cobo_iam_services/internal/inappnotification/app"
)

// Repository persists user_in_app_notifications in MySQL.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, n inappapp.InAppNotification) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_in_app_notifications
			(id, user_id, company_id, kind, title, body, resource_type, resource_id, is_read, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?)`,
		n.ID, n.UserID, n.CompanyID, n.Kind, n.Title,
		nullStr(n.Body), nullStrPtr(n.ResourceType), nullStrPtr(n.ResourceID),
		n.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("inappnotification create: %w", err)
	}
	return nil
}

func (r *Repository) ListByUser(ctx context.Context, userID, companyID string, limit int) ([]inappapp.InAppNotification, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, company_id, kind, title, body, resource_type, resource_id, is_read, created_at
		FROM user_in_app_notifications
		WHERE user_id = ? AND company_id = ?
		ORDER BY created_at DESC
		LIMIT ?`,
		userID, companyID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("inappnotification list: %w", err)
	}
	defer rows.Close()
	var out []inappapp.InAppNotification
	for rows.Next() {
		n, scanErr := scanNotif(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("inappnotification scan: %w", scanErr)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *Repository) UnreadCount(ctx context.Context, userID, companyID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM user_in_app_notifications
		WHERE user_id = ? AND company_id = ? AND is_read = 0`,
		userID, companyID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("inappnotification unread_count: %w", err)
	}
	return count, nil
}

func (r *Repository) MarkRead(ctx context.Context, userID, notifID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE user_in_app_notifications SET is_read = 1
		WHERE id = ? AND user_id = ?`,
		notifID, userID,
	)
	return err
}

func (r *Repository) MarkAllRead(ctx context.Context, userID, companyID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE user_in_app_notifications SET is_read = 1
		WHERE user_id = ? AND company_id = ? AND is_read = 0`,
		userID, companyID,
	)
	return err
}

// UserIDQuerier resolves user_ids from membership emails.
type UserIDQuerier struct {
	db *sql.DB
}

func NewUserIDQuerier(db *sql.DB) *UserIDQuerier {
	return &UserIDQuerier{db: db}
}

// UserIDsByEmails returns user_ids for active members with matching emails in a company.
func (q *UserIDQuerier) UserIDsByEmails(ctx context.Context, companyID string, emails []string) ([]string, error) {
	if len(emails) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(emails))
	placeholders = placeholders[:len(placeholders)-1]
	args := []any{companyID}
	for _, e := range emails {
		args = append(args, strings.ToLower(strings.TrimSpace(e)))
	}
	rows, err := q.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT DISTINCT u.user_id
		FROM memberships m
		JOIN users u ON u.user_id = m.user_id
		WHERE m.company_id = ?
		  AND m.membership_status = 'active'
		  AND LOWER(u.email) IN (%s)`, placeholders),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("inappnotification user_ids_by_emails: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		out = append(out, uid)
	}
	return out, rows.Err()
}

// scanner abstracts *sql.Rows for scanning.
type scanner interface {
	Scan(dest ...any) error
}

func scanNotif(s scanner) (inappapp.InAppNotification, error) {
	var n inappapp.InAppNotification
	var body sql.NullString
	var rt, ri sql.NullString
	err := s.Scan(
		&n.ID, &n.UserID, &n.CompanyID, &n.Kind, &n.Title,
		&body, &rt, &ri, &n.IsRead, &n.CreatedAt,
	)
	if err != nil {
		return n, err
	}
	if body.Valid {
		n.Body = body.String
	}
	if rt.Valid {
		n.ResourceType = &rt.String
	}
	if ri.Valid {
		n.ResourceID = &ri.String
	}
	return n, nil
}

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullStrPtr(s *string) sql.NullString {
	if s == nil || *s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}
