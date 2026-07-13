package app_test

import (
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func TestStampCompletedAtIfNeeded_setsOnceOnTerminal(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	rec := &disclosureapp.RecordDTO{Status: "Completed"}
	disclosureapp.StampCompletedAtIfNeeded(rec, "confirm_record", now)
	if rec.CompletedAt == nil || !rec.CompletedAt.Equal(now) {
		t.Fatalf("completed_at=%v", rec.CompletedAt)
	}
	if rec.CompletedSource != "confirm_record" {
		t.Fatalf("source=%q", rec.CompletedSource)
	}
	later := now.Add(time.Hour)
	disclosureapp.StampCompletedAtIfNeeded(rec, "other", later)
	if !rec.CompletedAt.Equal(now) {
		t.Fatalf("must not overwrite completed_at")
	}
	if rec.CompletedSource != "confirm_record" {
		t.Fatalf("must not overwrite source")
	}
}

func TestStampCompletedAtIfNeeded_skipsNonTerminal(t *testing.T) {
	rec := &disclosureapp.RecordDTO{Status: "in_progress"}
	disclosureapp.StampCompletedAtIfNeeded(rec, "confirm_record", time.Now().UTC())
	if rec.CompletedAt != nil {
		t.Fatal("expected nil completed_at")
	}
}

func TestStampCompletedAtIfNeeded_noFakeBackfillOnApproved(t *testing.T) {
	rec := &disclosureapp.RecordDTO{Status: "Approved"}
	disclosureapp.StampCompletedAtIfNeeded(rec, "approve", time.Now().UTC())
	if rec.CompletedAt != nil {
		t.Fatal("Approved is not terminal completion for on_time")
	}
}
