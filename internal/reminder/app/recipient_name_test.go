package app

import (
	"context"
	"testing"
	"time"
)

// TestRecipientName_PrePopulatedRecipients_UsesEmail asserts prepareDispatch greets by the
// first recipient email when RecipientEmails is pre-populated. Per spec the greeting is
// "tên user (nếu có) hoặc email"; the resolver yields emails, so recipient_name == email.
func TestRecipientName_PrePopulatedRecipients_UsesEmail(t *testing.T) {
	cs := &payloadCaptureSender{}
	svc := newDispatchSvcWithURL(
		"https://portal.cobo.vn",
		[]DispatchCandidate{{
			OccurrenceID:   "occ-rn1",
			IdempotencyKey: "idem-rn1",
			TemplateCode:   "reminder.deadline_approaching",
			TemplatePayload: map[string]any{
				"disclosure_title": "Báo cáo tài chính",
				"company_name":     "ACME",
			},
			RecipientEmails: []string{"lan.huong@acme.vn", "second@acme.vn"},
			ScopeType:       ScopeTypeDisclosure,
			ScopeID:         "disc-1",
		}},
		cs,
	)

	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if res.Sent != 1 {
		t.Fatalf("expected sent=1, got %+v", res)
	}
	if got, _ := cs.payload["recipient_name"].(string); got != "lan.huong@acme.vn" {
		t.Fatalf("recipient_name = %q, want first recipient email", got)
	}
}

// TestRecipientName_ResolvedRecipients_UsesResolvedEmail asserts that when RecipientEmails is
// empty and recipients are resolved, recipient_name uses the resolved email.
func TestRecipientName_ResolvedRecipients_UsesResolvedEmail(t *testing.T) {
	alertRepo := &fakeAlertConfigRepoForDispatch{rows: []AlertTemplateConfig{
		{TypeID: "dt-rn", AlertKind: AlertKindDeadline, TemplateKey: "reminder.deadline_approaching", Enabled: true},
	}}
	resolver := &fakeResolverForDispatch{emails: []string{"resolved@co.vn"}}
	cs := &payloadCaptureSender{}
	svc, _, _ := newDispatchSvc(
		[]DispatchCandidate{{
			OccurrenceID:     "occ-rn2",
			IdempotencyKey:   "idem-rn2",
			TemplateCode:     "reminder.deadline_approaching",
			TemplatePayload:  map[string]any{"title": "T", "disclosure_id": "d1", "company_name": "Co"},
			RecipientEmails:  nil,
			DisclosureTypeID: "dt-rn",
			ScopeType:        ScopeTypeDisclosure,
			CompanyID:        "c1",
			ScopeID:          "d1",
		}},
		alertRepo, resolver, cs,
	)

	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if res.Sent != 1 {
		t.Fatalf("expected sent=1, got %+v", res)
	}
	if got, _ := cs.payload["recipient_name"].(string); got != "resolved@co.vn" {
		t.Fatalf("recipient_name = %q, want resolved email", got)
	}
}
