package app_test

import (
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func TestClampDayOfMonth(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	cases := []struct {
		y, d int
		m    time.Month
		want string
	}{
		{2026, 31, time.January, "2026-01-31"},
		{2026, 31, time.February, "2026-02-28"},
		{2028, 31, time.February, "2028-02-29"},
		{2026, 29, time.February, "2026-02-28"},
		{2028, 29, time.February, "2028-02-29"},
		{2026, 31, time.April, "2026-04-30"},
	}
	for _, tc := range cases {
		got := disclosureapp.ClampDayOfMonth(tc.y, tc.m, tc.d, loc)
		if got.Format("2006-01-02") != tc.want {
			t.Fatalf("%v day=%d → %s want %s", tc.m, tc.d, got.Format("2006-01-02"), tc.want)
		}
	}
}

func TestResolveEffectiveAnchor_CompanyWins(t *testing.T) {
	cms := disclosureapp.AnchorConfig{Month: 9, Day: 30}
	co := disclosureapp.AnchorConfig{Month: 10, Day: 5}
	got, src := disclosureapp.ResolveEffectiveAnchor(cms, co)
	if src != disclosureapp.TSourceCompany || got.Month != 10 || got.Day != 5 {
		t.Fatalf("got=%+v src=%s", got, src)
	}
	got2, src2 := disclosureapp.ResolveEffectiveAnchor(cms, disclosureapp.AnchorConfig{})
	if src2 != disclosureapp.TSourceCMS || got2.Month != 9 || got2.Day != 30 {
		t.Fatalf("fallback got=%+v src=%s", got2, src2)
	}
}

func TestResolveOccurrenceT_YearlyInclusiveExamples(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	anchor := disclosureapp.AnchorConfig{Month: 9, Day: 30}
	tEff, err := disclosureapp.ResolveOccurrenceT("yearly", "2026", anchor, loc)
	if err != nil {
		t.Fatal(err)
	}
	if tEff.Format("2006-01-02") != "2026-09-30" {
		t.Fatalf("T=%s", tEff.Format("2006-01-02"))
	}
}

func TestSubmissionCompliance_EndOfBusinessDate(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	due := "2026-10-19"
	// Submit on due day evening HCM → ON_TIME
	sub := time.Date(2026, 10, 19, 22, 0, 0, 0, loc)
	if got := disclosureapp.SubmissionCompliance(due, &sub, sub, loc); got != "SUBMITTED_ON_TIME" {
		t.Fatalf("got %s", got)
	}
	late := time.Date(2026, 10, 20, 0, 30, 0, 0, loc)
	if got := disclosureapp.SubmissionCompliance(due, &late, late, loc); got != "SUBMITTED_LATE" {
		t.Fatalf("got %s", got)
	}
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, loc)
	if got := disclosureapp.SubmissionCompliance(due, nil, now, loc); got != "OVERDUE" {
		t.Fatalf("got %s", got)
	}
}

func TestResolveOpenAt(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	t0 := time.Date(2026, 9, 30, 0, 0, 0, 0, loc)
	if got := disclosureapp.ResolveOpenAt(t0, 0); !got.Equal(t0) {
		t.Fatalf("open=T got %v", got)
	}
	want := time.Date(2026, 9, 20, 0, 0, 0, 0, loc)
	if got := disclosureapp.ResolveOpenAt(t0, 10); !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestResolveLogicalSlot_Yearly(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, loc)
	if got := disclosureapp.ResolveLogicalSlot("yearly", now, loc); got != "2026" {
		t.Fatalf("got %s", got)
	}
}
