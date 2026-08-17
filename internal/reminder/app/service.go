package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/subscription/entitlement"
)

type service struct {
	configRepo                 ConfigRepository
	occurrenceRepo             OccurrenceRepository
	attemptRepo                AttemptRepository
	milestoneScanner           MilestoneScanner
	emailSender                EmailSender
	metrics                    Metrics
	auditor                    Auditor
	alertHook                  AlertHook
	alertConfigRepo            AlertConfigRepository
	recipientResolver          RecipientResolver
	stepReader                 WorkflowStepReader
	publicWebBaseURL           string
	inAppCreator               InAppNotificationCreator // optional, nil = disabled
	notificationRulesReader    NotificationRulesReader
	notificationRulesEvaluator *NotificationRulesEvaluator
	membershipQuerier          MembershipEmailQuerier
	taskAssigneeReader         WorkflowTaskAssigneeReader
	stepTaskStateReader        WorkflowStepTaskStateReader
	dispatchLogger             *slog.Logger
	tierEnforcement            *entitlement.Checker
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

func WithMilestoneScanner(ms MilestoneScanner) ServiceOption {
	return func(s *service) {
		s.milestoneScanner = ms
	}
}

func WithAlertConfigRepo(repo AlertConfigRepository) ServiceOption {
	return func(s *service) {
		s.alertConfigRepo = repo
	}
}

func WithPublicWebBaseURL(url string) ServiceOption {
	return func(s *service) {
		s.publicWebBaseURL = strings.TrimRight(strings.TrimSpace(url), "/")
	}
}

func WithRecipientResolver(r RecipientResolver) ServiceOption {
	return func(s *service) {
		s.recipientResolver = r
	}
}

func WithStepReader(r WorkflowStepReader) ServiceOption {
	return func(s *service) {
		s.stepReader = r
	}
}

func WithInAppCreator(c InAppNotificationCreator) ServiceOption {
	return func(s *service) {
		s.inAppCreator = c
	}
}

// WithNotificationRulesFoundation wires Sprint 3 reader/evaluator for Batch 2.
func WithNotificationRulesFoundation(reader NotificationRulesReader, evaluator *NotificationRulesEvaluator) ServiceOption {
	return func(s *service) {
		s.notificationRulesReader = reader
		s.notificationRulesEvaluator = evaluator
	}
}

// WithRecipientPolicyDeps wires membership queries used for recipient_policies filtering (Batch 2B).
func WithRecipientPolicyDeps(querier MembershipEmailQuerier, taskReader WorkflowTaskAssigneeReader) ServiceOption {
	return func(s *service) {
		s.membershipQuerier = querier
		s.taskAssigneeReader = taskReader
	}
}

// WithDispatchLogger sets structured logger for dispatch skip events.
func WithDispatchLogger(log *slog.Logger) ServiceOption {
	return func(s *service) {
		s.dispatchLogger = log
	}
}

// WithTierEnforcement wires subscription tier enforcement for runtime dispatch (Batch 5).
func WithTierEnforcement(checker *entitlement.Checker) ServiceOption {
	return func(s *service) {
		s.tierEnforcement = checker
	}
}

func WithWorkflowStepTaskStateReader(r WorkflowStepTaskStateReader) ServiceOption {
	return func(s *service) {
		s.stepTaskStateReader = r
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

// RequeueStaleDispatching is the worker reaper: requeue reminder occurrences stuck in
// DISPATCHING past olderThan (worker crashed between ClaimForDispatch and
// UpdateDispatchResult) back to PENDING so the next tick re-dispatches them. A non-zero
// count is alert-grade (worker instability), surfaced via the reminder_stale_dispatching
// counter and logged by the caller. Errors are returned (never panic) so the worker tick
// logs a warning and continues.
func (s *service) RequeueStaleDispatching(ctx context.Context, olderThan time.Time) (int, error) {
	if olderThan.IsZero() {
		olderThan = time.Now().UTC()
	}
	n, err := s.occurrenceRepo.RequeueStaleDispatching(ctx, olderThan.UTC())
	if err != nil {
		return 0, err
	}
	if n > 0 {
		s.metrics.IncCounter("reminder_stale_dispatching_requeued_total", map[string]string{})
	}
	return n, nil
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
				"window":        "10m",
				"occurrence_id": c.OccurrenceID,
			}, map[string]any{
				"scheduled_at": c.ScheduledAt.UTC().Format(time.RFC3339),
				"now":          now.UTC().Format(time.RFC3339),
			})
		}

		out := s.prepareDispatch(ctx, c, now.UTC())
		if out.evalErr != nil {
			s.handleEvaluatorError(ctx, c, out.evalErr, result)
			continue
		}
		if out.skip {
			s.recordDispatchSkipped(ctx, c, out.skipReason, out.ruleCode)
			result.Skipped++
			continue
		}

		resp, dispatchErr := s.DispatchOccurrence(ctx, DispatchOccurrenceRequest{
			OccurrenceID:    c.OccurrenceID,
			IdempotencyKey:  c.IdempotencyKey,
			TemplateCode:    out.templateCode,
			TemplatePayload: out.payload,
			RecipientEmails: out.recipients,
		})
		if dispatchErr != nil {
			result.Failed++
			continue
		}
		switch resp.Status {
		case ReminderStatusSent:
			result.Sent++
			if s.inAppCreator != nil && out.allowInApp {
				cc := c
				cc.RecipientEmails = out.recipients
				cc.TemplatePayload = out.payload
				go func(candidate DispatchCandidate) {
					_ = s.inAppCreator.CreateForReminderDispatch(context.Background(), candidate)
				}(cc)
			}
		case ReminderStatusRetryScheduled:
			result.Retried++
		case ReminderStatusFailed:
			result.Failed++
		}
	}
	s.metrics.ObserveLatency("reminder_dispatch_due_ms", time.Since(start).Milliseconds(), map[string]string{"status": "done"})
	return result, nil
}

