package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// DeliveryAdapter is the wire-layer interface the worker uses to ship one
// rendered email. SMTP-backed and log-only implementations live under
// internal/notification/infra/smtp.
type DeliveryAdapter interface {
	Send(ctx context.Context, msg DeliveryMessage) (DeliveryResult, error)
}

// DeliveryMessage is what the worker hands to a DeliveryAdapter. Sender +
// reply-to are owned by the adapter (from SMTP config) so the message is
// transport-agnostic.
type DeliveryMessage struct {
	NotificationID string
	To             string
	Subject        string
	TextBody       string
	HTMLBody       string
}

// DeliveryResult is the successful outcome of a Send call. ProviderMessageID
// is the SMTP server's Message-ID (or a synthetic id for log-only mode); the
// worker writes it into the email_delivery_attempts audit row.
type DeliveryResult struct {
	ProviderMessageID string
	Provider          string // "smtp" | "log_only"
}

// DeliveryAttempt is one row in email_delivery_attempts. The worker writes
// exactly one of these per Send call (success or failure).
type DeliveryAttempt struct {
	DeliveryAttemptID    string
	NotificationID       string
	AttemptNo            int
	Provider             string
	Status               string // "sent" | "retry" | "failed_permanent" | "render_error"
	SMTPResponseCode     *int
	ErrorCode            string
	ErrorMessageRedacted string
	StartedAt            time.Time
	FinishedAt           time.Time
	NextRetryAt          *time.Time
	CreatedAt            time.Time
}

// EmailDeliveryAttemptRepository persists delivery attempt rows. The worker
// inserts one per Send. attempt_no is unique per notification so the index
// (notification_id, attempt_no) catches duplicate handler firings.
type EmailDeliveryAttemptRepository interface {
	InsertAttempt(ctx context.Context, a *DeliveryAttempt) error
	CountByNotificationID(ctx context.Context, notificationID string) (int, error)
}

// Delivery attempt status values for email_delivery_attempts.status. These
// are intentionally narrower than email_notifications.status because an
// attempt only describes a single Send try, not the overall notification.
const (
	AttemptStatusSent            = "sent"
	AttemptStatusRetry           = "retry"
	AttemptStatusFailedPermanent = "failed_permanent"
	AttemptStatusRenderError     = "render_error"
)

// ErrorClass tells the worker whether to back off and try again or give up.
type ErrorClass string

const (
	ErrorClassTransient        ErrorClass = "transient"
	ErrorClassPermanent        ErrorClass = "permanent"
	ErrorClassPermanentAuthOps ErrorClass = "permanent_auth"
)

// MaxEmailDeliveryAttempts caps how many Send calls we run before declaring
// failed_permanent. The SA at docs/email-notification-system-proposal.md §10
// fixes this at 5 with a 5-step backoff (the SA itself listed 4 steps; the
// implementation plan picked 5 steps so each attempt has a documented wait).
const MaxEmailDeliveryAttempts = 5

// EmailRetryBackoff[i] is the wait BEFORE attempt i+2 (index 0 ⇒ after
// attempt 1 failed, wait 1m, then attempt 2). Going past the end of the slice
// means the next attempt would exceed MaxEmailDeliveryAttempts and the
// notification must be marked failed_permanent instead.
var EmailRetryBackoff = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	1 * time.Hour,
	6 * time.Hour,
}

// NextRetryDelay returns the wait before the (attemptNo+1)-th attempt and a
// boolean indicating whether the worker should retry at all. attemptNo is the
// 1-based number of the attempt that just failed.
func NextRetryDelay(attemptNo int) (time.Duration, bool) {
	if attemptNo <= 0 {
		return 0, false
	}
	if attemptNo >= MaxEmailDeliveryAttempts {
		return 0, false
	}
	idx := attemptNo - 1
	if idx >= len(EmailRetryBackoff) {
		idx = len(EmailRetryBackoff) - 1
	}
	return EmailRetryBackoff[idx], true
}

// ClassifySMTPError maps a Send error to ErrorClass. The worker uses this
// together with NextRetryDelay to decide between mark_retry and
// mark_failed_permanent. The list of permanent SMTP codes follows the SA at
// docs/email-notification-system-proposal.md §10.2.
func ClassifySMTPError(err error) ErrorClass {
	if err == nil {
		return ErrorClassTransient
	}
	// net.Error is always transient — DNS hiccups, connection resets, etc.
	var ne net.Error
	if errors.As(err, &ne) {
		return ErrorClassTransient
	}
	msg := strings.ToUpper(err.Error())

	// Auth failures need oncall attention — they will not heal on their own.
	if strings.Contains(msg, "535") || strings.Contains(msg, "530") {
		return ErrorClassPermanentAuthOps
	}

	// Hard delivery failures: invalid recipient, mailbox not found,
	// transaction-level rejections.
	for _, code := range []string{"550", "551", "553", "554"} {
		if strings.Contains(msg, code) {
			return ErrorClassPermanent
		}
	}

	// Standard transient SMTP codes.
	for _, code := range []string{"421", "450", "451", "452"} {
		if strings.Contains(msg, code) {
			return ErrorClassTransient
		}
	}

	// Default: transient. Letting unknown errors retry is safer than declaring
	// permanent and losing the email; the retry cap protects against runaway
	// failures.
	return ErrorClassTransient
}

// RedactErrorMessage strips SMTP banners, recipient hints and other things
// that could echo back PII into structured logs. Used when storing
// email_delivery_attempts.error_message_redacted and when emitting events.
func RedactErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	// Keep only the first line and cap the length so a chatty server cannot
	// flood our log/metric storage.
	if i := strings.IndexAny(msg, "\r\n"); i > 0 {
		msg = msg[:i]
	}
	const maxLen = 200
	if len(msg) > maxLen {
		msg = msg[:maxLen] + "…"
	}
	// Hide anything that looks like an email address: support@…, user@host.
	parts := strings.Fields(msg)
	for i, p := range parts {
		if strings.Contains(p, "@") {
			parts[i] = "<email-redacted>"
		}
	}
	return strings.Join(parts, " ")
}

// FormatAttemptError gives the worker a consistent error_code string for the
// audit row, e.g. "transient_smtp" / "permanent_smtp_550".
func FormatAttemptError(class ErrorClass, err error) string {
	if err == nil {
		return ""
	}
	switch class {
	case ErrorClassTransient:
		return "transient_smtp"
	case ErrorClassPermanent:
		return "permanent_smtp"
	case ErrorClassPermanentAuthOps:
		return "permanent_smtp_auth"
	default:
		return fmt.Sprintf("unclassified:%s", class)
	}
}
