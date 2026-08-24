package app_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func intPtr(v int) *int { return &v }

func TestValidateCycleAnchorWeekday(t *testing.T) {
	if err := disclosureapp.ValidateCycleAnchorWeekday(nil); err != nil {
		t.Fatalf("nil must be allowed: %v", err)
	}
	for _, d := range []int{0, 1, 2, 3, 4, 5, 6} {
		if err := disclosureapp.ValidateCycleAnchorWeekday(intPtr(d)); err != nil {
			t.Fatalf("weekday %d: %v", d, err)
		}
	}
	for _, d := range []int{-1, 7, 99} {
		if err := disclosureapp.ValidateCycleAnchorWeekday(intPtr(d)); err == nil {
			t.Fatalf("weekday %d must reject", d)
		}
	}
}

func TestValidateMonthInQuarter(t *testing.T) {
	if err := disclosureapp.ValidateMonthInQuarter(nil); err != nil {
		t.Fatalf("nil must be allowed: %v", err)
	}
	for _, m := range []int{1, 2, 3} {
		if err := disclosureapp.ValidateMonthInQuarter(intPtr(m)); err != nil {
			t.Fatalf("miq %d: %v", m, err)
		}
	}
	for _, m := range []int{0, 4, 100} {
		if err := disclosureapp.ValidateMonthInQuarter(intPtr(m)); err == nil {
			t.Fatalf("miq %d must reject", m)
		}
	}
}

func TestNormalizeScheduleAnchor_LegacyDefaults(t *testing.T) {
	w := disclosureapp.NormalizeScheduleAnchor("weekly", disclosureapp.AnchorConfig{})
	if w.Weekday == nil || *w.Weekday != int(time.Sunday) {
		t.Fatalf("weekly nil → Sunday, got %+v", w.Weekday)
	}
	mon := int(time.Monday)
	w2 := disclosureapp.NormalizeScheduleAnchor("weekly", disclosureapp.AnchorConfig{Weekday: &mon})
	if w2.Weekday == nil || *w2.Weekday != mon {
		t.Fatalf("weekly explicit Monday preserved")
	}

	q := disclosureapp.NormalizeScheduleAnchor("quarterly", disclosureapp.AnchorConfig{})
	if q.MonthInQuarter == nil || *q.MonthInQuarter != 1 || q.Day != 1 {
		t.Fatalf("quarterly nil → 1/1, got miq=%v day=%d", q.MonthInQuarter, q.Day)
	}
	miq := 2
	q2 := disclosureapp.NormalizeScheduleAnchor("quarterly", disclosureapp.AnchorConfig{MonthInQuarter: &miq, Day: 15})
	if q2.MonthInQuarter == nil || *q2.MonthInQuarter != 2 || q2.Day != 15 {
		t.Fatalf("quarterly explicit preserved: %+v", q2)
	}
}

