package app_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func hcmLoc(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	}
	return loc
}

func TestNormalizeApplicableTo_OpenEndedAndValid(t *testing.T) {
	got, err := disclosureapp.NormalizeApplicableTo("")
	if err != nil || got != "" {
		t.Fatalf("empty → OPEN_ENDED got %q err=%v", got, err)
	}
	got, err = disclosureapp.NormalizeApplicableTo("  ")
	if err != nil || got != "" {
		t.Fatalf("whitespace → OPEN_ENDED got %q err=%v", got, err)
	}
	got, err = disclosureapp.NormalizeApplicableTo("2026-09-30")
	if err != nil || got != "2026-09-30" {
		t.Fatalf("valid got %q err=%v", got, err)
	}
	got, err = disclosureapp.NormalizeApplicableTo("2028-02-29")
	if err != nil || got != "2028-02-29" {
		t.Fatalf("leap got %q err=%v", got, err)
	}
	if !disclosureapp.IsOpenEndedApplicableTo("") {
		t.Fatal("empty must be open-ended")
	}
}

func TestNormalizeApplicableTo_Invalid(t *testing.T) {
	invalids := []string{
		"2026-02-30",
		"2026-02-29", // non-leap
		"2026-13-01",
		"2026-00-01",
		"2026-09-31",
		"2026-04-31",
		"abc",
		"2026/09/30",
		"30/09/2026",
		"2026-09-30T00:00:00Z",
		"2026-9-30",
	}
	for _, raw := range invalids {
		if _, err := disclosureapp.NormalizeApplicableTo(raw); err == nil {
			t.Fatalf("%q must fail", raw)
		}
		if err := disclosureapp.ValidateApplicableToFormat(raw); err == nil {
			t.Fatalf("ValidateApplicableToFormat(%q) must fail", raw)
		}
	}
}

func TestEvaluateApplicableToEligibility_NullOpenEnded(t *testing.T) {
	loc := hcmLoc(t)
	tOcc := time.Date(2099, 12, 31, 12, 0, 0, 0, loc)
	ok, dec, err := disclosureapp.EvaluateApplicableToEligibility(tOcc, "", loc)
	if err != nil || !ok || dec != disclosureapp.ApplicableToDecisionOpenEnded {
		t.Fatalf("NULL open-ended: ok=%v dec=%s err=%v", ok, dec, err)
	}
}

func TestEvaluateApplicableToEligibility_InclusiveBoundary(t *testing.T) {
	loc := hcmLoc(t)
	before := time.Date(2026, 9, 4, 10, 0, 0, 0, loc)
	equal := time.Date(2026, 9, 5, 23, 59, 0, 0, loc)
	after := time.Date(2026, 9, 6, 0, 0, 0, 0, loc)
	to := "2026-09-05"

	ok, dec, err := disclosureapp.EvaluateApplicableToEligibility(before, to, loc)
	if err != nil || !ok || dec != disclosureapp.ApplicableToDecisionEligible {
		t.Fatalf("before: ok=%v dec=%s err=%v", ok, dec, err)
	}
	ok, dec, err = disclosureapp.EvaluateApplicableToEligibility(equal, to, loc)
	if err != nil || !ok || dec != disclosureapp.ApplicableToDecisionEligible {
		t.Fatalf("equal inclusive: ok=%v dec=%s err=%v", ok, dec, err)
	}
	ok, dec, err = disclosureapp.EvaluateApplicableToEligibility(after, to, loc)
	if err != nil || ok || dec != disclosureapp.ApplicableToDecisionSkipAfter {
		t.Fatalf("after: ok=%v dec=%s err=%v", ok, dec, err)
	}
}

func TestEvaluateApplicableToEligibility_AuthorityIsTNotToday(t *testing.T) {
	loc := hcmLoc(t)
	// "Today" after ApplicableTo, but occurrence T still within bound.
	tOcc := time.Date(2026, 9, 5, 8, 0, 0, 0, loc)
	ok, _, err := disclosureapp.EvaluateApplicableToEligibility(tOcc, "2026-09-05", loc)
	if err != nil || !ok {
		t.Fatalf("must use T not today: ok=%v err=%v", ok, err)
	}
}

