package app

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestBuildReminderTemplatePayload_WorkflowStepUsesRecordCTANotInstance(t *testing.T) {
	instanceID := "01a01307-cc59-751d-a370-19daaa83e5f4"
	recordID := "dca70251-f0cf-53cb-8ad5-77b5f7ba1336"
	payload := BuildReminderTemplatePayload(ReminderEmailBusinessContext{
		OccurrenceDisclosureID: instanceID,
		ScopeType:              ScopeTypeWorkflowStep,
		ScopeID:                "step-001",
		RecordID:               recordID,
		Title:                  "qa E1 regression D direct assignee",
		DeadlineAt:             time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
		ScheduledAt:            time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC),
		CompanyID:              "company-a",
		CompanyName:            "QA Manual QR Pkg 20260814",
		WorkflowInstanceID:     instanceID,
	})
	if got := payload["action_url"]; got != "/app/disclosures/"+recordID {
		t.Fatalf("action_url = %v, want record CTA", got)
	}
	if strings.Contains(fmt.Sprint(payload["action_url"]), instanceID) {
		t.Fatalf("action_url must not contain workflow instance id: %v", payload["action_url"])
	}
	if payload["record_id"] != recordID {
		t.Fatalf("record_id = %v", payload["record_id"])
	}
	if payload["deadline_date"] != "2026-08-19" {
		t.Fatalf("deadline_date = %v, want step EndDate 2026-08-19", payload["deadline_date"])
	}
	if payload["company_name"] != "QA Manual QR Pkg 20260814" {
		t.Fatalf("company_name = %v", payload["company_name"])
	}
}

func TestBuildReminderTemplatePayload_DoesNotUseRecipientCompany(t *testing.T) {
	payload := BuildReminderTemplatePayload(ReminderEmailBusinessContext{
		ScopeType:          ScopeTypeWorkflowStep,
		RecordID:           "rec-a",
		Title:              "Proposal A",
		CompanyID:          "company-a",
		CompanyName:        "Company A Legal",
		WorkflowInstanceID: "wf-a",
		DeadlineAt:         time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
	})
	if payload["company_name"] != "Company A Legal" {
		t.Fatalf("company_name = %v, want business company A", payload["company_name"])
	}
	if payload["company_id"] != "company-a" {
		t.Fatalf("company_id = %v", payload["company_id"])
	}
}

func TestBuildReminderTemplatePayload_MissingRecordIDOmitsInstanceCTA(t *testing.T) {
	instanceID := "wf-instance-id"
	payload := BuildReminderTemplatePayload(ReminderEmailBusinessContext{
		OccurrenceDisclosureID: instanceID,
		ScopeType:              ScopeTypeWorkflowStep,
		WorkflowInstanceID:     instanceID,
		DeadlineAt:             time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
	})
	if _, ok := payload["action_url"]; ok {
		t.Fatalf("workflow step without record_id must not invent CTA, got %v", payload["action_url"])
	}
}

func TestBuildReminderTemplatePayload_TitleIsBusinessTitleNotTemplateID(t *testing.T) {
	payload := BuildReminderTemplatePayload(ReminderEmailBusinessContext{
		ScopeType:  ScopeTypeWorkflowStep,
		RecordID:   "rec-1",
		Title:      "qa E1 regression D direct assignee",
		DeadlineAt: time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
	})
	if payload["title"] != "qa E1 regression D direct assignee" {
		t.Fatalf("title = %v", payload["title"])
	}
}

func TestPrepareDispatch_CompanyStaysOnBusinessRecordWhenRecipientsDiffer(t *testing.T) {
	payload := dispatchPayload(t, DispatchCandidate{
		OccurrenceID:   "occ-cross-co",
		IdempotencyKey: "idem-cross-co",
		TemplateCode:   "reminder.workflow_step_due",
		CompanyID:      "company-a",
		CompanyName:    "Company A Legal",
		RecordID:       "rec-a",
		TemplatePayload: map[string]any{
			"title":     "Proposal A",
			"record_id": "rec-a",
		},
		RecipientEmails:    []string{"multi-company-user@example.com"},
		ScopeType:          ScopeTypeWorkflowStep,
		ScopeID:            "step-001",
		WorkflowInstanceID: "wf-a",
		DeadlineAt:         time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
	})
	if payload["company_name"] != "Company A Legal" {
		t.Fatalf("company_name = %v, must stay on proposal company A", payload["company_name"])
	}
	portal, _ := payload["portal_url"].(string)
	if !strings.Contains(portal, "rec-a") {
		t.Fatalf("portal_url = %q, want record CTA", portal)
	}
	if strings.Contains(portal, "wf-a") {
		t.Fatalf("portal_url leaked workflow instance: %q", portal)
	}
}

func TestPrepareDispatch_WorkflowStepDoesNotCTAToInstanceID(t *testing.T) {
	instanceID := "01aaaaaaaa-instance"
	payload := dispatchPayload(t, DispatchCandidate{
		OccurrenceID:   "occ-cta",
		IdempotencyKey: "idem-cta",
		TemplateCode:   "reminder.workflow_step_due",
		CompanyName:    "Company A Legal",
		RecordID:       "disc-record-1",
		TemplatePayload: map[string]any{
			"disclosure_id": instanceID,
			"disclosure_title": "Annual report",
			"step_name":        "Step 1",
			"due_date":         "19/08/2026",
		},
		RecipientEmails:    []string{"head@example.com"},
		ScopeType:          ScopeTypeWorkflowStep,
		ScopeID:            "step-001",
		WorkflowInstanceID: instanceID,
		DeadlineAt:         time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
	})
	portal, _ := payload["portal_url"].(string)
	if !strings.Contains(portal, "disc-record-1") {
		t.Fatalf("portal_url = %q, want disclosure record id", portal)
	}
	if strings.Contains(portal, instanceID) {
		t.Fatalf("portal_url used instance id: %q", portal)
	}
}

func TestPrepareDispatch_DeadlineUsesDeadlineAtNotScheduledAt(t *testing.T) {
	payload := dispatchPayload(t, DispatchCandidate{
		OccurrenceID:    "occ-dl",
		IdempotencyKey:  "idem-dl",
		TemplateCode:    "reminder.workflow_step_due",
		CompanyName:     "Company A Legal",
		RecordID:        "rec-1",
		TemplatePayload: map[string]any{"title": "T", "record_id": "rec-1"},
		RecipientEmails: []string{"a@example.com"},
		ScopeType:       ScopeTypeWorkflowStep,
		ScheduledAt:     time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC),
		DeadlineAt:      time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
	})
	if payload["due_date"] != "19/08/2026" {
		t.Fatalf("due_date = %v, want step EndDate 19/08/2026", payload["due_date"])
	}
}