func TestTemplateDeadlineConfig_JSONRoundTripOptionalAnchors(t *testing.T) {
	legacy := `{"deadline_mode":"PERIODIC","frequency_unit":"weekly","cycle_anchor_day":1}`
	var cfg disclosureapp.TemplateDeadlineConfig
	if err := json.Unmarshal([]byte(legacy), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.CycleAnchorWeekday != nil || cfg.MonthInQuarter != nil {
		t.Fatal("absent fields must stay nil")
	}

	sun := 0
	miq := 2
	cfg2 := disclosureapp.TemplateDeadlineConfig{
		DeadlineMode:       "PERIODIC",
		FrequencyUnit:      "quarterly",
		CycleAnchorDay:     15,
		CycleAnchorWeekday: &sun,
		MonthInQuarter:     &miq,
	}
	raw, err := json.Marshal(cfg2)
	if err != nil {
		t.Fatal(err)
	}
	var back disclosureapp.TemplateDeadlineConfig
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.CycleAnchorWeekday == nil || *back.CycleAnchorWeekday != 0 {
		t.Fatalf("Sunday weekday=0 must roundtrip, got %v", back.CycleAnchorWeekday)
	}
	if back.MonthInQuarter == nil || *back.MonthInQuarter != 2 || back.CycleAnchorDay != 15 {
		t.Fatalf("quarterly fields roundtrip failed: %+v", back)
	}
}

func TestResolveOccurrenceT_DailyUnchanged(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	// Stale month/day/weekday must not influence daily T.
	mon := int(time.Monday)
	miq := 3
	got, err := disclosureapp.ResolveOccurrenceT("daily", "2026-08-24", disclosureapp.AnchorConfig{
		Month: 12, Day: 31, Weekday: &mon, MonthInQuarter: &miq,
	}, loc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Format("2006-01-02") != "2026-08-24" {
		t.Fatalf("got %s", got.Format("2006-01-02"))
	}
}

func TestResolveOccurrenceT_WeeklyLegacySundayExact(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	slot := "2026-08-23" // Sunday
	got, err := disclosureapp.ResolveOccurrenceT("weekly", slot, disclosureapp.AnchorConfig{}, loc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Format("2006-01-02") != "2026-08-23" {
		t.Fatalf("legacy weekly T drift: got %s", got.Format("2006-01-02"))
	}
}

func TestResolveOccurrenceT_WeeklyConfigurableWeekdaySameSlot(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	slot := "2026-08-23" // Sunday 23/08 .. Sat 29/08
	want := map[int]string{
		0: "2026-08-23",
		1: "2026-08-24",
		2: "2026-08-25",
		3: "2026-08-26",
		4: "2026-08-27",
		5: "2026-08-28",
		6: "2026-08-29",
	}
	for wd, date := range want {
		got, err := disclosureapp.ResolveOccurrenceT("weekly", slot, disclosureapp.AnchorConfig{Weekday: intPtr(wd)}, loc)
		if err != nil {
			t.Fatal(err)
		}
		if got.Format("2006-01-02") != date {
			t.Fatalf("weekday=%d got %s want %s", wd, got.Format("2006-01-02"), date)
		}
		// Slot identity unchanged: ResolveLogicalSlot for a day in week still Sunday key.
		now := time.Date(2026, 8, 23+wd, 12, 0, 0, 0, loc)
		if disclosureapp.ResolveLogicalSlot("weekly", now, loc) != slot {
			t.Fatalf("slot identity moved for weekday=%d", wd)
		}
	}
}

func TestResolveOccurrenceT_WeeklyCrossMonth(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	slot := "2026-08-30" // Sunday 30/08
	tue := int(time.Tuesday)
	got, err := disclosureapp.ResolveOccurrenceT("weekly", slot, disclosureapp.AnchorConfig{Weekday: &tue}, loc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Format("2006-01-02") != "2026-09-01" {
		t.Fatalf("got %s", got.Format("2006-01-02"))
	}
	if disclosureapp.ResolveLogicalSlot("weekly", got, loc) != slot {
		t.Fatal("cross-month T must remain same weekly slot")
	}
}

func TestResolveOccurrenceT_WeeklyCrossYear(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	// Sunday 2026-12-27 → Sat 2027-01-02
	slot := "2026-12-27"
	sat := int(time.Saturday)
	got, err := disclosureapp.ResolveOccurrenceT("weekly", slot, disclosureapp.AnchorConfig{Weekday: &sat}, loc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Format("2006-01-02") != "2027-01-02" {
		t.Fatalf("got %s", got.Format("2006-01-02"))
	}
	if disclosureapp.ResolveLogicalSlot("weekly", got, loc) != slot {
		t.Fatal("cross-year T must remain same weekly slot key")
	}
}

func TestResolveOccurrenceT_MonthlyClampUnchanged(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	cases := []struct {
		label string
		day   int
		want  string
	}{
		{"2026-01", 31, "2026-01-31"},
		{"2026-02", 31, "2026-02-28"},
		{"2028-02", 31, "2028-02-29"},
		{"2026-02", 29, "2026-02-28"},
		{"2028-02", 29, "2028-02-29"},
		{"2026-04", 31, "2026-04-30"},
		{"2026-05", 31, "2026-05-31"},
		{"2026-02", 30, "2026-02-28"},
		{"2026-01", 1, "2026-01-01"},
		{"2026-01", 28, "2026-01-28"},
	}
	for _, tc := range cases {
		got, err := disclosureapp.ResolveOccurrenceT("monthly", tc.label, disclosureapp.AnchorConfig{Day: tc.day}, loc)
		if err != nil {
			t.Fatal(err)
		}
		if got.Format("2006-01-02") != tc.want {
			t.Fatalf("%s day=%d → %s want %s", tc.label, tc.day, got.Format("2006-01-02"), tc.want)
		}
	}
}

func TestResolveOccurrenceT_QuarterlyLegacyFirstDayExact(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	cases := map[string]string{
		"2026-Q1": "2026-01-01",
		"2026-Q2": "2026-04-01",
		"2026-Q3": "2026-07-01",
		"2026-Q4": "2026-10-01",
	}
	for label, want := range cases {
		got, err := disclosureapp.ResolveOccurrenceT("quarterly", label, disclosureapp.AnchorConfig{}, loc)
		if err != nil {
			t.Fatal(err)
		}
		if got.Format("2006-01-02") != want {
			t.Fatalf("%s legacy drift: got %s want %s", label, got.Format("2006-01-02"), want)
		}
	}
}

func TestResolveOccurrenceT_QuarterlyConfigurableSameSlot(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	slot := "2026-Q1"
	cases := []struct {
		miq, day int
		want     string
	}{
		{1, 1, "2026-01-01"},
		{2, 15, "2026-02-15"},
		{3, 31, "2026-03-31"},
		{2, 31, "2026-02-28"}, // Feb clamp non-leap
	}
	for _, tc := range cases {
		got, err := disclosureapp.ResolveOccurrenceT("quarterly", slot, disclosureapp.AnchorConfig{
			MonthInQuarter: intPtr(tc.miq), Day: tc.day,
		}, loc)
		if err != nil {
			t.Fatal(err)
		}
		if got.Format("2006-01-02") != tc.want {
			t.Fatalf("MiQ=%d day=%d → %s want %s", tc.miq, tc.day, got.Format("2006-01-02"), tc.want)
		}
	}
}

func TestResolveOccurrenceT_QuarterlyAllQuartersMiQ2Day15(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	want := map[string]string{
		"2026-Q1": "2026-02-15",
		"2026-Q2": "2026-05-15",
		"2026-Q3": "2026-08-15",
		"2026-Q4": "2026-11-15",
	}
	for slot, date := range want {
		got, err := disclosureapp.ResolveOccurrenceT("quarterly", slot, disclosureapp.AnchorConfig{
			MonthInQuarter: intPtr(2), Day: 15,
		}, loc)
		if err != nil {
			t.Fatal(err)
		}
		if got.Format("2006-01-02") != date {
			t.Fatalf("%s → %s want %s", slot, got.Format("2006-01-02"), date)
		}
	}
}

func TestResolveOccurrenceT_QuarterlyQ4YearBoundary(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	got, err := disclosureapp.ResolveOccurrenceT("quarterly", "2026-Q4", disclosureapp.AnchorConfig{
		MonthInQuarter: intPtr(3), Day: 31,
	}, loc)
	if err != nil {
		t.Fatal(err)
	}
	if got.Format("2006-01-02") != "2026-12-31" {
		t.Fatalf("Q4 MiQ=3 day=31 → %s", got.Format("2006-01-02"))
	}
}

func TestResolveOccurrenceT_QuarterlyDay31ClampByQuarter(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	cases := []struct {
		slot string
		want string
	}{
		{"2026-Q1", "2026-02-28"}, // Feb
		{"2026-Q2", "2026-05-31"}, // May
		{"2026-Q3", "2026-08-31"}, // Aug
		{"2026-Q4", "2026-11-30"}, // Nov
	}
	for _, tc := range cases {
		got, err := disclosureapp.ResolveOccurrenceT("quarterly", tc.slot, disclosureapp.AnchorConfig{
			MonthInQuarter: intPtr(2), Day: 31,
		}, loc)
		if err != nil {
			t.Fatal(err)
		}
		if got.Format("2006-01-02") != tc.want {
			t.Fatalf("%s MiQ=2 day=31 → %s want %s", tc.slot, got.Format("2006-01-02"), tc.want)
		}
	}
}

func TestResolveOccurrenceT_YearlyLeapClampUnchanged(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	cases := []struct {
		label       string
		month, day  int
		want        string
	}{
		{"2028", 2, 29, "2028-02-29"},
		{"2029", 2, 29, "2029-02-28"},
		{"2026", 2, 31, "2026-02-28"},
		{"2026", 1, 1, "2026-01-01"},
		{"2026", 12, 31, "2026-12-31"},
	}
	for _, tc := range cases {
		got, err := disclosureapp.ResolveOccurrenceT("yearly", tc.label, disclosureapp.AnchorConfig{
			Month: tc.month, Day: tc.day,
		}, loc)
		if err != nil {
			t.Fatal(err)
		}
		if got.Format("2006-01-02") != tc.want {
			t.Fatalf("%s m=%d d=%d → %s want %s", tc.label, tc.month, tc.day, got.Format("2006-01-02"), tc.want)
		}
	}
}

func TestResolveLogicalSlot_IdentitiesUnchanged(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	now := time.Date(2026, 8, 24, 15, 0, 0, 0, loc) // Monday
	if got := disclosureapp.ResolveLogicalSlot("daily", now, loc); got != "2026-08-24" {
		t.Fatalf("daily slot %s", got)
	}
	if got := disclosureapp.ResolveLogicalSlot("weekly", now, loc); got != "2026-08-23" {
		t.Fatalf("weekly slot %s", got)
	}
	if got := disclosureapp.ResolveLogicalSlot("monthly", now, loc); got != "2026-08" {
		t.Fatalf("monthly slot %s", got)
	}
	if got := disclosureapp.ResolveLogicalSlot("quarterly", now, loc); got != "2026-Q3" {
		t.Fatalf("quarterly slot %s", got)
	}
	if got := disclosureapp.ResolveLogicalSlot("yearly", now, loc); got != "2026" {
		t.Fatalf("yearly slot %s", got)
	}
}

func TestResolveOpenAt_UsesNewWeeklyT(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	wed := int(time.Wednesday)
	tEff, err := disclosureapp.ResolveOccurrenceT("weekly", "2026-08-23", disclosureapp.AnchorConfig{Weekday: &wed}, loc)
	if err != nil {
		t.Fatal(err)
	}
	open := disclosureapp.ResolveOpenAt(tEff, 2)
	if open.Format("2006-01-02") != "2026-08-24" {
		t.Fatalf("OpenAt=%s want 2026-08-24 (Wed T - 2)", open.Format("2006-01-02"))
	}
}

func TestDeadlineInclusive_FromNewResolverT(t *testing.T) {
	calc := disclosureapp.NewDeadlineCalculator(nil)
	loc := time.FixedZone("ICT", 7*3600)
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, loc)

	// Yearly T=30/09 N=20 calendar inclusive → Due 19/10 (frozen golden).
	summary, err := calc.CalculateDeadlineSummary(context.Background(), &disclosureapp.TemplateDeadlineConfig{
		DeadlineMode:         disclosureapp.DeadlineModePeriodic,
		FrequencyUnit:        "yearly",
		CycleAnchorMonth:     9,
		CycleAnchorDay:       30,
		DeadlineDays:         20,
		DeadlineDurationType: disclosureapp.DurationTypeCalendarDays,
	}, disclosureapp.CompanyDeadlineContext{}, now)
	if err != nil {
		t.Fatal(err)
	}
	if summary.StartDate == nil || *summary.StartDate != "2026-09-30" {
		t.Fatalf("T=%v", summary.StartDate)
	}
	if summary.DeadlineDate == nil || *summary.DeadlineDate != "2026-10-19" {
		t.Fatalf("Due=%v want 2026-10-19", summary.DeadlineDate)
	}

	// Weekly Monday T in Aug week → feeds same inclusive formula.
	mon := int(time.Monday)
	nowW := time.Date(2026, 8, 26, 12, 0, 0, 0, loc)
	wsum, err := calc.CalculateDeadlineSummary(context.Background(), &disclosureapp.TemplateDeadlineConfig{
		DeadlineMode:         disclosureapp.DeadlineModePeriodic,
		FrequencyUnit:        "weekly",
		CycleAnchorWeekday:   &mon,
		DeadlineDays:         5,
		DeadlineDurationType: disclosureapp.DurationTypeCalendarDays,
	}, disclosureapp.CompanyDeadlineContext{}, nowW)
	if err != nil {
		t.Fatal(err)
	}
	if wsum.StartDate == nil || *wsum.StartDate != "2026-08-24" {
		t.Fatalf("weekly T=%v want 2026-08-24", wsum.StartDate)
	}
	if wsum.DeadlineDate == nil || *wsum.DeadlineDate != "2026-08-28" {
		t.Fatalf("weekly Due=%v want 2026-08-28 (T+5-1)", wsum.DeadlineDate)
	}

	// Quarterly MiQ=2 day=15 in Q3 → 15/08; N=10 calendar → 24/08.
	nowQ := time.Date(2026, 8, 20, 12, 0, 0, 0, loc)
	miq := 2
	qsum, err := calc.CalculateDeadlineSummary(context.Background(), &disclosureapp.TemplateDeadlineConfig{
		DeadlineMode:         disclosureapp.DeadlineModePeriodic,
		FrequencyUnit:        "quarterly",
		MonthInQuarter:       &miq,
		CycleAnchorDay:       15,
		DeadlineDays:         10,
		DeadlineDurationType: disclosureapp.DurationTypeCalendarDays,
	}, disclosureapp.CompanyDeadlineContext{}, nowQ)
	if err != nil {
		t.Fatal(err)
	}
	if qsum.StartDate == nil || *qsum.StartDate != "2026-08-15" {
		t.Fatalf("quarterly T=%v", qsum.StartDate)
	}
	if qsum.DeadlineDate == nil || *qsum.DeadlineDate != "2026-08-24" {
		t.Fatalf("quarterly Due=%v", qsum.DeadlineDate)
	}
}