func TestEvaluateApplicableToEligibility_HCMDateNormalization(t *testing.T) {
	loc := hcmLoc(t)
	// 2026-09-30 18:00 UTC = 2026-10-01 01:00 +07 → HCM date after ApplicableTo.
	tUTC := time.Date(2026, 9, 30, 18, 0, 0, 0, time.UTC)
	ok, dec, err := disclosureapp.EvaluateApplicableToEligibility(tUTC, "2026-09-30", loc)
	if err != nil || ok || dec != disclosureapp.ApplicableToDecisionSkipAfter {
		t.Fatalf("HCM cross-midnight: ok=%v dec=%s err=%v want skip", ok, dec, err)
	}
	// Same instant still on 2026-09-30 HCM when ApplicableTo is 2026-10-01.
	ok, dec, err = disclosureapp.EvaluateApplicableToEligibility(tUTC, "2026-10-01", loc)
	if err != nil || !ok || dec != disclosureapp.ApplicableToDecisionEligible {
		t.Fatalf("HCM eligible: ok=%v dec=%s err=%v", ok, dec, err)
	}
}

func TestEvaluateApplicableToEligibility_InvalidBoundary(t *testing.T) {
	loc := hcmLoc(t)
	tOcc := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	ok, dec, err := disclosureapp.EvaluateApplicableToEligibility(tOcc, "2026-02-30", loc)
	if err == nil || ok || dec != disclosureapp.ApplicableToDecisionInvalid {
		t.Fatalf("invalid boundary: ok=%v dec=%s err=%v", ok, dec, err)
	}
}

func TestApplicableTo_DailyViaResolveOccurrenceT(t *testing.T) {
	loc := hcmLoc(t)
	to := "2026-09-05"
	cases := []struct {
		slot    string
		wantOK  bool
		wantDec string
	}{
		{"2026-09-04", true, disclosureapp.ApplicableToDecisionEligible},
		{"2026-09-05", true, disclosureapp.ApplicableToDecisionEligible},
		{"2026-09-06", false, disclosureapp.ApplicableToDecisionSkipAfter},
	}
	for _, tc := range cases {
		tEff, err := disclosureapp.ResolveOccurrenceT("daily", tc.slot, disclosureapp.AnchorConfig{}, loc)
		if err != nil {
			t.Fatalf("ResolveOccurrenceT daily %s: %v", tc.slot, err)
		}
		ok, dec, err := disclosureapp.EvaluateApplicableToEligibility(tEff, to, loc)
		if err != nil || ok != tc.wantOK || dec != tc.wantDec {
			t.Fatalf("daily slot=%s T=%s: ok=%v dec=%s err=%v want ok=%v dec=%s",
				tc.slot, tEff.Format("2006-01-02"), ok, dec, err, tc.wantOK, tc.wantDec)
		}
	}
}

func TestApplicableTo_WeeklyPartialPeriod_TAnchorBased(t *testing.T) {
	loc := hcmLoc(t)
	// Week slot Sunday 2026-09-13; T = Wednesday → 2026-09-16; ApplicableTo=15 → skip.
	wed := int(time.Wednesday)
	slot := "2026-09-13"
	tEff, err := disclosureapp.ResolveOccurrenceT("weekly", slot, disclosureapp.AnchorConfig{Weekday: &wed}, loc)
	if err != nil {
		t.Fatal(err)
	}
	if tEff.Format("2006-01-02") != "2026-09-16" {
		t.Fatalf("weekly T want 2026-09-16 got %s", tEff.Format("2006-01-02"))
	}
	ok, dec, err := disclosureapp.EvaluateApplicableToEligibility(tEff, "2026-09-15", loc)
	if err != nil || ok || dec != disclosureapp.ApplicableToDecisionSkipAfter {
		t.Fatalf("week overlap but T after: ok=%v dec=%s err=%v", ok, dec, err)
	}
	// Inclusive equal: T=2026-09-15 with ApplicableTo=2026-09-15.
	tue := int(time.Tuesday) // Sun 13 + 2 = Tue 15
	tEqual, err := disclosureapp.ResolveOccurrenceT("weekly", slot, disclosureapp.AnchorConfig{Weekday: &tue}, loc)
	if err != nil {
		t.Fatal(err)
	}
	if tEqual.Format("2006-01-02") != "2026-09-15" {
		t.Fatalf("want Tue 15 got %s", tEqual.Format("2006-01-02"))
	}
	ok, dec, err = disclosureapp.EvaluateApplicableToEligibility(tEqual, "2026-09-15", loc)
	if err != nil || !ok || dec != disclosureapp.ApplicableToDecisionEligible {
		t.Fatalf("weekly equal: ok=%v dec=%s err=%v", ok, dec, err)
	}
}