// prepareDispatchOutcome is the internal result of prepareDispatch (Batch 2B).
type prepareDispatchOutcome struct {
	templateCode string
	recipients   []string
	payload      map[string]any
	skip         bool
	skipReason   string
	ruleCode     string
	allowInApp   bool
	evalErr      error
}

// prepareDispatch resolves template, recipients, and payload for a candidate.
// Flag OFF: Sprint 2 path unchanged. Flag ON: Step 0 evaluator + Step 2b policies.
func (s *service) prepareDispatch(ctx context.Context, c DispatchCandidate, asOf time.Time) prepareDispatchOutcome {
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	}
	dec := evaluateDispatchDecision(ctx, s.dispatchDecisionDeps(), dispatchDecisionInputFromCandidate(c, asOf), DispatchDecisionModeRuntime)
	out := prepareDispatchOutcome{
		templateCode: dec.TemplateCode,
		recipients:   dec.Recipients,
		payload:      c.TemplatePayload,
		skip:         dec.Skip,
		skipReason:   dec.SkipReason,
		ruleCode:     dec.RuleCode,
		allowInApp:   dec.AllowInApp,
		evalErr:      dec.EvalErr,
	}
	if out.payload == nil {
		out.payload = map[string]any{}
	}
	if out.templateCode == "" {
		out.templateCode = c.TemplateCode
	}
	if out.evalErr != nil || out.skip {
		return out
	}

	// Step 3: Payload augmentation (unchanged).
	if c.CompanyName != "" {
		out.payload["company_name"] = c.CompanyName
	}
	deadlineAt := dispatchDeadlineAt(c)
	loc := reminderCalculatorLocation()
	if _, ok := out.payload["due_date"]; !ok {
		out.payload["due_date"] = deadlineAt.In(loc).Format("02/01/2006")
	}
	if c.ScopeType == ScopeTypeWorkflowStep && c.ScopeID != "" {
		stepID := extractStepID(c.ScopeID)
		if _, ok := out.payload["step_id"]; !ok {
			out.payload["step_id"] = stepID
		}
		if milestoneType, ok := workflowStepReminderMilestoneType(c.IdempotencyKey); ok {
			if _, exists := out.payload["reminder_milestone_type"]; !exists {
				out.payload["reminder_milestone_type"] = milestoneType
			}
			if offset, ok := ParseDueMinusReminderOffset(milestoneType); ok {
				if _, exists := out.payload["reminder_offset_days"]; !exists {
					out.payload["reminder_offset_days"] = offset
				}
				if _, exists := out.payload["step_due_date"]; !exists {
					due := c.ScheduledAt.UTC().AddDate(0, 0, offset)
					out.payload["step_due_date"] = due.In(loc).Format("02/01/2006")
				}
			}
		}
		var step *WorkflowStepConfig
		if s.stepReader != nil {
			if st, err := s.stepReader.GetStepByID(ctx, stepID); err == nil {
				step = st
			}
		}
		if _, ok := out.payload["step_name"]; !ok {
			stepName := stepID
			if step != nil && step.StageName != "" {
				stepName = step.StageName
			}
			out.payload["step_name"] = stepName
		}
		if _, ok := out.payload["implementation_guide"]; !ok {
			if step != nil && strings.TrimSpace(step.Instructions) != "" {
				out.payload["implementation_guide"] = truncateImplementationGuide(step.Instructions, implementationGuideMaxChars)
			}
		}
	}
	if _, ok := out.payload["disclosure_title"]; !ok {
		if title, ok2 := out.payload["title"]; ok2 {
			out.payload["disclosure_title"] = title
		}
	}
	if _, ok := out.payload["portal_url"]; !ok {
		if actionURL, ok2 := out.payload["action_url"]; ok2 {
			relative := fmt.Sprint(actionURL)
			if s.publicWebBaseURL != "" {
				out.payload["portal_url"] = s.publicWebBaseURL + relative
			} else {
				out.payload["portal_url"] = relative
			}
		}
	}
	if _, ok := out.payload["portal_url"]; !ok && s.publicWebBaseURL != "" {
		switch c.ScopeType {
		case ScopeTypeDisclosure:
			if c.ScopeID != "" {
				out.payload["portal_url"] = s.publicWebBaseURL + "/app/disclosures/" + c.ScopeID
			}
		case ScopeTypeWorkflowStep:
			disclosureID, _ := out.payload["disclosure_id"].(string)
			if disclosureID == "" {
				if idx := strings.LastIndex(c.ScopeID, ":"); idx >= 0 {
					disclosureID = c.ScopeID[:idx]
				}
			}
			if disclosureID != "" {
				out.payload["portal_url"] = s.publicWebBaseURL + "/app/disclosures/" + disclosureID
			}
		}
	}
	if _, ok := out.payload["recipient_name"]; !ok && len(out.recipients) > 0 {
		out.payload["recipient_name"] = out.recipients[0]
	}
	var remaining int
	if rd, ok := out.payload["remaining_days"]; ok {
		remaining, _ = intFromPayload(rd)
	} else {
		remaining = calculateRemainingDays(deadlineAt, asOf)
		out.payload["remaining_days"] = remaining
	}
	if _, ok := out.payload["urgency_status"]; !ok {
		out.payload["urgency_status"] = determineUrgencyStatus(remaining)
	}
	if guide, ok := out.payload["implementation_guide"]; !ok || strings.TrimSpace(fmt.Sprint(guide)) == "" {
		out.payload["implementation_guide"] = defaultImplementationGuide
	}

	return out
}

