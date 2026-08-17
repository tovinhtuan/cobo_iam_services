package inapp

import (
	"context"

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
	kind := inappapp.KindReminderDeadline
	if c.ScopeType == reminderapp.ScopeTypeWorkflowStep {
		kind = inappapp.KindReminderWorkflow
	}
	title := "Nhắc nhở CBTT"
	if v, ok := c.TemplatePayload["disclosure_title"].(string); ok && v != "" {
		if c.ScopeType == reminderapp.ScopeTypeWorkflowStep {
			step := ""
			if s, ok2 := c.TemplatePayload["step_name"].(string); ok2 && s != "" {
				step = s
			}
			if step != "" {
				title = "Bước phê duyệt đến hạn: " + step
			} else {
				title = "Bước phê duyệt đến hạn: " + v
			}
		} else {
			title = "Sắp đến hạn CBTT: " + v
		}
	}
	body := ""
	if v, ok := c.TemplatePayload["due_date"].(string); ok && v != "" {
		body = "Deadline: " + v
	}
	resourceID := ""
	if c.ScopeType == reminderapp.ScopeTypeDisclosure {
		resourceID = c.ScopeID
	}
	return b.Svc.CreateForReminder(ctx, inappapp.ReminderInAppRequest{
		CompanyID:       c.CompanyID,
		Kind:            kind,
		Title:           title,
		Body:            body,
		ResourceType:    inappapp.ResourceTypeDisclosure,
		ResourceID:      resourceID,
		RecipientEmails: c.RecipientEmails,
	})
}
