package app

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/cobo/cobo_iam_services/internal/platform/events"
)

// Email notification lifecycle status values. The phase-2 service writes
// pending; phase-3 worker transitions through sending -> sent or retry ->
// failed_permanent. cancelled is reserved for admin overrides.
const (
	EmailStatusPending         = "pending"
	EmailStatusSending         = "sending"
	EmailStatusSent            = "sent"
	EmailStatusRetry           = "retry"
	EmailStatusFailedPermanent = "failed_permanent"
	EmailStatusCancelled       = "cancelled"
)

// DispatchEmailRequest is the request to enqueue a single email notification.
// Phase 2 stops at persistence; phase 3 wires the outbox handler that consumes
// these records.
type DispatchEmailRequest struct {
	To             string
	TemplateKey    string
	Locale         string
	Variables      map[string]any
	IdempotencyKey string
	// TriggeredByUserID is the actor (or system "system") who caused the email.
	TriggeredByUserID string
	// SourceEventType, SourceAggregateType, SourceAggregateID describe the
	// business event that asked for this email (e.g. auth.password_reset_requested
	// over aggregate user/u_123). Used purely for audit/debug joins.
	SourceEventType     string
	SourceAggregateType string
	SourceAggregateID   string
	CompanyID           string
	ScheduledAt         *time.Time
}

// EmailNotification is the persisted record returned by the service.
// Variables in this struct are already sanitized (sensitive values redacted)
// because the same struct is the audit trail we expose to callers and dashboards.
type EmailNotification struct {
	EmailNotificationID      string
	CompanyID                string
	RecipientEmail           string
	TemplateKey              string
	Locale                   string
	Status                   string
	IdempotencyKey           string
	TriggeredByUserID        string
	SourceEventType          string
	SourceAggregateType      string
	SourceAggregateID        string
	VariablesJSONSanitized   string
	ScheduledAt              *time.Time
	LastErrorCode            string
	LastErrorMessageRedacted string
	SentAt                   *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// NotificationService is the email dispatch entry point that business modules
// (auth, reminder, ...) call instead of building subject/body strings or
// reaching directly into outbox. Phase 2 implements DispatchEmail and
// PreviewEmail without taking over delivery — legacy paths still send.
type NotificationService interface {
	DispatchEmail(ctx context.Context, req DispatchEmailRequest) (*EmailNotification, error)
	PreviewEmail(ctx context.Context, templateKey, locale string, vars map[string]any) (*RenderedEmail, error)
}

// EmailNotificationRepository persists email_notifications rows.
// Idempotency is enforced at the storage layer via a unique index on
// idempotency_key; FindByIdempotencyKey lets the service short-circuit duplicate
// dispatches before render.
type EmailNotificationRepository interface {
	InsertNotification(ctx context.Context, n *EmailNotification) error
	FindByIdempotencyKey(ctx context.Context, idempotencyKey string) (*EmailNotification, error)
	GetByID(ctx context.Context, notificationID string) (*EmailNotification, error)
	MarkSending(ctx context.Context, notificationID string, attemptedAt time.Time) error
	MarkSent(ctx context.Context, notificationID string, sentAt time.Time) error
	MarkRetry(ctx context.Context, notificationID, errorCode, errorMessageRedacted string, nextRetryAt time.Time) error
	MarkFailedPermanent(ctx context.Context, notificationID, errorCode, errorMessageRedacted string, failedAt time.Time) error
}

// OutboxPublisher is the narrow outbox-publish contract the transactional
// dispatch path depends on. Satisfied structurally by *outboxmysql.Repository
// without this package importing it concretely — keeps the
// transport -> app -> infra layering intact, mirroring how notificationapp.service
// depends on its own narrow TxJobRepository/outbox abstractions.
type OutboxPublisher interface {
	PublishEventTx(ctx context.Context, tx *sql.Tx, event events.Event) error
}

// TxEmailNotificationRepository is implemented by MySQL storage to insert an
// email_notifications row and publish its email.dispatch outbox event in one
// transaction. Mirrors TxJobRepository's shape exactly.
type TxEmailNotificationRepository interface {
	EmailNotificationRepository
	InsertNotificationTx(ctx context.Context, tx *sql.Tx, n *EmailNotification) error
}

// Errors surfaced by DispatchEmail. Callers in business modules map these to
// their own error envelope (HTTP / service) — shadow mode in the iam/reminder
// layer swallows them so the legacy path always wins.
var (
	ErrDispatchValidation   = errors.New("email dispatch: request validation failed")
	ErrTemplateNotResolved  = errors.New("email dispatch: template could not be resolved")
	ErrTemplateRender       = errors.New("email dispatch: template render failed")
	ErrRepositoryPersist    = errors.New("email dispatch: persistence failed")
	ErrAlreadyDispatched    = errors.New("email dispatch: notification with idempotency key already exists")
)