func (s *service) dispatchDecisionDeps() DispatchDecisionDeps {
	var simulateEval *NotificationRulesEvaluator
	if s.notificationRulesReader != nil {
		simulateEval = NewNotificationRulesEvaluator(s.notificationRulesReader, true)
	}
	return DispatchDecisionDeps{
		EvaluatorRuntime:   s.notificationRulesEvaluator,
		EvaluatorSimulate:  simulateEval,
		AlertConfigRepo:    s.alertConfigRepo,
		RecipientResolver:  s.recipientResolver,
		MembershipQuerier:  s.membershipQuerier,
		TaskAssigneeReader: s.taskAssigneeReader,
		StepReader:         s.stepReader,
		TierEnforcement:    s.tierEnforcement,
	}
}

func emailChannelAllowed(dec EvaluateDecision) bool {
	for _, ch := range dec.ActiveChannels {
		if strings.EqualFold(ch, "email") {
			return true
		}
	}
	return false
}

func (s *service) recordDispatchSkipped(ctx context.Context, c DispatchCandidate, reason, ruleCode string) {
	if reason == "" {
		reason = "UNKNOWN"
	}
	s.metrics.IncCounter("reminder_dispatch_skipped_total", map[string]string{"reason": reason})
	log := s.dispatchLogger
	if log == nil {
		log = slog.Default()
	}
	log.InfoContext(ctx, "reminder_dispatch_skipped",
		slog.String("event", "reminder_dispatch_skipped"),
		slog.String("skip_reason", reason),
		slog.String("occurrence_id", c.OccurrenceID),
		slog.String("company_id", c.CompanyID),
		slog.String("rule_code", ruleCode),
		slog.String("channel", "email"),
	)
}

