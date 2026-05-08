package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

type service struct {
	configRepo     ConfigRepository
	occurrenceRepo OccurrenceRepository
	attemptRepo    AttemptRepository
	emailSender    EmailSender
	metrics        Metrics
	auditor        Auditor
	alertHook      AlertHook
}

type EmailSender interface {
	SendReminderEmail(ctx context.Context, templateCode string, payload map[string]any, recipients []string, idempotencyKey string) (providerMessageID string, err error)
}

type ServiceOption func(*service)

func WithEmailSender(sender EmailSender) ServiceOption {
	return func(s *service) {
		s.emailSender = sender
	}
}

func WithMetrics(metrics Metrics) ServiceOption {
	return func(s *service) {
		s.metrics = metrics
	}
}

func WithAuditor(auditor Auditor) ServiceOption {
	return func(s *service) {
		s.auditor = auditor
	}
}

func WithAlertHook(hook AlertHook) ServiceOption {
	return func(s *service) {
		s.alertHook = hook
	}
}

func NewService(configRepo ConfigRepository, occurrenceRepo OccurrenceRepository, attemptRepo AttemptRepository, opts ...ServiceOption) Service {
	s := &service{
		configRepo:     configRepo,
		occurrenceRepo: occurrenceRepo,
		attemptRepo:    attemptRepo,
		metrics:        noopMetrics{},
		auditor:        noopAuditor{},
		alertHook:      noopAlertHook{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

func (s *service) UpsertDisclosureReminderConfig(ctx context.Context, req UpsertReminderConfigRequest) (*ReminderConfigDTO, error) {
	if strings.TrimSpace(req.DisclosureID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "disclosure id is required", nil)
	}
	if err := validateReminderConfig(req.Config); err != nil {
		return nil, err
	}
	in := ReminderConfigDTO{
		ScopeType: ScopeTypeDisclosure,
		ScopeID:   req.DisclosureID,
		Config:    req.Config,
		UpdatedBy: req.Subject.UserID,
	}
	out, err := s.configRepo.UpsertByScope(ctx, in)
	if err == nil {
		s.metrics.IncCounter("reminder_config_upsert_total", map[string]string{"scope_type": string(in.ScopeType)})
		s.auditor.Record(ctx, "REMINDER_CONFIG_UPSERTED", "reminder_config", in.ScopeID, map[string]any{
			"scope_type": in.ScopeType,
		})
	}
	return out, err
}

func (s *service) UpsertWorkflowStepReminderConfig(ctx context.Context, req UpsertReminderConfigRequest) (*ReminderConfigDTO, error) {
	if strings.TrimSpace(req.WorkflowStepID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "workflow step id is required", nil)
	}
	if err := validateReminderConfig(req.Config); err != nil {
		return nil, err
	}
	in := ReminderConfigDTO{
		ScopeType: ScopeTypeWorkflowStep,
		ScopeID:   strings.TrimSpace(req.DisclosureID) + ":" + strings.TrimSpace(req.WorkflowStepID),
		Config:    req.Config,
		UpdatedBy: req.Subject.UserID,
	}
	out, err := s.configRepo.UpsertByScope(ctx, in)
	if err == nil {
		s.metrics.IncCounter("reminder_config_upsert_total", map[string]string{"scope_type": string(in.ScopeType)})
		s.auditor.Record(ctx, "REMINDER_CONFIG_UPSERTED", "reminder_config", in.ScopeID, map[string]any{
			"scope_type": in.ScopeType,
		})
	}
	return out, err
}

func (s *service) GetDisclosureReminderConfig(ctx context.Context, req GetReminderConfigRequest) (*ReminderConfigDTO, error) {
	if strings.TrimSpace(req.DisclosureID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "disclosure id is required", nil)
	}
	return s.configRepo.GetByScope(ctx, ScopeTypeDisclosure, req.DisclosureID)
}

func (s *service) GetWorkflowStepReminderConfig(ctx context.Context, req GetReminderConfigRequest) (*ReminderConfigDTO, error) {
	if strings.TrimSpace(req.WorkflowStepID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "workflow step id is required", nil)
	}
	if strings.TrimSpace(req.DisclosureID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "disclosure id is required", nil)
	}
	return s.configRepo.GetByScope(ctx, ScopeTypeWorkflowStep, strings.TrimSpace(req.DisclosureID)+":"+strings.TrimSpace(req.WorkflowStepID))
}

