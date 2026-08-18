package app

import (
	"strings"
	"time"
)

// ReminderEmailBusinessContext is the business-record authority for reminder
// email fields. Recipients must never be used to infer company/title/CTA.
type ReminderEmailBusinessContext struct {
	OccurrenceDisclosureID string
	ScopeType              ScopeType
	ScopeID                string
	RecordID               string
	Title                  string
	DeadlineAt             time.Time
	ScheduledAt            time.Time
	Status                 string
	CompanyID              string
	CompanyName            string
	WorkflowInstanceID     string
}

// ReminderDisclosureCTA returns the portal path for the business disclosure
// record. Empty when the record id is unknown — callers must not substitute
// a workflow instance id or a recipient identifier.
func ReminderDisclosureCTA(recordID string) string {
	id := strings.TrimSpace(recordID)
	if id == "" {
		return ""
	}
	return "/app/disclosures/" + id
}

// BuildReminderTemplatePayload maps business context onto the email payload.
// Company/title/CTA come only from the business record fields on ctx.
func BuildReminderTemplatePayload(ctx ReminderEmailBusinessContext) map[string]any {
	displayTitle := strings.TrimSpace(ctx.Title)
	if displayTitle == "" {
		displayTitle = strings.TrimSpace(ctx.RecordID)
	}
	recordID := strings.TrimSpace(ctx.RecordID)
	if recordID == "" && ctx.ScopeType != ScopeTypeWorkflowStep {
		recordID = strings.TrimSpace(ctx.OccurrenceDisclosureID)
	}
	payload := map[string]any{
		"disclosure_id":         strings.TrimSpace(ctx.OccurrenceDisclosureID),
		"scope_type":            string(ctx.ScopeType),
		"scope_id":              strings.TrimSpace(ctx.ScopeID),
		"title":                 displayTitle,
		"deadline_date":         ctx.DeadlineAt.UTC().Format("2006-01-02"),
		"scheduled_at":          ctx.ScheduledAt.UTC().Format(time.RFC3339),
		"status":                strings.TrimSpace(ctx.Status),
		"company_id":            strings.TrimSpace(ctx.CompanyID),
		"record_id":             recordID,
		"workflow_instance_id":  strings.TrimSpace(ctx.WorkflowInstanceID),
	}
	if name := strings.TrimSpace(ctx.CompanyName); name != "" {
		payload["company_name"] = name
	}
	if cta := ReminderDisclosureCTA(recordID); cta != "" {
		payload["action_url"] = cta
	}
	return payload
}
