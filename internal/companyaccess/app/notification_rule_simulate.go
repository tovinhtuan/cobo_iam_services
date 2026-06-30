package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// SimulateNotificationRuleRequest is the admin simulate API input.
type SimulateNotificationRuleRequest struct {
	Subject AdminSubject
	Body    SimulateNotificationRuleBody
}

// SimulateNotificationRuleBody is the JSON body for POST .../simulate.
type SimulateNotificationRuleBody struct {
	EventType          string   `json:"event_type"`
	Channel            string   `json:"channel"`
	DueAt              string   `json:"due_at"`
	ScheduledAt        string   `json:"scheduled_at"`
	AsOf               string   `json:"as_of"`
	ScopeType          string   `json:"scope_type"`
	ScopeID            string   `json:"scope_id"`
	DisclosureTypeID   string   `json:"disclosure_type_id"`
	WorkflowInstanceID string   `json:"workflow_instance_id"`
	RecipientEmails    []string `json:"recipient_emails"`
	TemplateCode       string   `json:"template_code"`
}

// SimulateNotificationRuleForbiddenBodyFields must never appear in simulate requests.
var SimulateNotificationRuleForbiddenBodyFields = []string{
	"company_id",
	"occurrence_id",
	"idempotency_key",
	"force_send",
	"bypass_rules",
}

func validateSimulateNotificationRuleBody(body SimulateNotificationRuleBody, rawForbidden map[string]any) error {
	for _, key := range SimulateNotificationRuleForbiddenBodyFields {
		if v, ok := rawForbidden[key]; ok && v != nil {
			if s, isStr := v.(string); isStr && strings.TrimSpace(s) == "" {
				continue
			}
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, key+" is not allowed in simulation request", nil)
		}
	}
	eventType := strings.TrimSpace(strings.ToLower(body.EventType))
	if eventType == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "event_type is required", nil)
	}
	if eventType != "deadline" && eventType != "workflow" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "event_type must be deadline or workflow", nil)
	}
	channel := strings.TrimSpace(strings.ToLower(body.Channel))
	if channel != "" && channel != "email" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "channel must be email in Batch 3B", nil)
	}
	if strings.TrimSpace(body.DueAt) == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "due_at is required", nil)
	}
	if strings.TrimSpace(body.ScheduledAt) == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "scheduled_at is required", nil)
	}
	dueAt, err := time.Parse(time.RFC3339, strings.TrimSpace(body.DueAt))
	if err != nil {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "due_at must be RFC3339", nil)
	}
	scheduledAt, err := time.Parse(time.RFC3339, strings.TrimSpace(body.ScheduledAt))
	if err != nil {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "scheduled_at must be RFC3339", nil)
	}
	if scheduledAt.After(dueAt.Add(365 * 24 * time.Hour)) {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "scheduled_at is too far after due_at", nil)
	}
	scopeType := strings.TrimSpace(strings.ToUpper(body.ScopeType))
	if scopeType == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "scope_type is required", nil)
	}
	if scopeType != "DISCLOSURE" && scopeType != "WORKFLOW_STEP" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "scope_type must be DISCLOSURE or WORKFLOW_STEP", nil)
	}
	if strings.TrimSpace(body.ScopeID) == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "scope_id is required", nil)
	}
	if eventType == "deadline" && scopeType != "DISCLOSURE" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "event_type deadline requires scope_type DISCLOSURE", nil)
	}
	if eventType == "workflow" && scopeType != "WORKFLOW_STEP" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "event_type workflow requires scope_type WORKFLOW_STEP", nil)
	}
	if scopeType == "WORKFLOW_STEP" && strings.TrimSpace(body.WorkflowInstanceID) == "" {
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "workflow_instance_id is required for WORKFLOW_STEP", nil)
	}
	if strings.TrimSpace(body.AsOf) != "" {
		if _, err := time.Parse(time.RFC3339, strings.TrimSpace(body.AsOf)); err != nil {
			return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "as_of must be RFC3339", nil)
		}
	}
	_ = dueAt
	return nil
}

func (s *adminService) SimulateNotificationRule(ctx context.Context, req SimulateNotificationRuleRequest, rawForbidden map[string]any) (*NotificationDispatchSimulateResult, error) {
	if err := s.authorize(ctx, req.Subject, "admin.notification_rule.list", ""); err != nil {
		return nil, err
	}
	if err := validateSimulateNotificationRuleBody(req.Body, rawForbidden); err != nil {
		return nil, err
	}
	if s.dispatchSimulator == nil {
		return nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "simulation is not configured", nil)
	}
	dueAt, _ := time.Parse(time.RFC3339, strings.TrimSpace(req.Body.DueAt))
	scheduledAt, _ := time.Parse(time.RFC3339, strings.TrimSpace(req.Body.ScheduledAt))
	asOf := time.Now().UTC()
	if strings.TrimSpace(req.Body.AsOf) != "" {
		asOf, _ = time.Parse(time.RFC3339, strings.TrimSpace(req.Body.AsOf))
	}
	simID := s.idg.NewUUID()
	return s.dispatchSimulator.SimulateDispatch(ctx, NotificationDispatchSimulateInput{
		CompanyID:          req.Subject.CompanyID,
		EventType:          strings.TrimSpace(strings.ToLower(req.Body.EventType)),
		Channel:            simulateChannel(req.Body),
		DueAt:              dueAt.UTC(),
		ScheduledAt:        scheduledAt.UTC(),
		AsOf:               asOf.UTC(),
		ScopeType:          strings.TrimSpace(strings.ToUpper(req.Body.ScopeType)),
		ScopeID:            strings.TrimSpace(req.Body.ScopeID),
		DisclosureTypeID:   strings.TrimSpace(req.Body.DisclosureTypeID),
		WorkflowInstanceID: strings.TrimSpace(req.Body.WorkflowInstanceID),
		RecipientEmails:    append([]string(nil), req.Body.RecipientEmails...),
		TemplateCode:       strings.TrimSpace(req.Body.TemplateCode),
		SimulationID:       simID,
		SubscriptionTier:   s.subscriptionTier(ctx, req.Subject.UserID),
	})
}

func simulateChannel(body SimulateNotificationRuleBody) string {
	channel := strings.TrimSpace(strings.ToLower(body.Channel))
	if channel == "" {
		return "email"
	}
	return channel
}

