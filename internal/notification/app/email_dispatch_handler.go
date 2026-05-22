package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// EmailDispatchOutboxEventType is the outbox event_type the NotificationService
// emits and the worker handler consumes. Kept as a package-level constant so the
// publisher and the consumer cannot drift apart.
const EmailDispatchOutboxEventType = "email.dispatch"

// EmailDispatchOutboxPayload is the JSON shape persisted in the outbox row.
// It carries the notification_id (for status reload + audit) plus the PLAIN
// template variables. The variables MUST NOT be stored in
// email_notifications.variables_json_sanitized — those are redacted; the
// outbox row is ephemeral and is removed after the handler succeeds, so the
// plain vars only live for the lifetime of a single delivery attempt.
type EmailDispatchOutboxPayload struct {
	NotificationID string         `json:"notification_id"`
	Variables      map[string]any `json:"variables,omitempty"`
}

// EmailDispatchHandler is the worker-side consumer of email.dispatch events.
// It is registered with the outbox processor by cmd/worker/main.go and is also
// unit-testable in isolation (no MySQL, no SMTP) using fake repositories +
// adapter.
type EmailDispatchHandler struct {
	notifRepo    EmailNotificationRepository
	attemptRepo  EmailDeliveryAttemptRepository
	registry     TemplateRegistry
	renderer     EmailRenderer
	adapter      DeliveryAdapter
	idg          idgen.Generator
	clock        func() time.Time
	maxAttempts  int
}

// NewEmailDispatchHandler wires the worker handler. maxAttempts can be 0 to
// pick up the package default (MaxEmailDeliveryAttempts) — tests pass a small
// value to exercise the cap quickly.
func NewEmailDispatchHandler(
	notifRepo EmailNotificationRepository,
	attemptRepo EmailDeliveryAttemptRepository,
	registry TemplateRegistry,
	renderer EmailRenderer,
	adapter DeliveryAdapter,
	idg idgen.Generator,
	clock func() time.Time,
	maxAttempts int,
) *EmailDispatchHandler {
	if idg == nil {
		idg = idgen.UUIDv7Generator{}
	}
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	if maxAttempts <= 0 {
		maxAttempts = MaxEmailDeliveryAttempts
	}
	return &EmailDispatchHandler{
		notifRepo:   notifRepo,
		attemptRepo: attemptRepo,
		registry:    registry,
		renderer:    renderer,
		adapter:     adapter,
		idg:         idg,
		clock:       clock,
		maxAttempts: maxAttempts,
	}
}

