package app_test

import (
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func boolPtr(v bool) *bool { return &v }

func TestResolveEffectiveAnchor_DailyIgnoresCompany(t *testing.T) {
	cms := disclosureapp.AnchorConfig{Month: 3, Day: 10}
	co := disclosureapp.CompanyOverrideAuthority{
		Active: true, Frequency: "daily",
		Anchor: disclosureapp.AnchorConfig{Month: 6, Day: 20},
	}
	got, src := disclosureapp.ResolveEffectiveAnchor("daily", cms, co)
	if src != disclosureapp.TSourceCMS || got.Month != 3 || got.Day != 10 {
		t.Fatalf("daily must ignore company: got=%+v src=%s", got, src)
	}
}

func TestResolveEffectiveAnchor_WeeklyWeekdayAtomic(t *testing.T) {
	cms := disclosureapp.AnchorConfig{Weekday: intPtr(1)} // Mon
	co := disclosureapp.CompanyOverrideAuthority{
		Active: true, Frequency: "weekly",
		Anchor: disclosureapp.AnchorConfig{Weekday: intPtr(5)}, // Fri
	}
	got, src := disclosureapp.ResolveEffectiveAnchor("weekly", cms, co)
	if src != disclosureapp.TSourceCompany || got.Weekday == nil || *got.Weekday != 5 {
		t.Fatalf("weekly company weekday: got=%+v src=%s", got, src)
	}
}

func TestResolveEffectiveAnchor_MonthlyDay(t *testing.T) {
	cms := disclosureapp.AnchorConfig{Day: 31}
	co := disclosureapp.CompanyOverrideAuthority{
		Active: true, Frequency: "monthly",
		Anchor: disclosureapp.AnchorConfig{Day: 20},
	}
	got, src := disclosureapp.ResolveEffectiveAnchor("monthly", cms, co)
	if src != disclosureapp.TSourceCompany || got.Day != 20 {
		t.Fatalf("monthly: got=%+v src=%s", got, src)
	}
}

func TestResolveEffectiveAnchor_QuarterlyAtomic(t *testing.T) {
	cms := disclosureapp.AnchorConfig{MonthInQuarter: intPtr(2), Day: 15}
	co := disclosureapp.CompanyOverrideAuthority{
		Active: true, Frequency: "quarterly",
		Anchor: disclosureapp.AnchorConfig{MonthInQuarter: intPtr(3), Day: 20},
	}
	got, src := disclosureapp.ResolveEffectiveAnchor("quarterly", cms, co)
	if src != disclosureapp.TSourceCompany || got.MonthInQuarter == nil || *got.MonthInQuarter != 3 || got.Day != 20 {
		t.Fatalf("quarterly atomic: got=%+v src=%s", got, src)
	}
	// Must not be CMS MiQ + Company day
	if *got.MonthInQuarter == 2 && got.Day == 20 {
		t.Fatal("partial quarterly merge forbidden")
	}
}

func TestResolveEffectiveAnchor_YearlyAtomic(t *testing.T) {
	cms := disclosureapp.AnchorConfig{Month: 2, Day: 29}
	co := disclosureapp.CompanyOverrideAuthority{
		Active: true, Frequency: "yearly",
		Anchor: disclosureapp.AnchorConfig{Month: 3, Day: 10},
	}
	got, src := disclosureapp.ResolveEffectiveAnchor("yearly", cms, co)
	if src != disclosureapp.TSourceCompany || got.Month != 3 || got.Day != 10 {
		t.Fatalf("yearly atomic: got=%+v src=%s", got, src)
	}
}

func TestResolveEffectiveAnchor_FrequencyMismatchIgnoresCompany(t *testing.T) {
	cms := disclosureapp.AnchorConfig{MonthInQuarter: intPtr(2), Day: 10}
	co := disclosureapp.CompanyOverrideAuthority{
		Active: true, Frequency: "monthly",
		Anchor: disclosureapp.AnchorConfig{Day: 20},
	}
	got, src := disclosureapp.ResolveEffectiveAnchor("quarterly", cms, co)
	if src != disclosureapp.TSourceCMS || got.Day != 10 || got.MonthInQuarter == nil || *got.MonthInQuarter != 2 {
		t.Fatalf("mismatch must use CMS: got=%+v src=%s", got, src)
	}
}

func TestResolveEffectiveAnchor_InactiveDoesNotReactivate(t *testing.T) {
	cms := disclosureapp.AnchorConfig{Day: 15}
	co := disclosureapp.CompanyOverrideAuthority{
		Active: false, Frequency: "monthly",
		Anchor: disclosureapp.AnchorConfig{Day: 20},
	}
	got, src := disclosureapp.ResolveEffectiveAnchor("monthly", cms, co)
	if src != disclosureapp.TSourceCMS || got.Day != 15 {
		t.Fatalf("inactive must not reactivate: got=%+v src=%s", got, src)
	}
}

func TestPreferenceToOverrideAuthority_InactiveKeepsValuesButNotActive(t *testing.T) {
	pref := &disclosureapp.CompanyTypePreference{
		CycleAnchorDay:    20,
		OverrideFrequency: "monthly",
		OverrideActive:    boolPtr(false),
	}
	auth := disclosureapp.PreferenceToOverrideAuthority(pref, "monthly")
	if auth.Active {
		t.Fatal("inactive preference must not be Active")
	}
	if auth.Anchor.Day != 20 {
		t.Fatalf("historical values must be retained, day=%d", auth.Anchor.Day)
	}
}

func TestValidateCompanyCycleAnchorOverride_Matrix(t *testing.T) {
	if err := disclosureapp.ValidateCompanyCycleAnchorOverride("daily", disclosureapp.UpsertCompanyTypePreferenceRequest{CycleAnchorDay: 5}); err == nil {
		t.Fatal("daily with day must reject")
	}
	if err := disclosureapp.ValidateCompanyCycleAnchorOverride("weekly", disclosureapp.UpsertCompanyTypePreferenceRequest{}); err == nil {
		t.Fatal("weekly without weekday must reject")
	}
	if err := disclosureapp.ValidateCompanyCycleAnchorOverride("weekly", disclosureapp.UpsertCompanyTypePreferenceRequest{CycleAnchorWeekday: intPtr(7)}); err == nil {
		t.Fatal("weekday 7 must reject")
	}
	if err := disclosureapp.ValidateCompanyCycleAnchorOverride("monthly", disclosureapp.UpsertCompanyTypePreferenceRequest{CycleAnchorDay: 32}); err == nil {
		t.Fatal("day 32 must reject")
	}
	if err := disclosureapp.ValidateCompanyCycleAnchorOverride("quarterly", disclosureapp.UpsertCompanyTypePreferenceRequest{CycleAnchorDay: 15}); err == nil {
		t.Fatal("partial quarterly must reject")
	}
	if err := disclosureapp.ValidateCompanyCycleAnchorOverride("quarterly", disclosureapp.UpsertCompanyTypePreferenceRequest{
		MonthInQuarter: intPtr(4), CycleAnchorDay: 15,
	}); err == nil {
		t.Fatal("MiQ 4 must reject")
	}
	if err := disclosureapp.ValidateCompanyCycleAnchorOverride("yearly", disclosureapp.UpsertCompanyTypePreferenceRequest{CycleAnchorMonth: 2}); err == nil {
		t.Fatal("partial yearly must reject")
	}
	if err := disclosureapp.ValidateCompanyCycleAnchorOverride("yearly", disclosureapp.UpsertCompanyTypePreferenceRequest{
		CycleAnchorMonth: 13, CycleAnchorDay: 1,
	}); err == nil {
		t.Fatal("month 13 must reject")
	}
}

func TestBuildCompanyOverrideWrite_ClearsCrossFrequencyFields(t *testing.T) {
	req := disclosureapp.UpsertCompanyTypePreferenceRequest{
		Subject:            disclosureapp.Subject{CompanyID: "c1", MembershipID: "m1"},
		TypeID:             "t1",
		AutoCreateEnabled:  true,
		CycleAnchorMonth:   6,
		CycleAnchorDay:     15,
		CycleAnchorWeekday: intPtr(3),
		MonthInQuarter:     intPtr(2),
	}
	w := disclosureapp.BuildCompanyOverrideWrite("monthly", req)
	if w.CycleAnchorDay != 15 || w.CycleAnchorMonth != 0 || w.CycleAnchorWeekday != nil || w.MonthInQuarter != nil {
		t.Fatalf("monthly write must only keep day: %+v", w)
	}
	if w.OverrideFrequency != "monthly" || w.OverrideActive == nil || !*w.OverrideActive {
		t.Fatalf("binding/active: freq=%s active=%v", w.OverrideFrequency, w.OverrideActive)
	}
	q := disclosureapp.BuildCompanyOverrideWrite("quarterly", disclosureapp.UpsertCompanyTypePreferenceRequest{
		Subject: req.Subject, TypeID: req.TypeID, AutoCreateEnabled: true,
		MonthInQuarter: intPtr(2), CycleAnchorDay: 31, CycleAnchorMonth: 12,
	})
	if q.MonthInQuarter == nil || *q.MonthInQuarter != 2 || q.CycleAnchorDay != 31 || q.CycleAnchorMonth != 0 {
		t.Fatalf("quarterly atomic write: %+v", q)
	}
}

func TestResolveOccurrenceT_CompanyQuarterlyClampNoWriteClamp(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	cms := disclosureapp.AnchorConfig{MonthInQuarter: intPtr(1), Day: 1}
	co := disclosureapp.CompanyOverrideAuthority{
		Active: true, Frequency: "quarterly",
		Anchor: disclosureapp.AnchorConfig{MonthInQuarter: intPtr(2), Day: 31},
	}
	eff, _ := disclosureapp.ResolveEffectiveAnchor("quarterly", cms, co)
	if eff.Day != 31 || eff.MonthInQuarter == nil || *eff.MonthInQuarter != 2 {
		t.Fatalf("raw config must stay 2/31: %+v", eff)
	}
	got, err := disclosureapp.ResolveOccurrenceT("quarterly", "2026-Q1", eff, loc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Format("2006-01-02") != "2026-02-28" {
		t.Fatalf("Q1 non-leap clamp → 2026-02-28, got %s", got.Format("2006-01-02"))
	}
}

func TestResolveOccurrenceT_WeeklyCompanyWeekdaySameSlot(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	slot := "2026-08-23" // Sunday
	cms := disclosureapp.AnchorConfig{Weekday: intPtr(1)}
	co := disclosureapp.CompanyOverrideAuthority{
		Active: true, Frequency: "weekly",
		Anchor: disclosureapp.AnchorConfig{Weekday: intPtr(5)},
	}
	eff, src := disclosureapp.ResolveEffectiveAnchor("weekly", cms, co)
	if src != disclosureapp.TSourceCompany {
		t.Fatal(src)
	}
	got, err := disclosureapp.ResolveOccurrenceT("weekly", slot, eff, loc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Format("2006-01-02") != "2026-08-28" {
		t.Fatalf("Friday T want 2026-08-28 got %s", got.Format("2006-01-02"))
	}
	if disclosureapp.ResolveLogicalSlot("weekly", got, loc) != slot {
		t.Fatal("slot identity must be unchanged")
	}
}
