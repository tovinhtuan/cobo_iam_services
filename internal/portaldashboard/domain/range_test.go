package domain

import (
	"testing"
	"time"
)

func TestParseRange_7d(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	dr, err := ParseRange(ParseRangeInput{Range: "7d", Now: now})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dr.Preset != "7d" || dr.From != "2026-07-04" || dr.To != "2026-07-10" {
		t.Fatalf("got %+v", dr)
	}
}

func TestParseRange_30d(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	dr, err := ParseRange(ParseRangeInput{Range: "30d", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if dr.From != "2026-06-11" || dr.To != "2026-07-10" {
		t.Fatalf("got %+v", dr)
	}
}

func TestParseRange_quarter(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	dr, err := ParseRange(ParseRangeInput{Range: "quarter", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if dr.From != "2026-07-01" || dr.To != "2026-07-10" {
		t.Fatalf("got %+v", dr)
	}
}

func TestParseRange_customValid(t *testing.T) {
	dr, err := ParseRange(ParseRangeInput{Range: "custom", From: "2026-01-01", To: "2026-01-31"})
	if err != nil {
		t.Fatal(err)
	}
	if dr.From != "2026-01-01" || dr.To != "2026-01-31" {
		t.Fatalf("got %+v", dr)
	}
}

func TestParseRange_customMissing(t *testing.T) {
	_, err := ParseRange(ParseRangeInput{Range: "custom", From: "2026-01-01"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRange_fromAfterTo(t *testing.T) {
	_, err := ParseRange(ParseRangeInput{Range: "custom", From: "2026-02-01", To: "2026-01-01"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseRange_unsupported(t *testing.T) {
	_, err := ParseRange(ParseRangeInput{Range: "1y"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNext7DaysWindow(t *testing.T) {
	now := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)
	dr, _ := ParseRange(ParseRangeInput{Range: "30d", Now: now})
	start, end := Next7DaysWindow(dr)
	if start != "2026-07-10" || end != "2026-07-17" {
		t.Fatalf("got %s..%s", start, end)
	}
}
