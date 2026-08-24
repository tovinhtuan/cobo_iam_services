package app_test

import (
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func TestValidateAndNormalizeLogicalSlots_AllFrequencies(t *testing.T) {
	cases := []struct {
		freq, raw, want string
	}{
		{"daily", "2026-08-24", "2026-08-24"},
		{"weekly", "2026-08-24", "2026-08-23"}, // Mon → Sunday
		{"weekly", "2026-08-23", "2026-08-23"},
		{"monthly", "2026-9", "2026-09"},
		{"monthly", "2026-09", "2026-09"},
		{"quarterly", "2026-Q3", "2026-Q3"},
		{"yearly", "2026", "2026"},
	}
	for _, tc := range cases {
		got, err := disclosureapp.NormalizeLogicalSlot(tc.freq, tc.raw)
		if err != nil {
			t.Fatalf("%s %s: %v", tc.freq, tc.raw, err)
		}
		if got != tc.want {
			t.Fatalf("%s %s: got %s want %s", tc.freq, tc.raw, got, tc.want)
		}
	}
	if err := disclosureapp.ValidateLogicalSlot("weekly", "2026-08-24"); err == nil {
		t.Fatal("weekly non-Sunday must fail ValidateLogicalSlot")
	}
	if err := disclosureapp.ValidateLogicalSlot("quarterly", "2026-Q5"); err == nil {
		t.Fatal("Q5 must fail")
	}
}

func TestNextLogicalSlot_Boundaries(t *testing.T) {
	cases := []struct {
		freq, slot, want string
	}{
		{"daily", "2026-12-31", "2027-01-01"},
		{"weekly", "2026-12-27", "2027-01-03"}, // Sun 27 Dec → next Sun
		{"monthly", "2026-12", "2027-01"},
		{"quarterly", "2026-Q4", "2027-Q1"},
		{"yearly", "2026", "2027"},
	}
	for _, tc := range cases {
		got, err := disclosureapp.NextLogicalSlot(tc.freq, tc.slot)
		if err != nil {
			t.Fatalf("%s: %v", tc.freq, err)
		}
		if got != tc.want {
			t.Fatalf("%s next(%s)=%s want %s", tc.freq, tc.slot, got, tc.want)
		}
	}
}

func TestFreezeApplicableFrom_CurrentNextSpecific_HCM(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	// UTC still 23 Aug 17:00 → HCM already 24 Aug 00:00
	utcBoundary := time.Date(2026, 8, 23, 17, 0, 0, 0, time.UTC)
	if utcBoundary.In(loc).Format("2006-01-02") != "2026-08-24" {
		t.Fatalf("fixture timezone mismatch")
	}

	cfg := &disclosureapp.TemplateDeadlineConfig{
		FrequencyUnit:      "monthly",
		ApplicableFromMode: disclosureapp.ApplicableFromModeCurrent,
	}
	_, slot, err := disclosureapp.FreezeApplicableFromAtActivate(cfg, utcBoundary)
	if err != nil {
		t.Fatal(err)
	}
	if slot != "2026-08" {
		t.Fatalf("CURRENT monthly HCM day → 2026-08, got %s", slot)
	}

	cfg.ApplicableFromMode = disclosureapp.ApplicableFromModeNext
	cfg.ApplicableFromSlot = ""
	_, slot, err = disclosureapp.FreezeApplicableFromAtActivate(cfg, utcBoundary)
	if err != nil {
		t.Fatal(err)
	}
	if slot != "2026-09" {
		t.Fatalf("NEXT monthly → 2026-09, got %s", slot)
	}

	cfg.ApplicableFromMode = disclosureapp.ApplicableFromModeSpecific
	cfg.ApplicableFromSlot = "2027-Q1"
	cfg.FrequencyUnit = "quarterly"
	mode, slot, err := disclosureapp.FreezeApplicableFromAtActivate(cfg, utcBoundary)
	if err != nil {
		t.Fatal(err)
	}
	if mode != disclosureapp.ApplicableFromModeSpecific || slot != "2027-Q1" {
		t.Fatalf("specific preserve got %s %s", mode, slot)
	}

	// Retry: already frozen NEXT must not move
	cfg.FrequencyUnit = "monthly"
	cfg.ApplicableFromMode = disclosureapp.ApplicableFromModeNext
	cfg.ApplicableFromSlot = "2026-09"
	later := utcBoundary.Add(48 * time.Hour)
	_, slot, err = disclosureapp.FreezeApplicableFromAtActivate(cfg, later)
	if err != nil {
		t.Fatal(err)
	}
	if slot != "2026-09" {
		t.Fatalf("retry must keep frozen slot, got %s", slot)
	}
}

func TestFreezeWeeklyIndependentOfTWeekday(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, loc) // Monday
	cfg := &disclosureapp.TemplateDeadlineConfig{
		FrequencyUnit:      "weekly",
		ApplicableFromMode: disclosureapp.ApplicableFromModeCurrent,
	}
	wd := 1
	cfg.CycleAnchorWeekday = &wd // Monday T
	_, slot1, err := disclosureapp.FreezeApplicableFromAtActivate(cfg, now)
	if err != nil {
		t.Fatal(err)
	}
	fri := 5
	cfg.CycleAnchorWeekday = &fri
	_, slot2, err := disclosureapp.FreezeApplicableFromAtActivate(cfg, now)
	if err != nil {
		t.Fatal(err)
	}
	if slot1 != "2026-08-23" || slot2 != "2026-08-23" {
		t.Fatalf("weekly slot must be Sunday 2026-08-23, got %s / %s", slot1, slot2)
	}
}

