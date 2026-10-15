package inapp

import (
	"context"
	"strings"

	inappapp "github.com/cobo/cobo_iam_services/internal/inappnotification/app"
	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
)

// Bridge adapts in-app Service to reminder InAppNotificationCreator.
type Bridge struct {
	Svc inappapp.Service
}

func (b *Bridge) CreateForReminderDispatch(ctx context.Context, c reminderapp.DispatchCandidate) error {
	if b == nil || b.Svc == nil {
		return nil
	}
	return b.Svc.CreateForReminder(ctx, b.request(c))
}

type countedReminderCreator interface {
	CreateForReminderCounted(ctx context.Context, req inappapp.ReminderInAppRequest) (int, error)
}

// DeliverForReminderDispatch creates the in-app notifications and reports how many were created
// (reminderapp.InAppNotificationDeliverer). Used when in-app is the only channel of a reminder.
func (b *Bridge) DeliverForReminderDispatch(ctx context.Context, c reminderapp.DispatchCandidate) (int, error) {
	if b == nil || b.Svc == nil {
		return 0, nil
	}
	if counted, ok := b.Svc.(countedReminderCreator); ok {
		return counted.CreateForReminderCounted(ctx, b.request(c))
	}
	// A service without counting cannot prove delivery: report none so the caller falls back.
	return 0, b.Svc.CreateForReminder(ctx, b.request(c))
}

func (b *Bridge) request(c reminderapp.DispatchCandidate) inappapp.ReminderInAppRequest {
	kind := inappapp.KindReminderDeadline
	if c.ScopeType == reminderapp.ScopeTypeWorkflowStep {
		kind = inappapp.KindReminderWorkflow
	}

	disclosureTitle := payloadString(c.TemplatePayload, "disclosure_title")
	if disclosureTitle == "" {
		disclosureTitle = payloadString(c.TemplatePayload, "title")
	}
	stepName := payloadString(c.TemplatePayload, "step_name")
	dueDate := payloadString(c.TemplatePayload, "due_date")
	if dueDate == "" {
		dueDate = payloadString(c.TemplatePayload, "step_due_date")
	}

	// Title must identify the specific alert/report. Step name belongs in body.
	title := "Nhắc nhở CBTT"
	if disclosureTitle != "" {
		title = disclosureTitle
	} else if c.ScopeType == reminderapp.ScopeTypeWorkflowStep && stepName != "" {
		// Legacy fallback when disclosure title is missing from older payloads.
		title = "Bước phê duyệt đến hạn: " + stepName
	}

	body := buildReminderBody(c.ScopeType, stepName, dueDate)
	resourceID := resolveDisclosureResourceID(c)

	return inappapp.ReminderInAppRequest{
		CompanyID:       c.CompanyID,
		Kind:            kind,
		Title:           title,
		Body:            body,
		ResourceType:    inappapp.ResourceTypeDisclosure,
		ResourceID:      resourceID,
		RecipientEmails: c.RecipientEmails,
	}
}

func buildReminderBody(scope reminderapp.ScopeType, stepName, dueDate string) string {
	if scope == reminderapp.ScopeTypeWorkflowStep {
		parts := make([]string, 0, 2)
		if stepName != "" {
			parts = append(parts, "Bước: "+stepName)
		}
		if dueDate != "" {
			parts = append(parts, "Hạn: "+dueDate)
		}
		return strings.Join(parts, " · ")
	}
	if dueDate != "" {
		return "Hạn: " + dueDate
	}
	return ""
}

// resolveDisclosureResourceID returns the disclosure/deadline record id for deep-links.
// Never uses WORKFLOW_STEP ScopeID (that is a step id).
func resolveDisclosureResourceID(c reminderapp.DispatchCandidate) string {
	if id := strings.TrimSpace(c.RecordID); id != "" {
		return id
	}
	if id := payloadString(c.TemplatePayload, "record_id"); id != "" {
		return id
	}
	if c.ScopeType == reminderapp.ScopeTypeDisclosure {
		if id := strings.TrimSpace(c.ScopeID); id != "" {
			return id
		}
	}
	if id := payloadString(c.TemplatePayload, "disclosure_id"); id != "" {
		return id
	}
	return ""
}

func payloadString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	v, ok := payload[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}