func (s *service) handleEvaluatorError(ctx context.Context, c DispatchCandidate, evalErr error, result *DispatchDueResult) {
	log := s.dispatchLogger
	if log == nil {
		log = slog.Default()
	}
	log.WarnContext(ctx, "reminder_dispatch_evaluator_error",
		slog.String("event", "reminder_dispatch_evaluator_error"),
		slog.String("skip_reason", SkipReasonEvaluatorUnavailable),
		slog.String("occurrence_id", c.OccurrenceID),
		slog.String("company_id", c.CompanyID),
		slog.String("err", evalErr.Error()),
	)
	s.metrics.IncCounter("reminder_dispatch_skipped_total", map[string]string{"reason": SkipReasonEvaluatorUnavailable})

	occurrence, err := s.occurrenceRepo.ClaimForDispatch(ctx, c.OccurrenceID)
	if err != nil {
		result.Failed++
		return
	}
	if strings.TrimSpace(c.IdempotencyKey) != strings.TrimSpace(occurrence.IdempotencyKey) {
		result.Failed++
		return
	}
	resp, retryErr := s.failRetryable(ctx, c.OccurrenceID, occurrence.AttemptCount+1, evalErr)
	if retryErr != nil {
		result.Failed++
		return
	}
	if resp.Status == ReminderStatusRetryScheduled {
		result.Retried++
	} else {
		result.Failed++
	}
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
		OccurrenceID:      req.OccurrenceID,
		AttemptNo:         attemptNo,
		Status:            ReminderStatusSent,
		ProviderMessageID: providerMessageID,
		CreatedAt:         now,
	}); err != nil {
		return nil, fmt.Errorf("insert sent attempt: %w", err)
	}
	if err := s.occurrenceRepo.UpdateDispatchResult(ctx, DispatchResultInput{
		OccurrenceID:      req.OccurrenceID,
		Status:            ReminderStatusSent,
		ProviderMessageID: providerMessageID,
		IncrementAttempt:  true,
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
			"threshold":     "3",
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

// ─── Phase 3: reminder content enrichment helpers ────────────────────────────

const (
	// implementationGuideMaxChars caps the "Hướng dẫn thực hiện" block so emails stay compact.
	implementationGuideMaxChars = 500
	// defaultImplementationGuide is the non-empty fallback when no step instructions exist
	// (templates declare implementation_guide as required, so it must never render empty).
	defaultImplementationGuide = "Vui lòng truy cập hệ thống để xem chi tiết và hoàn thành công việc đúng hạn."
)

// reminderCalculatorLocation returns Asia/Ho_Chi_Minh, falling back to a fixed +07:00 zone
// when tzdata is unavailable — matching the deadline engine's timezone handling.
// dispatchDeadlineAt returns the disclosure/workflow due instant for email rendering.
// Falls back to ScheduledAt when the repository did not resolve a planned_date.
func dispatchDeadlineAt(c DispatchCandidate) time.Time {
	if !c.DeadlineAt.IsZero() {
		return c.DeadlineAt
	}
	return c.ScheduledAt
}

func intFromPayload(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	default:
		return 0, false
	}
}

func reminderCalculatorLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	}
	return loc
}