func (s *service) GetReminderHistory(ctx context.Context, req GetReminderHistoryRequest) (*ReminderHistoryPage, error) {
	if strings.TrimSpace(req.DisclosureID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "disclosure id is required", nil)
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	query := HistoryQuery{
		Scope:    req.Scope,
		Status:   req.Status,
		Page:     page,
		PageSize: pageSize,
	}
	if !req.From.IsZero() {
		from := req.From
		query.From = &from
	}
	if !req.To.IsZero() {
		to := req.To
		query.To = &to
	}
	return s.occurrenceRepo.ListHistoryByDisclosure(ctx, req.DisclosureID, query)
}

func (s *service) DispatchOccurrence(ctx context.Context, req DispatchOccurrenceRequest) (*DispatchOccurrenceResponse, error) {
	if strings.TrimSpace(req.OccurrenceID) == "" || strings.TrimSpace(req.IdempotencyKey) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "occurrence_id and idempotency_key are required", nil)
	}
	if len(req.RecipientEmails) == 0 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "recipient_emails is required", nil)
	}

	occurrence, err := s.occurrenceRepo.ClaimForDispatch(ctx, req.OccurrenceID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.IdempotencyKey) != strings.TrimSpace(occurrence.IdempotencyKey) {
		return nil, perr.NewHTTPError(http.StatusConflict, perr.CodeStateConflict, "idempotency_key mismatch", nil)
	}

	resp, err := s.dispatchClaimedOccurrence(ctx, occurrence, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *service) DispatchDueOccurrences(ctx context.Context, now time.Time, limit int) (*DispatchDueResult, error) {
	start := time.Now()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 {
		limit = 50
	}
	candidates, err := s.occurrenceRepo.ListDispatchCandidates(ctx, now.UTC(), limit)
	if err != nil {
		return nil, err
	}
	result := &DispatchDueResult{}
	for _, c := range candidates {
		result.Processed++
		if !c.ScheduledAt.IsZero() && now.UTC().Sub(c.ScheduledAt.UTC()) > 10*time.Minute {
			s.metrics.IncCounter("reminder_sla_breach_total", map[string]string{"window": "10m"})
			s.alertHook.Notify(ctx, "REMINDER_SLA_BREACH", map[string]string{
				"window":       "10m",
				"occurrence_id": c.OccurrenceID,
			}, map[string]any{
				"scheduled_at": c.ScheduledAt.UTC().Format(time.RFC3339),
				"now":          now.UTC().Format(time.RFC3339),
			})
		}
		resp, dispatchErr := s.DispatchOccurrence(ctx, DispatchOccurrenceRequest{
			OccurrenceID:   c.OccurrenceID,
			IdempotencyKey: c.IdempotencyKey,
			TemplateCode:   c.TemplateCode,
			TemplatePayload: c.TemplatePayload,
			RecipientEmails: c.RecipientEmails,
		})
		if dispatchErr != nil {
			result.Failed++
			continue
		}
		switch resp.Status {
		case ReminderStatusSent:
			result.Sent++
		case ReminderStatusRetryScheduled:
			result.Retried++
		case ReminderStatusFailed:
			result.Failed++
		}
	}
	s.metrics.ObserveLatency("reminder_dispatch_due_ms", time.Since(start).Milliseconds(), map[string]string{"status": "done"})
	return result, nil
}

func (s *service) dispatchClaimedOccurrence(ctx context.Context, occurrence *ReminderOccurrenceDTO, req DispatchOccurrenceRequest) (*DispatchOccurrenceResponse, error) {
	now := time.Now().UTC()
	attemptNo := occurrence.AttemptCount + 1
	templateCode := strings.TrimSpace(req.TemplateCode)
	if templateCode == "" {
		return s.failPermanent(ctx, req.OccurrenceID, attemptNo, "EMAIL_PROVIDER_PERMANENT_ERROR", "template_code is required")
	}
	validEmails := normalizeEmails(req.RecipientEmails)
	if len(validEmails) == 0 {
		return s.failPermanent(ctx, req.OccurrenceID, attemptNo, "REMINDER_RECIPIENT_REQUIRED", "no valid recipient email")
	}

	providerMessageID := "mock-accepted"
	if s.emailSender != nil {
		msgID, sendErr := s.emailSender.SendReminderEmail(ctx, templateCode, req.TemplatePayload, validEmails, req.IdempotencyKey)
		if sendErr != nil {
			if isRetryableError(sendErr) && attemptNo < 5 {
				return s.failRetryable(ctx, req.OccurrenceID, attemptNo, sendErr)
			}
			return s.failPermanent(ctx, req.OccurrenceID, attemptNo, "EMAIL_PROVIDER_PERMANENT_ERROR", sendErr.Error())
		}
		providerMessageID = msgID
	}
	if err := s.attemptRepo.InsertAttempt(ctx, ReminderDeliveryAttemptDTO{
		OccurrenceID: req.OccurrenceID,
		AttemptNo:    attemptNo,
		Status:       ReminderStatusSent,
		ProviderMessageID: providerMessageID,
		CreatedAt:    now,
	}); err != nil {
		return nil, fmt.Errorf("insert sent attempt: %w", err)
	}
	if err := s.occurrenceRepo.UpdateDispatchResult(ctx, DispatchResultInput{
		OccurrenceID:     req.OccurrenceID,
		Status:           ReminderStatusSent,
		ProviderMessageID: providerMessageID,
		IncrementAttempt: true,
	}); err != nil {
		return nil, fmt.Errorf("update sent occurrence: %w", err)
	}
	s.metrics.IncCounter("reminder_dispatch_attempt_total", map[string]string{"status": string(ReminderStatusSent)})
	s.auditor.Record(ctx, "REMINDER_DISPATCH_SENT", "reminder_occurrence", req.OccurrenceID, map[string]any{
		"attempt_no": attemptNo,
	})

	return &DispatchOccurrenceResponse{
		Accepted: true,
		Status:   ReminderStatusSent,
		Message:  "dispatch accepted",
	}, nil
}

