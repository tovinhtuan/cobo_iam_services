package app_test

import (
	"encoding/json"
	"testing"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

func monthlyCfg(applicableFromSlot, applicableTo string) *disclosureapp.TemplateDeadlineConfig {
	return &disclosureapp.TemplateDeadlineConfig{
		DeadlineMode:       "PERIODIC",
		FrequencyUnit:      "monthly",
		ApplicableFromMode: disclosureapp.ApplicableFromModeSpecific,
		ApplicableFromSlot: applicableFromSlot,
		ApplicableTo:       applicableTo,
	}
}

func TestResolveTemplateApplicabilityState_OpenEndedActive(t *testing.T) {
	eval := time.Date(2026, 9, 15, 10, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	state, ok := disclosureapp.ResolveTemplateApplicabilityState(
		monthlyCfg("2026-09", ""),
		"monthly",
		eval,
	)
	if !ok || state != disclosureapp.TemplateApplicabilityStateActive {
		t.Fatalf("got state=%q ok=%v want ACTIVE", state, ok)
	}
}

func TestResolveTemplateApplicabilityState_OpenEndedUpcoming(t *testing.T) {
	eval := time.Date(2026, 9, 15, 10, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	state, ok := disclosureapp.ResolveTemplateApplicabilityState(
		monthlyCfg("2026-10", ""),
		"monthly",
		eval,
	)
	if !ok || state != disclosureapp.TemplateApplicabilityStateUpcoming {
		t.Fatalf("got state=%q ok=%v want UPCOMING", state, ok)
	}
}

func TestResolveTemplateApplicabilityState_Ended(t *testing.T) {
	eval := time.Date(2026, 9, 10, 10, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	state, ok := disclosureapp.ResolveTemplateApplicabilityState(
		monthlyCfg("2026-01", "2026-09-09"),
		"monthly",
		eval,
	)
	if !ok || state != disclosureapp.TemplateApplicabilityStateEnded {
		t.Fatalf("got state=%q ok=%v want ENDED", state, ok)
	}
}

func TestResolveTemplateApplicabilityState_EqualTodayNotEnded(t *testing.T) {
	eval := time.Date(2026, 9, 10, 10, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	state, ok := disclosureapp.ResolveTemplateApplicabilityState(
		monthlyCfg("2026-01", "2026-09-10"),
		"monthly",
		eval,
	)
	if !ok || state != disclosureapp.TemplateApplicabilityStateActive {
		t.Fatalf("got state=%q ok=%v want ACTIVE (inclusive end)", state, ok)
	}
}

func TestResolveTemplateApplicabilityState_HCMBoundary(t *testing.T) {
	// 2026-09-09 17:30 UTC = 2026-09-10 00:30 ICT → TodayHCM = 2026-09-10
	evalUTC := time.Date(2026, 9, 9, 17, 30, 0, 0, time.UTC)
	state, ok := disclosureapp.ResolveTemplateApplicabilityState(
		monthlyCfg("2026-01", "2026-09-09"),
		"monthly",
		evalUTC,
	)
	if !ok || state != disclosureapp.TemplateApplicabilityStateEnded {
		t.Fatalf("got state=%q ok=%v want ENDED using HCM calendar", state, ok)
	}
	// Same UTC instant earlier: 2026-09-09 16:30 UTC = still 2026-09-09 ICT
	evalSameDay := time.Date(2026, 9, 9, 16, 30, 0, 0, time.UTC)
	state2, ok2 := disclosureapp.ResolveTemplateApplicabilityState(
		monthlyCfg("2026-01", "2026-09-09"),
		"monthly",
		evalSameDay,
	)
	if !ok2 || state2 != disclosureapp.TemplateApplicabilityStateActive {
		t.Fatalf("got state=%q ok=%v want ACTIVE on ApplicableTo==TodayHCM", state2, ok2)
	}
}

func TestResolveTemplateApplicabilityState_DailyUpcoming(t *testing.T) {
	eval := time.Date(2026, 9, 5, 10, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	cfg := &disclosureapp.TemplateDeadlineConfig{
		FrequencyUnit:      "daily",
		ApplicableFromMode: disclosureapp.ApplicableFromModeSpecific,
		ApplicableFromSlot: "2026-09-10",
	}
	state, ok := disclosureapp.ResolveTemplateApplicabilityState(cfg, "daily", eval)
	if !ok || state != disclosureapp.TemplateApplicabilityStateUpcoming {
		t.Fatalf("got state=%q ok=%v want UPCOMING", state, ok)
	}
}

func TestResolveTemplateApplicabilityState_WeeklyUpcoming(t *testing.T) {
	// Sunday-based week: slot 2026-09-13 is week of Sep 13. Today Sep 8 is prior week (Sep 6 Sunday).
	eval := time.Date(2026, 9, 8, 10, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	cfg := &disclosureapp.TemplateDeadlineConfig{
		FrequencyUnit:      "weekly",
		ApplicableFromMode: disclosureapp.ApplicableFromModeSpecific,
		ApplicableFromSlot: "2026-09-13",
	}
	state, ok := disclosureapp.ResolveTemplateApplicabilityState(cfg, "weekly", eval)
	if !ok || state != disclosureapp.TemplateApplicabilityStateUpcoming {
		t.Fatalf("got state=%q ok=%v want UPCOMING", state, ok)
	}
}

func TestResolveTemplateApplicabilityState_QuarterlyUpcoming(t *testing.T) {
	eval := time.Date(2026, 8, 1, 10, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	cfg := &disclosureapp.TemplateDeadlineConfig{
		FrequencyUnit:      "quarterly",
		ApplicableFromMode: disclosureapp.ApplicableFromModeSpecific,
		ApplicableFromSlot: "2026-Q4",
	}
	state, ok := disclosureapp.ResolveTemplateApplicabilityState(cfg, "quarterly", eval)
	if !ok || state != disclosureapp.TemplateApplicabilityStateUpcoming {
		t.Fatalf("got state=%q ok=%v want UPCOMING", state, ok)
	}
}

func TestResolveTemplateApplicabilityState_YearlyUpcoming(t *testing.T) {
	eval := time.Date(2026, 6, 1, 10, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	cfg := &disclosureapp.TemplateDeadlineConfig{
		FrequencyUnit:      "yearly",
		ApplicableFromMode: disclosureapp.ApplicableFromModeSpecific,
		ApplicableFromSlot: "2027",
	}
	state, ok := disclosureapp.ResolveTemplateApplicabilityState(cfg, "yearly", eval)
	if !ok || state != disclosureapp.TemplateApplicabilityStateUpcoming {
		t.Fatalf("got state=%q ok=%v want UPCOMING", state, ok)
	}
}

func TestResolveTemplateApplicabilityState_EndedPrecedenceOverFutureFrom(t *testing.T) {
	eval := time.Date(2026, 9, 10, 10, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	state, ok := disclosureapp.ResolveTemplateApplicabilityState(
		monthlyCfg("2027-01", "2026-09-05"),
		"monthly",
		eval,
	)
	if !ok || state != disclosureapp.TemplateApplicabilityStateEnded {
		t.Fatalf("got state=%q ok=%v want ENDED first", state, ok)
	}
}

func TestResolveTemplateApplicabilityState_InvalidApplicableToOmits(t *testing.T) {
	eval := time.Date(2026, 9, 10, 10, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	cfg := monthlyCfg("2026-01", "not-a-date")
	state, ok := disclosureapp.ResolveTemplateApplicabilityState(cfg, "monthly", eval)
	if ok || state != "" {
		t.Fatalf("want omit on invalid ApplicableTo, got state=%q ok=%v", state, ok)
	}
}

func TestResolveTemplateApplicabilityState_InvalidApplicableFromOmits(t *testing.T) {
	eval := time.Date(2026, 9, 10, 10, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	cfg := &disclosureapp.TemplateDeadlineConfig{
		FrequencyUnit:      "monthly",
		ApplicableFromMode: disclosureapp.ApplicableFromModeSpecific,
		ApplicableFromSlot: "bad-slot",
	}
	state, ok := disclosureapp.ResolveTemplateApplicabilityState(cfg, "monthly", eval)
	if ok || state != "" {
		t.Fatalf("want omit on invalid ApplicableFrom, got state=%q ok=%v", state, ok)
	}
}

func TestResolveTemplateApplicabilityState_NonPeriodicOmits(t *testing.T) {
	eval := time.Date(2026, 9, 10, 10, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	cfg := &disclosureapp.TemplateDeadlineConfig{
		FrequencyUnit: "ad_hoc",
		ApplicableTo:  "2020-01-01",
	}
	state, ok := disclosureapp.ResolveTemplateApplicabilityState(cfg, "event_based", eval)
	if ok || state != "" {
		t.Fatalf("want omit for non-periodic, got state=%q ok=%v", state, ok)
	}
}

func TestApplicabilityState_JSONAdditiveWithActiveVersion(t *testing.T) {
	item := disclosureapp.DisclosureTypeSummaryDTO{
		TypeID:             "dt-1",
		ActiveVersionNo:    1,
		ApplicabilityState: disclosureapp.TemplateApplicabilityStateEnded,
	}
	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["applicability_state"] != "ENDED" {
		t.Fatalf("applicability_state=%v", m["applicability_state"])
	}
	if int(m["active_version_no"].(float64)) != 1 {
		t.Fatalf("lifecycle pointer missing: %v", m["active_version_no"])
	}
	if _, has := m["deadline_config"]; has {
		t.Fatal("deadline_config must not leak on list summary")
	}
}

func TestResolveTemplateApplicabilityState_LegacyApplicableFromActive(t *testing.T) {
	eval := time.Date(2026, 9, 15, 10, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	cfg := &disclosureapp.TemplateDeadlineConfig{
		FrequencyUnit: "monthly",
		ApplicableTo:  "",
	}
	state, ok := disclosureapp.ResolveTemplateApplicabilityState(cfg, "monthly", eval)
	if !ok || state != disclosureapp.TemplateApplicabilityStateActive {
		t.Fatalf("legacy ApplicableFrom should allow ACTIVE, got %q ok=%v", state, ok)
	}
}
