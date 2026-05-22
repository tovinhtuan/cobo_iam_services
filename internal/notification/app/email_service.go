package app

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// EmailNotificationService is the phase-2 implementation of
// NotificationService. It validates, resolves the template, renders to fail
// fast, sanitises variables, and persists an email_notifications row. It does
// NOT publish an outbox event in phase 2 — phase 3 wires the worker handler
// and the outbox publish step together.
//
// Callers should treat this service as additive: legacy email paths still
// build subject/body strings and publish their own outbox events. When the
// shadow-mode flag is set, the iam/reminder modules call DispatchEmail in
// parallel with the legacy path; if DispatchEmail fails (validation, render,
// persistence), the caller logs and continues — the legacy email still ships.
type EmailNotificationService struct {
	repo     EmailNotificationRepository
	registry TemplateRegistry
	renderer EmailRenderer
	idgen    idgen.Generator
	clock    func() time.Time
}

// NewEmailNotificationService wires the renderer + registry + repo. A nil
// idgen falls back to UUIDv7; a nil clock falls back to time.Now in UTC.
func NewEmailNotificationService(repo EmailNotificationRepository, registry TemplateRegistry, renderer EmailRenderer, idg idgen.Generator, clock func() time.Time) *EmailNotificationService {
	if idg == nil {
		idg = idgen.UUIDv7Generator{}
	}
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	return &EmailNotificationService{
		repo:     repo,
		registry: registry,
		renderer: renderer,
		idgen:    idg,
		clock:    clock,
	}
}

// DispatchEmail validates the request, dedups by idempotency key, resolves the
// template, runs a fail-fast render pass, and persists a sanitised
// email_notifications row. The returned record reflects what was stored
// (variables redacted). Outbox publish is intentionally deferred to phase 3.
func (s *EmailNotificationService) DispatchEmail(ctx context.Context, req DispatchEmailRequest) (*EmailNotification, error) {
	if err := validateDispatchRequest(req); err != nil {
		return nil, err
	}

	if existing, err := s.repo.FindByIdempotencyKey(ctx, req.IdempotencyKey); err == nil && existing != nil {
		return existing, nil
	}

	locale := strings.TrimSpace(req.Locale)
	if locale == "" {
		locale = "vi"
	}

	resolved, err := s.registry.Resolve(ctx, req.TemplateKey, locale)
	if err != nil {
		return nil, fmt.Errorf("%w: key=%s locale=%s: %v", ErrTemplateNotResolved, req.TemplateKey, locale, err)
	}

	// Fail fast: render with the in-memory plain vars so a missing required
	// field or template syntax error never reaches the persistence layer.
	if _, err := s.renderer.Render(resolved, req.Variables); err != nil {
		return nil, fmt.Errorf("%w: key=%s: %v", ErrTemplateRender, req.TemplateKey, err)
	}

	sanitisedJSON, err := SanitizeVariablesToJSON(req.Variables)
	if err != nil {
		return nil, fmt.Errorf("%w: sanitize variables: %v", ErrRepositoryPersist, err)
	}

	now := s.clock()
	notif := &EmailNotification{
		EmailNotificationID:    s.idgen.NewUUID(),
		CompanyID:              strings.TrimSpace(req.CompanyID),
		RecipientEmail:         strings.TrimSpace(req.To),
		TemplateKey:            req.TemplateKey,
		Locale:                 resolved.Locale,
		Status:                 EmailStatusPending,
		IdempotencyKey:         req.IdempotencyKey,
		TriggeredByUserID:      strings.TrimSpace(req.TriggeredByUserID),
		SourceEventType:        strings.TrimSpace(req.SourceEventType),
		SourceAggregateType:    strings.TrimSpace(req.SourceAggregateType),
		SourceAggregateID:      strings.TrimSpace(req.SourceAggregateID),
		VariablesJSONSanitized: sanitisedJSON,
		ScheduledAt:            req.ScheduledAt,
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	if err := s.repo.InsertNotification(ctx, notif); err != nil {
		// Lost a race with a concurrent dispatcher on the same idempotency
		// key: return the existing record so the caller sees the idempotent
		// outcome rather than a duplicate-key error.
		if errors.Is(err, ErrAlreadyDispatched) {
			if existing, lookupErr := s.repo.FindByIdempotencyKey(ctx, req.IdempotencyKey); lookupErr == nil && existing != nil {
				return existing, nil
			}
		}
		return nil, fmt.Errorf("%w: %v", ErrRepositoryPersist, err)
	}

	return notif, nil
}

// PreviewEmail renders a template with the supplied vars without persisting
// anything. Useful for admin tools and integration smoke tests.
func (s *EmailNotificationService) PreviewEmail(ctx context.Context, templateKey, locale string, vars map[string]any) (*RenderedEmail, error) {
	if strings.TrimSpace(templateKey) == "" {
		return nil, fmt.Errorf("%w: template_key is required", ErrDispatchValidation)
	}
	if strings.TrimSpace(locale) == "" {
		locale = "vi"
	}
	resolved, err := s.registry.Resolve(ctx, templateKey, locale)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTemplateNotResolved, err)
	}
	rendered, err := s.renderer.Render(resolved, vars)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTemplateRender, err)
	}
	return &rendered, nil
}

// validateDispatchRequest enforces the minimum invariants the persistence
// layer relies on. Recipient must be a syntactically valid email; idempotency
// key + template key must be non-empty.
func validateDispatchRequest(req DispatchEmailRequest) error {
	if strings.TrimSpace(req.To) == "" {
		return fmt.Errorf("%w: to is required", ErrDispatchValidation)
	}
	if _, err := mail.ParseAddress(req.To); err != nil {
		return fmt.Errorf("%w: invalid recipient %q: %v", ErrDispatchValidation, req.To, err)
	}
	if strings.TrimSpace(req.TemplateKey) == "" {
		return fmt.Errorf("%w: template_key is required", ErrDispatchValidation)
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return fmt.Errorf("%w: idempotency_key is required", ErrDispatchValidation)
	}
	return nil
}
