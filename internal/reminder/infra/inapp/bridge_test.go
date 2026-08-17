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

func TestBridge_WorkflowStepMapsKindAndTitle(t *testing.T) {
	fake := &fakeInApp{}
	b := &Bridge{Svc: fake}
	err := b.CreateForReminderDispatch(context.Background(), reminderapp.DispatchCandidate{
		CompanyID: "c1",
		ScopeType: reminderapp.ScopeTypeWorkflowStep,
		ScopeID:   "step-1",
		TemplatePayload: map[string]any{
			"disclosure_title": "QA record",
			"step_name":        "Soát xét",
			"due_date":         "24/08/2026",
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
	if got.Title != "Bước phê duyệt đến hạn: Soát xét" {
		t.Fatalf("title=%s", got.Title)
	}
	if got.Body != "Deadline: 24/08/2026" {
		t.Fatalf("body=%s", got.Body)
	}
}
