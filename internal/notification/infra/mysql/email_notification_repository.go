package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	notificationapp "github.com/cobo/cobo_iam_services/internal/notification/app"
)

// EmailNotificationRepository implements
// notificationapp.EmailNotificationRepository backed by the email_notifications
// table (migration 0051). Unique idempotency violations are translated to
// notificationapp.ErrAlreadyDispatched so the service can short-circuit
// without inspecting raw MySQL error codes.
type EmailNotificationRepository struct {
	db *sql.DB
}

var _ notificationapp.EmailNotificationRepository = (*EmailNotificationRepository)(nil)

func NewEmailNotificationRepository(db *sql.DB) *EmailNotificationRepository {
	return &EmailNotificationRepository{db: db}
}

func (r *EmailNotificationRepository) InsertNotification(ctx context.Context, n *notificationapp.EmailNotification) error {
	if n == nil {
		return notificationapp.ErrRepositoryPersist
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO email_notifications (
			email_notification_id, company_id, recipient_email, template_key, locale, status,
			idempotency_key, triggered_by_user_id, source_event_type, source_aggregate_type,
			source_aggregate_id, variables_json_sanitized, scheduled_at, last_error_code,
			last_error_message_redacted, sent_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		n.EmailNotificationID, nullString(n.CompanyID), n.RecipientEmail, n.TemplateKey, n.Locale, n.Status,
		n.IdempotencyKey, nullString(n.TriggeredByUserID), nullString(n.SourceEventType), nullString(n.SourceAggregateType),
		nullString(n.SourceAggregateID), n.VariablesJSONSanitized, nullTime(n.ScheduledAt), nullString(n.LastErrorCode),
		nullString(n.LastErrorMessageRedacted), nullTime(n.SentAt), n.CreatedAt, n.UpdatedAt,
	)
	if err != nil {
		if isDuplicateIdempotencyKeyErr(err) {
			return notificationapp.ErrAlreadyDispatched
		}
		return fmt.Errorf("insert email_notification: %w", err)
	}
	return nil
}

func (r *EmailNotificationRepository) FindByIdempotencyKey(ctx context.Context, idempotencyKey string) (*notificationapp.EmailNotification, error) {
	return r.queryOne(ctx, `idempotency_key = ?`, idempotencyKey)
}

func (r *EmailNotificationRepository) GetByID(ctx context.Context, notificationID string) (*notificationapp.EmailNotification, error) {
	return r.queryOne(ctx, `email_notification_id = ?`, notificationID)
}

func (r *EmailNotificationRepository) MarkSending(ctx context.Context, notificationID string, attemptedAt time.Time) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE email_notifications
		SET status = ?, updated_at = ?
		WHERE email_notification_id = ?
	`, notificationapp.EmailStatusSending, attemptedAt, notificationID)
	return checkAffected(res, err, "mark sending")
}

func (r *EmailNotificationRepository) MarkSent(ctx context.Context, notificationID string, sentAt time.Time) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE email_notifications
		SET status = ?, sent_at = ?, updated_at = ?
		WHERE email_notification_id = ?
	`, notificationapp.EmailStatusSent, sentAt, sentAt, notificationID)
	return checkAffected(res, err, "mark sent")
}

func (r *EmailNotificationRepository) MarkRetry(ctx context.Context, notificationID, errorCode, errorMessageRedacted string, nextRetryAt time.Time) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE email_notifications
		SET status = ?, last_error_code = ?, last_error_message_redacted = ?, updated_at = ?
		WHERE email_notification_id = ?
	`, notificationapp.EmailStatusRetry, errorCode, errorMessageRedacted, nextRetryAt, notificationID)
	return checkAffected(res, err, "mark retry")
}

func (r *EmailNotificationRepository) MarkFailedPermanent(ctx context.Context, notificationID, errorCode, errorMessageRedacted string, failedAt time.Time) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE email_notifications
		SET status = ?, last_error_code = ?, last_error_message_redacted = ?, updated_at = ?
		WHERE email_notification_id = ?
	`, notificationapp.EmailStatusFailedPermanent, errorCode, errorMessageRedacted, failedAt, notificationID)
	return checkAffected(res, err, "mark failed_permanent")
}

func (r *EmailNotificationRepository) queryOne(ctx context.Context, where string, args ...any) (*notificationapp.EmailNotification, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT email_notification_id, company_id, recipient_email, template_key, locale, status,
			idempotency_key, triggered_by_user_id, source_event_type, source_aggregate_type,
			source_aggregate_id, variables_json_sanitized, scheduled_at, last_error_code,
			last_error_message_redacted, sent_at, created_at, updated_at
		FROM email_notifications
		WHERE `+where+`
		LIMIT 1
	`, args...)
	var n notificationapp.EmailNotification
	var (
		companyID, triggeredBy, eventType, aggType, aggID sql.NullString
		lastErrorCode, lastErrorMsg                       sql.NullString
		scheduledAt, sentAt                               sql.NullTime
	)
	err := row.Scan(
		&n.EmailNotificationID, &companyID, &n.RecipientEmail, &n.TemplateKey, &n.Locale, &n.Status,
		&n.IdempotencyKey, &triggeredBy, &eventType, &aggType,
		&aggID, &n.VariablesJSONSanitized, &scheduledAt, &lastErrorCode,
		&lastErrorMsg, &sentAt, &n.CreatedAt, &n.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan email_notification: %w", err)
	}
	n.CompanyID = companyID.String
	n.TriggeredByUserID = triggeredBy.String
	n.SourceEventType = eventType.String
	n.SourceAggregateType = aggType.String
	n.SourceAggregateID = aggID.String
	n.LastErrorCode = lastErrorCode.String
	n.LastErrorMessageRedacted = lastErrorMsg.String
	if scheduledAt.Valid {
		t := scheduledAt.Time
		n.ScheduledAt = &t
	}
	if sentAt.Valid {
		t := sentAt.Time
		n.SentAt = &t
	}
	return &n, nil
}

func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

func checkAffected(res sql.Result, err error, op string) error {
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s rows affected: %w", op, err)
	}
	if n == 0 {
		return fmt.Errorf("%s: %w", op, notificationapp.ErrRepositoryPersist)
	}
	return nil
}

// isDuplicateIdempotencyKeyErr matches both MySQL error 1062 and the readable
// "Duplicate entry ... uk_email_notifications_idempotency" message so the
// service can convert any unique violation on that index to
// ErrAlreadyDispatched, regardless of driver version.
func isDuplicateIdempotencyKeyErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if !strings.Contains(msg, "uk_email_notifications_idempotency") &&
		!strings.Contains(msg, "Duplicate entry") {
		return false
	}
	return true
}