func TestApplicableTo_MonthlyPartialPeriod_P0(t *testing.T) {
	loc := hcmLoc(t)
	// PARTIAL_PERIOD_POLICY=T_ANCHOR_BASED: slot Sept, day=30, To=15 → NOT eligible.
	tEff, err := disclosureapp.ResolveOccurrenceT("monthly", "2026-09", disclosureapp.AnchorConfig{Day: 30}, loc)
	if err != nil {
		t.Fatal(err)
	}
	if tEff.Format("2006-01-02") != "2026-09-30" {
		t.Fatalf("T=%s", tEff.Format("2006-01-02"))
	}
	ok, dec, err := disclosureapp.EvaluateApplicableToEligibility(tEff, "2026-09-15", loc)
	if err != nil || ok || dec != disclosureapp.ApplicableToDecisionSkipAfter {
		t.Fatalf("monthly T=30 To=15: ok=%v dec=%s err=%v (PARTIAL_PERIOD=T_ANCHOR)", ok, dec, err)
	}
	// Positive: day=10, To=15 → eligible.
	t10, err := disclosureapp.ResolveOccurrenceT("monthly", "2026-09", disclosureapp.AnchorConfig{Day: 10}, loc)
	if err != nil {
		t.Fatal(err)
	}
	ok, dec, err = disclosureapp.EvaluateApplicableToEligibility(t10, "2026-09-15", loc)
	if err != nil || !ok || dec != disclosureapp.ApplicableToDecisionEligible {
		t.Fatalf("monthly T=10 To=15: ok=%v dec=%s err=%v", ok, dec, err)
	}
}

func TestApplicableTo_MonthlyClamp(t *testing.T) {
	loc := hcmLoc(t)
	// day=31 in April → clamp to 2026-04-30.
	tEff, err := disclosureapp.ResolveOccurrenceT("monthly", "2026-04", disclosureapp.AnchorConfig{Day: 31}, loc)
	if err != nil {
		t.Fatal(err)
	}
	if tEff.Format("2006-01-02") != "2026-04-30" {
		t.Fatalf("April clamp T=%s", tEff.Format("2006-01-02"))
	}
	ok, _, err := disclosureapp.EvaluateApplicableToEligibility(tEff, "2026-04-30", loc)
	if err != nil || !ok {
		t.Fatalf("clamp equal: ok=%v err=%v", ok, err)
	}
	ok, dec, err := disclosureapp.EvaluateApplicableToEligibility(tEff, "2026-04-29", loc)
	if err != nil || ok || dec != disclosureapp.ApplicableToDecisionSkipAfter {
		t.Fatalf("clamp after To: ok=%v dec=%s err=%v", ok, dec, err)
	}
	// February non-leap day=31 → 2026-02-28.
	tFeb, err := disclosureapp.ResolveOccurrenceT("monthly", "2026-02", disclosureapp.AnchorConfig{Day: 31}, loc)
	if err != nil {
		t.Fatal(err)
	}
	if tFeb.Format("2006-01-02") != "2026-02-28" {
		t.Fatalf("Feb clamp T=%s", tFeb.Format("2006-01-02"))
	}
	ok, _, err = disclosureapp.EvaluateApplicableToEligibility(tFeb, "2026-02-28", loc)
	if err != nil || !ok {
		t.Fatalf("Feb clamp equal: ok=%v err=%v", ok, err)
	}
}

func TestApplicableTo_Quarterly(t *testing.T) {
	loc := hcmLoc(t)
	miq3 := 3
	tEff, err := disclosureapp.ResolveOccurrenceT("quarterly", "2026-Q3", disclosureapp.AnchorConfig{
		Day: 30, MonthInQuarter: &miq3,
	}, loc)
	if err != nil {
		t.Fatal(err)
	}
	if tEff.Format("2006-01-02") != "2026-09-30" {
		t.Fatalf("Q3 T=%s", tEff.Format("2006-01-02"))
	}
	ok, dec, err := disclosureapp.EvaluateApplicableToEligibility(tEff, "2026-08-31", loc)
	if err != nil || ok || dec != disclosureapp.ApplicableToDecisionSkipAfter {
		t.Fatalf("quarterly after: ok=%v dec=%s err=%v", ok, dec, err)
	}
	ok, _, err = disclosureapp.EvaluateApplicableToEligibility(tEff, "2026-09-30", loc)
	if err != nil || !ok {
		t.Fatalf("quarterly inclusive: ok=%v err=%v", ok, err)
	}
}

