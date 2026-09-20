package inapp

import (
	"context"
	"testing"

	inappapp "github.com/cobo/cobo_iam_services/internal/inappnotification/app"
	reminderapp "github.com/cobo/cobo_iam_services/internal/reminder/app"
)

type fakeInApp struct {
	reqs []inappapp.ReminderInAppRequest
}

func (f *fakeInApp) CreateForReminder(_ context.Context, req inappapp.ReminderInAppRequest) error {
	f.reqs = append(f.reqs, req)
	return nil
}

func (f *fakeInApp) CreateForUser(context.Context, string, string, string, string, string, *string, *string) error {
	return nil
}
func (f *fakeInApp) List(context.Context, string, string) ([]inappapp.InAppNotification, error) {
	return nil, nil
}
func (f *fakeInApp) UnreadCount(context.Context, string, string) (int, error) { return 0, nil }
func (f *fakeInApp) MarkRead(context.Context, string, string) error           { return nil }
func (f *fakeInApp) MarkAllRead(context.Context, string, string) error        { return nil }

func TestBridge_WorkflowStepUsesDisclosureTitleAndStepBody(t *testing.T) {
	fake := &fakeInApp{}
	b := &Bridge{Svc: fake}
	err := b.CreateForReminderDispatch(context.Background(), reminderapp.DispatchCandidate{
		CompanyID: "c1",
		ScopeType: reminderapp.ScopeTypeWorkflowStep,
		ScopeID:   "step-1",
		RecordID:  "52698f3f-53c0-53e7-9e40-d99f90774f41",
		TemplatePayload: map[string]any{
			"disclosure_title": "QA DEF006 Milestones 20260904213636",
			"step_name":        "Xác định nghĩa vụ",
			"due_date":         "17/09/2026",
		},
		RecipientEmails: []string{"a@co.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.reqs) != 1 {
		t.Fatalf("reqs=%d", len(fake.reqs))
	}
	got := fake.reqs[0]
	if got.Kind != inappapp.KindReminderWorkflow {
		t.Fatalf("kind=%s", got.Kind)
	}
	if got.Title != "QA DEF006 Milestones 20260904213636" {
		t.Fatalf("title=%q", got.Title)
	}
	if got.Body != "Bước: Xác định nghĩa vụ · Hạn: 17/09/2026" {
		t.Fatalf("body=%q", got.Body)
	}
	if got.ResourceType != inappapp.ResourceTypeDisclosure {
		t.Fatalf("resource_type=%s", got.ResourceType)
	}
	if got.ResourceID != "52698f3f-53c0-53e7-9e40-d99f90774f41" {
		t.Fatalf("resource_id=%q (must be record, not step)", got.ResourceID)
	}
}

func TestBridge_WorkflowStepResourceIDFromPayloadWhenRecordIDEmpty(t *testing.T) {
	fake := &fakeInApp{}
	b := &Bridge{Svc: fake}
	err := b.CreateForReminderDispatch(context.Background(), reminderapp.DispatchCandidate{
		CompanyID: "c1",
		ScopeType: reminderapp.ScopeTypeWorkflowStep,
		ScopeID:   "step-xyz",
		TemplatePayload: map[string]any{
			"disclosure_title": "Annual report",
			"step_name":        "Soát xét",
			"due_date":         "24/08/2026",
			"record_id":        "rec-from-payload",
		},
		RecipientEmails: []string{"a@co.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := fake.reqs[0]
	if got.ResourceID != "rec-from-payload" {
		t.Fatalf("resource_id=%q", got.ResourceID)
	}
	if got.ResourceID == "step-xyz" {
		t.Fatal("must not use workflow step ScopeID as ResourceID")
	}
}

func TestBridge_WorkflowStepDoesNotUseStepIDAsResource(t *testing.T) {
	fake := &fakeInApp{}
	b := &Bridge{Svc: fake}
	_ = b.CreateForReminderDispatch(context.Background(), reminderapp.DispatchCandidate{
		CompanyID: "c1",
		ScopeType: reminderapp.ScopeTypeWorkflowStep,
		ScopeID:   "step-only",
		TemplatePayload: map[string]any{
			"disclosure_title": "Report A",
			"step_name":        "Step",
		},
		RecipientEmails: []string{"a@co.com"},
	})
	if fake.reqs[0].ResourceID != "" {
		t.Fatalf("expected empty ResourceID when record unknown, got %q", fake.reqs[0].ResourceID)
	}
}

func TestBridge_MissingMetadataDoesNotPanic(t *testing.T) {
	fake := &fakeInApp{}
	b := &Bridge{Svc: fake}
	err := b.CreateForReminderDispatch(context.Background(), reminderapp.DispatchCandidate{
		CompanyID:       "c1",
		ScopeType:       reminderapp.ScopeTypeWorkflowStep,
		ScopeID:         "step-1",
		TemplatePayload: nil,
		RecipientEmails: []string{"a@co.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := fake.reqs[0]
	if got.Title != "Nhắc nhở CBTT" {
		t.Fatalf("title=%q", got.Title)
	}
	if got.Body != "" {
		t.Fatalf("body=%q", got.Body)
	}
}

func TestBridge_LegacyFallbackWhenTitleMissingUsesStep(t *testing.T) {
	fake := &fakeInApp{}
	b := &Bridge{Svc: fake}
	_ = b.CreateForReminderDispatch(context.Background(), reminderapp.DispatchCandidate{
		CompanyID: "c1",
		ScopeType: reminderapp.ScopeTypeWorkflowStep,
		ScopeID:   "step-1",
		RecordID:  "rec-1",
		TemplatePayload: map[string]any{
			"step_name": "Soát xét",
			"due_date":  "24/08/2026",
		},
		RecipientEmails: []string{"a@co.com"},
	})
	got := fake.reqs[0]
	if got.Title != "Bước phê duyệt đến hạn: Soát xét" {
		t.Fatalf("legacy title=%q", got.Title)
	}
	if got.Body != "Bước: Soát xét · Hạn: 24/08/2026" {
		t.Fatalf("body=%q", got.Body)
	}
}

func TestBridge_DisclosureScopeUsesScopeIDAndTitle(t *testing.T) {
	fake := &fakeInApp{}
	b := &Bridge{Svc: fake}
	_ = b.CreateForReminderDispatch(context.Background(), reminderapp.DispatchCandidate{
		CompanyID: "c1",
		ScopeType: reminderapp.ScopeTypeDisclosure,
		ScopeID:   "disc-1",
		TemplatePayload: map[string]any{
			"disclosure_title": "Báo cáo Q2",
			"due_date":         "30/06/2026",
		},
		RecipientEmails: []string{"a@co.com"},
	})
	got := fake.reqs[0]
	if got.Kind != inappapp.KindReminderDeadline {
		t.Fatalf("kind=%s", got.Kind)
	}
	if got.Title != "Báo cáo Q2" {
		t.Fatalf("title=%q", got.Title)
	}
	if got.Body != "Hạn: 30/06/2026" {
		t.Fatalf("body=%q", got.Body)
	}
	if got.ResourceID != "disc-1" {
		t.Fatalf("resource_id=%q", got.ResourceID)
	}
}

func TestBridge_NilBridgeAndNilSvcAreNoops(t *testing.T) {
	var b *Bridge
	if err := b.CreateForReminderDispatch(context.Background(), reminderapp.DispatchCandidate{}); err != nil {
		t.Fatal(err)
	}
	b2 := &Bridge{}
	if err := b2.CreateForReminderDispatch(context.Background(), reminderapp.DispatchCandidate{}); err != nil {
		t.Fatal(err)
	}
}