func TestIsLegacyApplicableFrom(t *testing.T) {
	if !disclosureapp.IsLegacyApplicableFrom("", "") {
		t.Fatal("empty is legacy")
	}
	if disclosureapp.IsLegacyApplicableFrom(disclosureapp.ApplicableFromModeNext, "") {
		t.Fatal("NEXT is not legacy")
	}
}

func TestPrepareApplicableFrom_LegacyUntouchedAndSameRootPromote(t *testing.T) {
	cfg := &disclosureapp.TemplateDeadlineConfig{FrequencyUnit: "monthly"}
	if err := disclosureapp.PrepareApplicableFromForDraftWrite(cfg, ""); err != nil {
		t.Fatal(err)
	}
	if cfg.ApplicableFromMode != "" || cfg.ApplicableFromSlot != "" {
		t.Fatalf("legacy must stay empty, got %+v", cfg)
	}

	cfg = &disclosureapp.TemplateDeadlineConfig{
		FrequencyUnit:      "monthly",
		ApplicableFromMode: disclosureapp.ApplicableFromModeNext,
		ApplicableFromSlot: "2026-09",
	}
	if err := disclosureapp.PrepareApplicableFromForDraftWrite(cfg, ""); err != nil {
		t.Fatal(err)
	}
	if cfg.ApplicableFromMode != disclosureapp.ApplicableFromModeSpecific || cfg.ApplicableFromSlot != "2026-09" {
		t.Fatalf("same-root relative+slot → SPECIFIC, got %+v", cfg)
	}

	cfg = &disclosureapp.TemplateDeadlineConfig{
		FrequencyUnit:      "quarterly",
		ApplicableFromMode: disclosureapp.ApplicableFromModeSpecific,
		ApplicableFromSlot: "2026-09",
	}
	if err := disclosureapp.PrepareApplicableFromForDraftWrite(cfg, "monthly"); err != nil {
		t.Fatal(err)
	}
	if cfg.ApplicableFromMode != disclosureapp.ApplicableFromModeNext || cfg.ApplicableFromSlot != "" {
		t.Fatalf("freq change must reset incompatible specific, got %+v", cfg)
	}
}

func TestCloneApplicableFromDefaults(t *testing.T) {
	cfg := &disclosureapp.TemplateDeadlineConfig{
		ApplicableFromMode: disclosureapp.ApplicableFromModeSpecific,
		ApplicableFromSlot: "2026-09",
	}
	disclosureapp.ApplyCloneApplicableFromDefaults(cfg)
	if cfg.ApplicableFromMode != disclosureapp.ApplicableFromModeNext || cfg.ApplicableFromSlot != "" {
		t.Fatalf("clone must NEXT empty slot, got %+v", cfg)
	}
}

