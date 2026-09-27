package app

import (
	"context"
	"testing"
	"time"

	"github.com/cobo/cobo_iam_services/internal/workflowdept"
)

type countingReminderSender struct{ calls int }

func (c *countingReminderSender) SendReminderEmail(context.Context, string, map[string]any, []string, string) (string, error) {
	c.calls++
	return "legacy-id", nil
}

type ownLookup struct {
	owns  bool
	calls int
}

func (o *ownLookup) BindingOwnsOccurrence(context.Context, string) (bool, error) {
	o.calls++
	return o.owns, nil
}

func TestBindingOwnershipDoesNotDoubleSend(t *testing.T) {
	candidate := DispatchCandidate{
		OccurrenceID:    "occ-own",
		IdempotencyKey:  "idem-own",
		TemplateCode:    "REMINDER_DISCLOSURE_DUE",
		TemplatePayload: map[string]any{"title": "T", "deadline_date": "2026-06-01", "disclosure_id": "d1"},
		RecipientEmails: []string{"a@example.com"},
	}
	t.Run("flag off keeps legacy sender", func(t *testing.T) {
		t.Setenv(workflowdept.EnvBindingEnabled, "")
		t.Setenv(workflowdept.EnvEmailBindingEnabled, "")
		sender := &countingReminderSender{}
		lookup := &ownLookup{owns: true}
		svc, _, _ := newDispatchSvc([]DispatchCandidate{candidate}, nil, nil, sender)
		svc.(*service).bindingOwnership = lookup
		res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
		if err != nil {
			t.Fatal(err)
		}
		if res.Sent != 1 || sender.calls != 1 || lookup.calls != 0 {
			t.Fatalf("sent=%d calls=%d lookup=%d", res.Sent, sender.calls, lookup.calls)
		}
	})
	t.Run("flag on skips legacy when binding owns occurrence", func(t *testing.T) {
		t.Setenv(workflowdept.EnvBindingEnabled, "true")
		t.Setenv(workflowdept.EnvEmailBindingEnabled, "true")
		sender := &countingReminderSender{}
		lookup := &ownLookup{owns: true}
		svc, _, _ := newDispatchSvc([]DispatchCandidate{candidate}, nil, nil, sender)
		svc.(*service).bindingOwnership = lookup
		res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
		if err != nil {
			t.Fatal(err)
		}
		if res.Sent != 0 || res.Skipped != 1 || sender.calls != 0 || lookup.calls != 1 {
			t.Fatalf("sent=%d skipped=%d calls=%d lookup=%d", res.Sent, res.Skipped, sender.calls, lookup.calls)
		}
	})
}
