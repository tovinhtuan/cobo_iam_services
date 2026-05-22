package mysql

import (
	"context"
	"database/sql"
	"fmt"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
)

// EmailDeliveryAttemptRepository persists rows in email_delivery_attempts
// (migration 0052). The worker writes one row per Send call.
type EmailDeliveryAttemptRepository struct {
	db *sql.DB
}

var _ notificationapp.EmailDeliveryAttemptRepository = (*EmailDeliveryAttemptRepository)(nil)

func NewEmailDeliveryAttemptRepository(db *sql.DB) *EmailDeliveryAttemptRepository {
	return &EmailDeliveryAttemptRepository{db: db}
}

func (r *EmailDeliveryAttemptRepository) InsertAttempt(ctx context.Context, a *notificationapp.DeliveryAttempt) error {
	if a == nil {
		return notificationapp.ErrRepositoryPersist
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO email_delivery_attempts (
			email_delivery_attempt_id, email_notification_id, attempt_no, provider, status,
			smtp_response_code, error_code, error_message_redacted, started_at, finished_at,
			next_retry_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		a.DeliveryAttemptID, a.NotificationID, a.AttemptNo, a.Provider, a.Status,
		nullableInt(a.SMTPResponseCode), nullString(a.ErrorCode), nullString(a.ErrorMessageRedacted),
		a.StartedAt, a.FinishedAt, nullTime(a.NextRetryAt), a.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert email_delivery_attempt: %w", err)
	}
	return nil
}

func (r *EmailDeliveryAttemptRepository) CountByNotificationID(ctx context.Context, notificationID string) (int, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM email_delivery_attempts WHERE email_notification_id = ?
	`, notificationID)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, fmt.Errorf("count attempts: %w", err)
	}
	return n, nil
}

func nullableInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}