func TestEvaluateApplicableFromEligibility_LegacyAndBounds(t *testing.T) {
	ok, dec, err := disclosureapp.EvaluateApplicableFromEligibility("monthly", "2026-08", "", "")
	if err != nil || !ok || dec != disclosureapp.ApplicableFromDecisionLegacyAllow {
		t.Fatalf("legacy: ok=%v dec=%s err=%v", ok, dec, err)
	}

	cases := []struct {
		freq, cand, bound string
		wantOK            bool
		wantDec           string
	}{
		{"daily", "2026-08-23", "2026-08-24", false, disclosureapp.ApplicableFromDecisionSkipBefore},
		{"daily", "2026-08-24", "2026-08-24", true, disclosureapp.ApplicableFromDecisionEligible},
		{"daily", "2026-08-25", "2026-08-24", true, disclosureapp.ApplicableFromDecisionEligible},
		{"weekly", "2026-08-23", "2026-08-30", false, disclosureapp.ApplicableFromDecisionSkipBefore},
		{"weekly", "2026-08-30", "2026-08-30", true, disclosureapp.ApplicableFromDecisionEligible},
		{"weekly", "2026-09-06", "2026-08-30", true, disclosureapp.ApplicableFromDecisionEligible},
		{"monthly", "2026-08", "2026-09", false, disclosureapp.ApplicableFromDecisionSkipBefore},
		{"monthly", "2026-09", "2026-09", true, disclosureapp.ApplicableFromDecisionEligible},
		{"monthly", "2026-10", "2026-09", true, disclosureapp.ApplicableFromDecisionEligible},
		{"quarterly", "2026-Q3", "2026-Q4", false, disclosureapp.ApplicableFromDecisionSkipBefore},
		{"quarterly", "2026-Q4", "2026-Q4", true, disclosureapp.ApplicableFromDecisionEligible},
		{"quarterly", "2027-Q1", "2026-Q4", true, disclosureapp.ApplicableFromDecisionEligible},
		{"yearly", "2026", "2027", false, disclosureapp.ApplicableFromDecisionSkipBefore},
		{"yearly", "2027", "2027", true, disclosureapp.ApplicableFromDecisionEligible},
		{"yearly", "2028", "2027", true, disclosureapp.ApplicableFromDecisionEligible},
	}
	for _, tc := range cases {
		ok, dec, err := disclosureapp.EvaluateApplicableFromEligibility(tc.freq, tc.cand, disclosureapp.ApplicableFromModeSpecific, tc.bound)
		if err != nil || ok != tc.wantOK || dec != tc.wantDec {
			t.Fatalf("%s cand=%s bound=%s: ok=%v dec=%s err=%v want ok=%v dec=%s",
				tc.freq, tc.cand, tc.bound, ok, dec, err, tc.wantOK, tc.wantDec)
		}
	}
}

func TestEvaluateApplicableFromEligibility_InvalidAndUnfrozen(t *testing.T) {
	ok, dec, err := disclosureapp.EvaluateApplicableFromEligibility("monthly", "2026-08", disclosureapp.ApplicableFromModeNext, "")
	if err == nil || ok || dec != disclosureapp.ApplicableFromDecisionUnfrozenMode {
		t.Fatalf("unfrozen NEXT: ok=%v dec=%s err=%v", ok, dec, err)
	}
	ok, dec, err = disclosureapp.EvaluateApplicableFromEligibility("quarterly", "2026-Q3", disclosureapp.ApplicableFromModeSpecific, "2026-09")
	if err == nil || ok || dec != disclosureapp.ApplicableFromDecisionInvalidSlot {
		t.Fatalf("cross-freq slot: ok=%v dec=%s err=%v", ok, dec, err)
	}
	ok, dec, err = disclosureapp.EvaluateApplicableFromEligibility("monthly", "2026-08", "", "2026-09")
	if err != nil || ok || dec != disclosureapp.ApplicableFromDecisionSkipBefore {
		t.Fatalf("slot-only: ok=%v dec=%s err=%v", ok, dec, err)
	}
}

func TestEvaluateApplicableFromEligibility_WeeklyIndependentOfTWeekday(t *testing.T) {
	boundary := "2026-08-30"
	cand := "2026-08-30"
	ok, _, err := disclosureapp.EvaluateApplicableFromEligibility("weekly", cand, disclosureapp.ApplicableFromModeSpecific, boundary)
	if err != nil || !ok {
		t.Fatal("Sunday boundary must be eligible")
	}
	ok, _, err = disclosureapp.EvaluateApplicableFromEligibility("weekly", "2026-08-23", disclosureapp.ApplicableFromModeSpecific, boundary)
	if err != nil || ok {
		t.Fatal("prior Sunday week must skip")
	}
}

func TestEvaluateApplicableFromEligibility_PastBoundaryDoesNotRequireHistoricalCandidates(t *testing.T) {
	// Current candidate Aug with boundary Jan → eligible; filter does not invent Jan–Jul.
	ok, dec, err := disclosureapp.EvaluateApplicableFromEligibility("monthly", "2026-08", disclosureapp.ApplicableFromModeSpecific, "2025-01")
	if err != nil || !ok || dec != disclosureapp.ApplicableFromDecisionEligible {
		t.Fatalf("past boundary + current candidate: ok=%v dec=%s err=%v", ok, dec, err)
	}
}
