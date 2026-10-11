package app

import (
	"context"
	"testing"
	"time"
)

// Contract cms-companies-safety-2026-10-11 D-04: an inactive company gets no reminders (closed as
// FAILED + COMPANY_INACTIVE, never sent later); a suspended company still gets them.

func companyCandidate(id, companyStatus string) DispatchCandidate {
	return DispatchCandidate{
		OccurrenceID:    id,
		IdempotencyKey:  "idem-" + id,
		TemplateCode:    "REMINDER_DISCLOSURE_DUE",
		TemplatePayload: map[string]any{"title": "T", "deadline_date": "2026-06-01", "disclosure_id": "d1"},
		RecipientEmails: []string{"a@example.com"},
		CompanyID:       "c1",
		CompanyStatus:   companyStatus,
	}
}

func TestDispatchDue_InactiveCompanyClosedWithoutSending(t *testing.T) {
	svc, occRepo, _ := newDispatchSvc([]DispatchCandidate{companyCandidate("occ-inactive", "inactive")}, nil, nil, fakeSender{})
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if res.Sent != 0 || res.Skipped != 1 {
		t.Fatalf("result = %+v, want skipped=1 sent=0", res)
	}
	if len(occRepo.updates) != 1 {
		t.Fatalf("updates = %+v, want one terminal update", occRepo.updates)
	}
	u := occRepo.updates[0]
	if u.Status != ReminderStatusFailed || u.LastErrorCode != ReminderSkipCompanyInactive || u.IncrementAttempt {
		t.Fatalf("update = %+v, want FAILED/COMPANY_INACTIVE without attempt", u)
	}
}

func TestDispatchDue_SuspendedCompanyStillReminded(t *testing.T) {
	svc, _, _ := newDispatchSvc([]DispatchCandidate{companyCandidate("occ-suspended", "suspended")}, nil, nil, fakeSender{})
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if res.Sent != 1 || res.Skipped != 0 {
		t.Fatalf("result = %+v, want sent=1", res)
	}
}
