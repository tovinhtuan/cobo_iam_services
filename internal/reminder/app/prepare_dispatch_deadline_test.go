package app

import (
	"context"
	"testing"
	"time"
)

func dispatchPayload(t *testing.T, c DispatchCandidate) map[string]any {
	t.Helper()
	cs := &payloadCaptureSender{}
	svc := newDispatchSvcWithURL("https://portal.cobo.vn", []DispatchCandidate{c}, cs)
	res, err := svc.DispatchDueOccurrences(context.Background(), time.Now().UTC(), 10)
	if err != nil {
		t.Fatalf("DispatchDueOccurrences error: %v", err)
	}
	if res.Sent != 1 {
		t.Fatalf("expected sent=1, got %+v", res)
	}
	return cs.payload
}

func hcmNoon(year int, month time.Month, day int) time.Time {
	loc := reminderCalculatorLocation()
	return time.Date(year, month, day, 12, 0, 0, 0, loc)
}

// Test 1 — reminder fires today but deadline is 6 days out.
func TestPrepareDispatch_DeadlineAtSixDaysAfterScheduled(t *testing.T) {
	loc := reminderCalculatorLocation()
	now := time.Now().UTC()
	scheduledAt := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 8, 0, 0, 0, loc)
	deadlineAt := scheduledAt.AddDate(0, 0, 6)

	payload := dispatchPayload(t, DispatchCandidate{
		OccurrenceID:    "occ-deadline-6d",
		IdempotencyKey:  "idem-deadline-6d",
		TemplateCode:    "reminder.deadline_approaching",
		TemplatePayload: map[string]any{"title": "Báo cáo Q2", "disclosure_id": "d1"},
		RecipientEmails: []string{"a@example.com"},
		ScheduledAt:     scheduledAt,
		DeadlineAt:      deadlineAt,
		ScopeType:       ScopeTypeDisclosure,
		ScopeID:         "d1",
	})

	wantDue := deadlineAt.In(loc).Format("02/01/2006")
	if got, _ := payload["due_date"].(string); got != wantDue {
		t.Fatalf("due_date = %q, want %q", got, wantDue)
	}
	if got, ok := payload["remaining_days"].(int); !ok || got != 6 {
		t.Fatalf("remaining_days = %v (%T), want 6", payload["remaining_days"], payload["remaining_days"])
	}
	if got, _ := payload["urgency_status"].(string); got != "Sắp đến hạn" {
		t.Fatalf("urgency_status = %q, want Sắp đến hạn", got)
	}
}

// Test 2 — deadline is today → 0 days remaining, đã đến hạn.
func TestPrepareDispatch_DeadlineAtToday(t *testing.T) {
	loc := reminderCalculatorLocation()
	now := time.Now().UTC()
	today := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 15, 0, 0, 0, loc)
	scheduledAt := today.Add(-2 * time.Hour)

	payload := dispatchPayload(t, DispatchCandidate{
		OccurrenceID:    "occ-deadline-today",
		IdempotencyKey:  "idem-deadline-today",
		TemplateCode:    "reminder.deadline_approaching",
		TemplatePayload: map[string]any{"title": "Báo cáo Q2", "disclosure_id": "d2"},
		RecipientEmails: []string{"a@example.com"},
		ScheduledAt:     scheduledAt,
		DeadlineAt:      today,
		ScopeType:       ScopeTypeDisclosure,
		ScopeID:         "d2",
	})

	wantDue := today.In(loc).Format("02/01/2006")
	if got, _ := payload["due_date"].(string); got != wantDue {
		t.Fatalf("due_date = %q, want %q", got, wantDue)
	}
	if got, ok := payload["remaining_days"].(int); !ok || got != 0 {
		t.Fatalf("remaining_days = %v, want 0", payload["remaining_days"])
	}
	if got, _ := payload["urgency_status"].(string); got != "Đã đến hạn" {
		t.Fatalf("urgency_status = %q, want Đã đến hạn", got)
	}
}

// Test 3 — DeadlineAt zero falls back to ScheduledAt (legacy behavior).
func TestPrepareDispatch_DeadlineAtZeroFallsBackToScheduledAt(t *testing.T) {
	loc := reminderCalculatorLocation()
	now := time.Now().UTC()
	scheduledAt := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 10, 0, 0, 0, loc)

	payload := dispatchPayload(t, DispatchCandidate{
		OccurrenceID:    "occ-fallback-sched",
		IdempotencyKey:  "idem-fallback-sched",
		TemplateCode:    "reminder.deadline_approaching",
		TemplatePayload: map[string]any{"title": "Legacy", "disclosure_id": "d3"},
		RecipientEmails: []string{"a@example.com"},
		ScheduledAt:     scheduledAt,
		DeadlineAt:      time.Time{},
		ScopeType:       ScopeTypeDisclosure,
		ScopeID:         "d3",
	})

	wantDue := scheduledAt.In(loc).Format("02/01/2006")
	if got, _ := payload["due_date"].(string); got != wantDue {
		t.Fatalf("due_date = %q, want %q (ScheduledAt fallback)", got, wantDue)
	}
	if got, ok := payload["remaining_days"].(int); !ok || got != 0 {
		t.Fatalf("remaining_days = %v, want 0 (same-day ScheduledAt)", payload["remaining_days"])
	}
}

// Test 4 — pre-populated payload fields are not overwritten (additive contract).
func TestPrepareDispatch_ExistingPayloadFieldsNotOverwritten(t *testing.T) {
	deadlineAt := hcmNoon(2026, 12, 31)
	scheduledAt := hcmNoon(2026, 6, 19)

	payload := dispatchPayload(t, DispatchCandidate{
		OccurrenceID:   "occ-payload-override",
		IdempotencyKey: "idem-payload-override",
		TemplateCode:   "reminder.deadline_approaching",
		TemplatePayload: map[string]any{
			"title":           "Override",
			"disclosure_id":   "d4",
			"due_date":        "01/01/2099",
			"remaining_days":  99,
			"urgency_status":  "Quá hạn",
		},
		RecipientEmails: []string{"a@example.com"},
		ScheduledAt:     scheduledAt,
		DeadlineAt:      deadlineAt,
		ScopeType:       ScopeTypeDisclosure,
		ScopeID:         "d4",
	})

	if got, _ := payload["due_date"].(string); got != "01/01/2099" {
		t.Fatalf("due_date = %q, want preserved 01/01/2099", got)
	}
	if got, ok := payload["remaining_days"].(int); !ok || got != 99 {
		t.Fatalf("remaining_days = %v, want preserved 99", payload["remaining_days"])
	}
	if got, _ := payload["urgency_status"].(string); got != "Quá hạn" {
		t.Fatalf("urgency_status = %q, want preserved Quá hạn", got)
	}
}

func TestDispatchDeadlineAt(t *testing.T) {
	sched := hcmNoon(2026, 6, 19)
	deadline := hcmNoon(2026, 6, 25)
	if got := dispatchDeadlineAt(DispatchCandidate{DeadlineAt: deadline, ScheduledAt: sched}); !got.Equal(deadline) {
		t.Fatalf("want DeadlineAt, got %v", got)
	}
	if got := dispatchDeadlineAt(DispatchCandidate{ScheduledAt: sched}); !got.Equal(sched) {
		t.Fatalf("zero DeadlineAt should fall back to ScheduledAt, got %v", got)
	}
}