// calculateRemainingDays returns the whole-day difference between the due instant and now,
// floored to midnight in Asia/Ho_Chi_Minh (calendar-day semantics, consistent with the
// deadline UI's remainingDaysFromDue). Negative = overdue, 0 = due today, positive = upcoming.
func calculateRemainingDays(dueDate, now time.Time) int {
	if dueDate.IsZero() {
		return 0
	}
	loc := reminderCalculatorLocation()
	d := dueDate.In(loc)
	n := now.In(loc)
	d = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
	n = time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc)
	return int(d.Sub(n).Hours() / 24)
}

// determineUrgencyStatus maps remaining days to the Vietnamese urgency phrase embedded by
// the reminder email subject/body. Values are self-contained (templates insert them directly).
func determineUrgencyStatus(remainingDays int) string {
	switch {
	case remainingDays < 0:
		return "Quá hạn"
	case remainingDays == 0:
		return "Đã đến hạn"
	default:
		return "Sắp đến hạn"
	}
}

// extractStepID returns the bare step id from a scope_id that may be "disclosureID:stepID"
// (config path) or just "stepID" (milestone path).
func extractStepID(scopeID string) string {
	if idx := strings.LastIndex(scopeID, ":"); idx >= 0 {
		return scopeID[idx+1:]
	}
	return scopeID
}

// truncateImplementationGuide trims and caps the guide to maxChars runes (UTF-8 safe for
// Vietnamese), appending an ellipsis when truncated. maxChars <= 0 disables capping.
func truncateImplementationGuide(guide string, maxChars int) string {
	guide = strings.TrimSpace(guide)
	if maxChars <= 0 {
		return guide
	}
	runes := []rune(guide)
	if len(runes) <= maxChars {
		return guide
	}
	return strings.TrimSpace(string(runes[:maxChars])) + "..."
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

// SeedOccurrencesFromDueMilestones bridges workflow_step_milestones → reminder_occurrences.
// It is idempotent: each milestone row carries a unique milestone_id used as the idempotency_key,
// and reminder_occurrences has a unique index on idempotency_key.
func (s *service) SeedOccurrencesFromDueMilestones(ctx context.Context, now time.Time) (int, error) {
	if s.milestoneScanner == nil {
		return 0, nil // feature not wired; skip silently
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	const batchLimit = 200
	milestones, err := s.milestoneScanner.ListDueMilestones(ctx, now, batchLimit)
	if err != nil {
		return 0, fmt.Errorf("list due milestones: %w", err)
	}
	seeded := 0
	for _, m := range milestones {
		if !IsDueMinusReminderMilestone(m.MilestoneType) {
			continue
		}
		if !s.workflowStepReminderEligible(ctx, m) {
			s.metrics.IncCounter("reminder_milestone_ineligible_total", map[string]string{"milestone_type": m.MilestoneType})
			continue
		}
		idempotencyKey := WorkflowStepReminderIdempotencyKey(m.WorkflowInstanceID, m.StepID, m.MilestoneType)
		occ, err := s.occurrenceRepo.SeedOccurrence(ctx, ReminderOccurrenceDTO{
			OccurrenceID:   m.MilestoneID,
			DisclosureID:   m.WorkflowInstanceID, // use instance as disclosure scope
			ScopeType:      ScopeTypeWorkflowStep,
			ScopeID:        m.StepID,
			ScheduledAt:    m.ScheduledDate.UTC(),
			Status:         ReminderStatusPending,
			IdempotencyKey: idempotencyKey,
		})
		if err != nil {
			// duplicate idempotency_key = already seeded; not an error
			s.metrics.IncCounter("reminder_milestone_seed_error_total", map[string]string{"milestone_type": m.MilestoneType})
			continue
		}
		if err := s.milestoneScanner.MarkMilestoneSent(ctx, m.MilestoneID, occ.OccurrenceID); err != nil {
			s.metrics.IncCounter("reminder_milestone_mark_error_total", map[string]string{"milestone_type": m.MilestoneType})
			continue
		}
		s.metrics.IncCounter("reminder_milestone_seeded_total", map[string]string{"milestone_type": m.MilestoneType})
		seeded++
	}
	return seeded, nil
}