func (s *service) SeedOccurrence(ctx context.Context, req SeedOccurrenceRequest) (*ReminderOccurrenceDTO, error) {
	if strings.TrimSpace(req.DisclosureID) == "" {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "disclosure_id is required", nil)
	}
	scopeType := req.ScopeType
	if scopeType == "" {
		scopeType = ScopeTypeDisclosure
	}
	scopeID := strings.TrimSpace(req.ScopeID)
	if scopeID == "" {
		scopeID = req.DisclosureID
	}
	scheduledAt := req.ScheduledAt
	if scheduledAt.IsZero() {
		scheduledAt = time.Now().UTC()
	}
	status := req.Status
	if status == "" {
		status = ReminderStatusPending
	}
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(req.DisclosureID) + "-" + scheduledAt.UTC().Format(time.RFC3339Nano)
	}

	in := ReminderOccurrenceDTO{
		OccurrenceID:   idempotencyKey,
		DisclosureID:   req.DisclosureID,
		ScopeType:      scopeType,
		ScopeID:        scopeID,
		ScheduledAt:    scheduledAt.UTC(),
		Status:         status,
		AttemptCount:   0,
		IdempotencyKey: idempotencyKey,
	}
	return s.occurrenceRepo.SeedOccurrence(ctx, in)
}

func (s *service) MaterializeDueOccurrences(ctx context.Context, now time.Time) (int, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	inserted, err := s.occurrenceRepo.MaterializeDueOccurrences(ctx, now.UTC())
	if err == nil && inserted > 0 {
		s.metrics.IncCounter("reminder_occurrence_created_total", map[string]string{"source": "scheduler"})
	}
	return inserted, err
}

func (s *service) failRetryable(ctx context.Context, occurrenceID string, attemptNo int, cause error) (*DispatchOccurrenceResponse, error) {
	nextRetry := retryAtForAttempt(time.Now().UTC(), attemptNo)
	errMsg := cause.Error()
	if err := s.attemptRepo.InsertAttempt(ctx, ReminderDeliveryAttemptDTO{
		OccurrenceID: occurrenceID,
		AttemptNo:    attemptNo,
		Status:       ReminderStatusRetryScheduled,
		ErrorCode:    "EMAIL_PROVIDER_TEMPORARY_ERROR",
		ErrorMessage: errMsg,
		CreatedAt:    time.Now().UTC(),
	}); err != nil {
		return nil, fmt.Errorf("insert retry attempt: %w", err)
	}
	if err := s.occurrenceRepo.UpdateDispatchResult(ctx, DispatchResultInput{
		OccurrenceID:     occurrenceID,
		Status:           ReminderStatusRetryScheduled,
		NextRetryAt:      &nextRetry,
		LastErrorCode:    "EMAIL_PROVIDER_TEMPORARY_ERROR",
		LastErrorMessage: errMsg,
		IncrementAttempt: true,
	}); err != nil {
		return nil, fmt.Errorf("update retry occurrence: %w", err)
	}
	s.metrics.IncCounter("reminder_dispatch_attempt_total", map[string]string{"status": string(ReminderStatusRetryScheduled)})
	if attemptNo >= 3 {
		s.metrics.IncCounter("reminder_retry_alert_threshold_total", map[string]string{"threshold": "3"})
		s.alertHook.Notify(ctx, "REMINDER_RETRY_THRESHOLD", map[string]string{
			"threshold":    "3",
			"occurrence_id": occurrenceID,
		}, map[string]any{
			"attempt_no": attemptNo,
		})
	}
	s.auditor.Record(ctx, "REMINDER_DISPATCH_RETRY_SCHEDULED", "reminder_occurrence", occurrenceID, map[string]any{
		"attempt_no": attemptNo,
		"next_retry": nextRetry.Format(time.RFC3339),
	})
	msg := "retry scheduled"
	if attemptNo >= 3 {
		msg = "retry scheduled; operations alert threshold reached"
	}
	return &DispatchOccurrenceResponse{
		Accepted: false,
		Status:   ReminderStatusRetryScheduled,
		Message:  msg,
	}, nil
}

