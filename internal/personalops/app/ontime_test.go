package app_test

import (
	"testing"
	"time"

	personalopsapp "github.com/cobo/cobo_iam_services/internal/personalops/app"
)

func TestComputeOnTimeRate_exactAndUnavailable(t *testing.T) {
	r := personalopsapp.ComputeOnTimeRate(12, 14)
	if r.Accuracy != "exact" || r.Value == nil || *r.Value != 86 {
		t.Fatalf("got %#v", r)
	}
	if r.SampleSize != 14 || r.CompletedOnTime != 12 || r.CompletedTotal != 14 {
		t.Fatalf("sample fields %#v", r)
	}
	if r.Source == nil || *r.Source != "disclosure_records.completed_at" {
		t.Fatalf("source=%v", r.Source)
	}

	zero := personalopsapp.ComputeOnTimeRate(0, 5)
	if zero.Accuracy != "exact" || zero.Value == nil || *zero.Value != 0 {
		t.Fatalf("0%% exact got %#v", zero)
	}

	full := personalopsapp.ComputeOnTimeRate(3, 3)
	if full.Value == nil || *full.Value != 100 {
		t.Fatalf("100%% got %#v", full)
	}

	empty := personalopsapp.ComputeOnTimeRate(0, 0)
	if empty.Accuracy != "unavailable" || empty.Value != nil || empty.SampleSize != 0 {
		t.Fatalf("empty got %#v", empty)
	}
	if empty.Reason == nil || *empty.Reason != "no_completed_items_with_due_and_outcome" {
		t.Fatalf("reason=%v", empty.Reason)
	}
}

func TestIsOutcomeOnTime(t *testing.T) {
	loc := time.UTC
	outcome := time.Date(2026, 7, 10, 23, 0, 0, 0, time.UTC)
	if !personalopsapp.IsOutcomeOnTime(outcome, "2026-07-10", loc) {
		t.Fatal("same day should be on time")
	}
	if personalopsapp.IsOutcomeOnTime(outcome, "2026-07-09", loc) {
		t.Fatal("after due should be late")
	}
	if !personalopsapp.IsOutcomeOnTime(outcome, "2026-07-11", loc) {
		t.Fatal("before due should be on time")
	}
}