// Handle is called per outbox event. The handler is idempotent: redelivering
// the same event after a previous success returns nil without sending again.
// Returning a non-nil error tells the outbox processor to retry the event;
// returning nil marks the outbox row processed even if the notification ends
// up failed_permanent — the failure is recorded in email_delivery_attempts.
func (h *EmailDispatchHandler) Handle(ctx context.Context, payloadJSON []byte) error {
	var p EmailDispatchOutboxPayload
	if err := json.Unmarshal(payloadJSON, &p); err != nil {
		return fmt.Errorf("decode email.dispatch payload: %w", err)
	}
	notifID := strings.TrimSpace(p.NotificationID)
	if notifID == "" {
		return fmt.Errorf("email.dispatch payload missing notification_id")
	}

	notif, err := h.notifRepo.GetByID(ctx, notifID)
	if err != nil {
		return fmt.Errorf("load email_notification %s: %w", notifID, err)
	}
	if notif == nil {
		return fmt.Errorf("email_notification %s not found", notifID)
	}
	// Idempotent skip — a retried outbox event after success must not double-send.
	if notif.Status == EmailStatusSent {
		return nil
	}
	// Permanent failure is also terminal; don't try again.
	if notif.Status == EmailStatusFailedPermanent || notif.Status == EmailStatusCancelled {
		return nil
	}

	prevAttempts, err := h.attemptRepo.CountByNotificationID(ctx, notifID)
	if err != nil {
		return fmt.Errorf("count attempts for %s: %w", notifID, err)
	}
	attemptNo := prevAttempts + 1
	startedAt := h.clock()
	if err := h.notifRepo.MarkSending(ctx, notifID, startedAt); err != nil {
		return fmt.Errorf("mark sending %s: %w", notifID, err)
	}

	resolved, resolveErr := h.registry.Resolve(ctx, notif.TemplateKey, notif.Locale)
	if resolveErr != nil {
		return h.recordRenderFailure(ctx, notif, attemptNo, startedAt, "template_not_resolved", resolveErr)
	}
	// Plain template variables travel in the outbox payload only. The DB row
	// at email_notifications.variables_json_sanitized has secrets redacted
	// and is unsuitable for rendering — it would emit "[REDACTED]" inside the
	// OTP / reset link.
	rendered, renderErr := h.renderer.Render(resolved, p.Variables)
	if renderErr != nil {
		return h.recordRenderFailure(ctx, notif, attemptNo, startedAt, "template_render_failed", renderErr)
	}

	result, sendErr := h.adapter.Send(ctx, DeliveryMessage{
		NotificationID: notif.EmailNotificationID,
		To:             notif.RecipientEmail,
		Subject:        rendered.Subject,
		TextBody:       rendered.TextBody,
		HTMLBody:       rendered.HTMLBody,
	})
	finishedAt := h.clock()

	if sendErr == nil {
		attempt := &DeliveryAttempt{
			DeliveryAttemptID: h.idg.NewUUID(),
			NotificationID:    notif.EmailNotificationID,
			AttemptNo:         attemptNo,
			Provider:          result.Provider,
			Status:            AttemptStatusSent,
			StartedAt:         startedAt,
			FinishedAt:        finishedAt,
			CreatedAt:         finishedAt,
		}
		if err := h.attemptRepo.InsertAttempt(ctx, attempt); err != nil {
			// Attempt persistence failure is treated as transient — the outbox
			// will redeliver. Status stays at sending until the next attempt,
			// which is safe because the unique attempt_no would catch a true
			// double-send.
			return fmt.Errorf("record success attempt %d for %s: %w", attemptNo, notifID, err)
		}
		if err := h.notifRepo.MarkSent(ctx, notifID, finishedAt); err != nil {
			return fmt.Errorf("mark sent %s: %w", notifID, err)
		}
		return nil
	}

	class := ClassifySMTPError(sendErr)
	errCode := FormatAttemptError(class, sendErr)
	redacted := RedactErrorMessage(sendErr)
	delay, retryOK := NextRetryDelay(attemptNo)
	canRetry := retryOK && attemptNo < h.maxAttempts && class == ErrorClassTransient

	attempt := &DeliveryAttempt{
		DeliveryAttemptID:    h.idg.NewUUID(),
		NotificationID:       notif.EmailNotificationID,
		AttemptNo:            attemptNo,
		Provider:             "smtp",
		Status:               attemptStatusForOutcome(canRetry, class),
		ErrorCode:            errCode,
		ErrorMessageRedacted: redacted,
		StartedAt:            startedAt,
		FinishedAt:           finishedAt,
		CreatedAt:            finishedAt,
	}
	if canRetry {
		next := finishedAt.Add(delay)
		attempt.NextRetryAt = &next
	}
	if err := h.attemptRepo.InsertAttempt(ctx, attempt); err != nil {
		return fmt.Errorf("record failure attempt %d for %s: %w", attemptNo, notifID, err)
	}

	if canRetry {
		next := finishedAt.Add(delay)
		if err := h.notifRepo.MarkRetry(ctx, notifID, errCode, redacted, next); err != nil {
			return fmt.Errorf("mark retry %s: %w", notifID, err)
		}
		// Returning an error tells the outbox processor to redeliver; the
		// processor uses its own backoff which our application backoff is
		// composed on top of.
		return fmt.Errorf("transient delivery failure attempt %d: %w", attemptNo, sendErr)
	}

	if err := h.notifRepo.MarkFailedPermanent(ctx, notifID, errCode, redacted, finishedAt); err != nil {
		return fmt.Errorf("mark failed_permanent %s: %w", notifID, err)
	}
	// Returning nil here drops the outbox event — permanent failures are
	// terminal and must not loop. The audit row in email_delivery_attempts
	// keeps the full breadcrumb.
	return nil
}

func (h *EmailDispatchHandler) recordRenderFailure(ctx context.Context, notif *EmailNotification, attemptNo int, startedAt time.Time, errCode string, cause error) error {
	finishedAt := h.clock()
	attempt := &DeliveryAttempt{
		DeliveryAttemptID:    h.idg.NewUUID(),
		NotificationID:       notif.EmailNotificationID,
		AttemptNo:            attemptNo,
		Provider:             "smtp",
		Status:               AttemptStatusRenderError,
		ErrorCode:            errCode,
		ErrorMessageRedacted: RedactErrorMessage(cause),
		StartedAt:            startedAt,
		FinishedAt:           finishedAt,
		CreatedAt:            finishedAt,
	}
	if err := h.attemptRepo.InsertAttempt(ctx, attempt); err != nil {
		return fmt.Errorf("record render failure for %s: %w", notif.EmailNotificationID, err)
	}
	if err := h.notifRepo.MarkFailedPermanent(ctx, notif.EmailNotificationID, errCode, RedactErrorMessage(cause), finishedAt); err != nil {
		return fmt.Errorf("mark render failure permanent %s: %w", notif.EmailNotificationID, err)
	}
	return nil
}

func attemptStatusForOutcome(canRetry bool, class ErrorClass) string {
	if canRetry {
		return AttemptStatusRetry
	}
	if class == ErrorClassPermanent || class == ErrorClassPermanentAuthOps {
		return AttemptStatusFailedPermanent
	}
	return AttemptStatusFailedPermanent
}

// Sentinel error used by integration tests so they can assert the handler
// returned a transient retry signal.
var ErrTransientDeliveryRetried = errors.New("email dispatch: transient delivery, retry scheduled")