func TestApplicableTo_Yearly(t *testing.T) {
	loc := hcmLoc(t)
	tEff, err := disclosureapp.ResolveOccurrenceT("yearly", "2026", disclosureapp.AnchorConfig{
		Month: 12, Day: 31,
	}, loc)
	if err != nil {
		t.Fatal(err)
	}
	if tEff.Format("2006-01-02") != "2026-12-31" {
		t.Fatalf("yearly T=%s", tEff.Format("2006-01-02"))
	}
	ok, dec, err := disclosureapp.EvaluateApplicableToEligibility(tEff, "2026-06-30", loc)
	if err != nil || ok || dec != disclosureapp.ApplicableToDecisionSkipAfter {
		t.Fatalf("yearly after: ok=%v dec=%s err=%v", ok, dec, err)
	}
	ok, _, err = disclosureapp.EvaluateApplicableToEligibility(tEff, "2026-12-31", loc)
	if err != nil || !ok {
		t.Fatalf("yearly inclusive: ok=%v err=%v", ok, err)
	}
}

func TestApplicableTo_YearlyLeapClamp(t *testing.T) {
	loc := hcmLoc(t)
	// Non-leap 2026 Feb 29 config → clamp to Feb 28.
	t2026, err := disclosureapp.ResolveOccurrenceT("yearly", "2026", disclosureapp.AnchorConfig{
		Month: 2, Day: 29,
	}, loc)
	if err != nil {
		t.Fatal(err)
	}
	if t2026.Format("2006-01-02") != "2026-02-28" {
		t.Fatalf("2026 leap-clamp T=%s", t2026.Format("2006-01-02"))
	}
	ok, _, err := disclosureapp.EvaluateApplicableToEligibility(t2026, "2026-02-28", loc)
	if err != nil || !ok {
		t.Fatalf("2026 clamp equal: ok=%v err=%v", ok, err)
	}
	// Leap year 2028 Feb 29.
	t2028, err := disclosureapp.ResolveOccurrenceT("yearly", "2028", disclosureapp.AnchorConfig{
		Month: 2, Day: 29,
	}, loc)
	if err != nil {
		t.Fatal(err)
	}
	if t2028.Format("2006-01-02") != "2028-02-29" {
		t.Fatalf("2028 leap T=%s", t2028.Format("2006-01-02"))
	}
	ok, _, err = disclosureapp.EvaluateApplicableToEligibility(t2028, "2028-02-29", loc)
	if err != nil || !ok {
		t.Fatalf("2028 leap inclusive: ok=%v err=%v", ok, err)
	}
}

func TestTemplateDeadlineConfig_ApplicableToJSONCompat(t *testing.T) {
	// Legacy omit → OPEN_ENDED
	var legacy disclosureapp.TemplateDeadlineConfig
	if err := json.Unmarshal([]byte(`{"frequency_unit":"monthly"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.ApplicableTo != "" || !disclosureapp.IsOpenEndedApplicableTo(legacy.ApplicableTo) {
		t.Fatalf("legacy omit ApplicableTo=%q", legacy.ApplicableTo)
	}
	out, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "applicable_to") {
		t.Fatalf("omitempty must omit empty applicable_to: %s", out)
	}

	// JSON null → OPEN_ENDED
	var nullCfg disclosureapp.TemplateDeadlineConfig
	if err := json.Unmarshal([]byte(`{"applicable_to":null}`), &nullCfg); err != nil {
		t.Fatal(err)
	}
	if nullCfg.ApplicableTo != "" {
		t.Fatalf("null → empty got %q", nullCfg.ApplicableTo)
	}

	// Value round-trip
	var withVal disclosureapp.TemplateDeadlineConfig
	if err := json.Unmarshal([]byte(`{"applicable_to":"2026-09-30","frequency_unit":"daily"}`), &withVal); err != nil {
		t.Fatal(err)
	}
	if withVal.ApplicableTo != "2026-09-30" {
		t.Fatalf("value=%q", withVal.ApplicableTo)
	}
	round, err := json.Marshal(withVal)
	if err != nil {
		t.Fatal(err)
	}
	var again disclosureapp.TemplateDeadlineConfig
	if err := json.Unmarshal(round, &again); err != nil {
		t.Fatal(err)
	}
	if again.ApplicableTo != "2026-09-30" {
		t.Fatalf("roundtrip=%q", again.ApplicableTo)
	}
}

func TestPrepareApplicableToForDraftWrite(t *testing.T) {
	cfg := &disclosureapp.TemplateDeadlineConfig{ApplicableTo: " 2026-09-30 "}
	if err := disclosureapp.PrepareApplicableToForDraftWrite(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.ApplicableTo != "2026-09-30" {
		t.Fatalf("got %q", cfg.ApplicableTo)
	}
	cfg.ApplicableTo = "bad"
	if err := disclosureapp.PrepareApplicableToForDraftWrite(cfg); err == nil {
		t.Fatal("bad must fail")
	}
	if err := disclosureapp.PrepareApplicableToForDraftWrite(nil); err != nil {
		t.Fatal(err)
	}
}