func (s *service) failPermanent(ctx context.Context, occurrenceID string, attemptNo int, code, message string) (*DispatchOccurrenceResponse, error) {
	if err := s.attemptRepo.InsertAttempt(ctx, ReminderDeliveryAttemptDTO{
		OccurrenceID: occurrenceID,
		AttemptNo:    attemptNo,
		Status:       ReminderStatusFailed,
		ErrorCode:    code,
		ErrorMessage: message,
		CreatedAt:    time.Now().UTC(),
	}); err != nil {
		return nil, fmt.Errorf("insert failed attempt: %w", err)
	}
	if err := s.occurrenceRepo.UpdateDispatchResult(ctx, DispatchResultInput{
		OccurrenceID:     occurrenceID,
		Status:           ReminderStatusFailed,
		LastErrorCode:    code,
		LastErrorMessage: message,
		IncrementAttempt: true,
	}); err != nil {
		return nil, fmt.Errorf("update failed occurrence: %w", err)
	}
	s.metrics.IncCounter("reminder_dispatch_attempt_total", map[string]string{"status": string(ReminderStatusFailed)})
	s.auditor.Record(ctx, "REMINDER_DISPATCH_FAILED", "reminder_occurrence", occurrenceID, map[string]any{
		"attempt_no": attemptNo,
		"error_code": code,
	})
	return &DispatchOccurrenceResponse{
		Accepted: false,
		Status:   ReminderStatusFailed,
		Message:  message,
	}, nil
}

type retryable interface {
	Temporary() bool
}

func isRetryableError(err error) bool {
	var r retryable
	if errors.As(err, &r) {
		return r.Temporary()
	}
	if strings.Contains(strings.ToLower(err.Error()), "timeout") ||
		strings.Contains(strings.ToLower(err.Error()), "temporar") ||
		strings.Contains(strings.ToLower(err.Error()), "rate") {
		return true
	}
	return false
}

func retryAtForAttempt(now time.Time, attemptNo int) time.Time {
	switch attemptNo {
	case 1:
		return now.Add(1 * time.Minute)
	case 2:
		return now.Add(3 * time.Minute)
	case 3:
		return now.Add(6 * time.Minute)
	case 4:
		return now.Add(10 * time.Minute)
	default:
		return now.Add(20 * time.Minute)
	}
}

func normalizeEmails(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		e := strings.ToLower(strings.TrimSpace(raw))
		if e == "" || !strings.Contains(e, "@") {
			continue
		}
		if _, ok := seen[e]; ok {
			continue
		}
		seen[e] = struct{}{}
		out = append(out, e)
	}
	return out
}

func validateReminderConfig(cfg ReminderConfigInput) error {
	if cfg.Mode != ReminderModeDaysBefore && cfg.Mode != ReminderModeSpecificDate {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid reminder mode", nil)
	}
	if cfg.Mode == ReminderModeDaysBefore && len(cfg.DaysBefore) == 0 {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "daysBefore is required for DaysBefore mode", nil)
	}
	if cfg.Mode == ReminderModeSpecificDate && len(cfg.SpecificDates) == 0 {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "specificDates is required for SpecificDate mode", nil)
	}
	switch cfg.RecipientType {
	case ReminderRecipientTypeDepartments:
		if len(cfg.Departments) == 0 {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "departments is required", nil)
		}
	case ReminderRecipientTypeIndividuals:
		if len(cfg.Recipients) == 0 {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "recipients is required", nil)
		}
	case ReminderRecipientTypeBoth:
		if len(cfg.Departments) == 0 || len(cfg.Recipients) == 0 {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "departments and recipients are required", nil)
		}
	default:
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid recipientType", nil)
	}
	return nil
}
