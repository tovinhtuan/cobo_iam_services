package app_test

import (
	"context"
	"encoding/json"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	"github.com/cobo/cobo_iam_services/internal/disclosure/infra/inmemory"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

type resolvedRuleTypeDetailRepo struct {
	*inmemory.Repository
	detail   *disclosureapp.DisclosureTypeDTO
	profiles map[string]applicability.CompanyApplicabilityProfile
}

func (r *resolvedRuleTypeDetailRepo) GetTypeDetail(_ context.Context, _, _ string) (*disclosureapp.DisclosureTypeDTO, error) {
	cp := *r.detail
	if r.detail.DeadlineConfig != nil {
		cfg := *r.detail.DeadlineConfig
		cp.DeadlineConfig = &cfg
	}
	if r.detail.ApplicabilityRules != nil {
		rules := *r.detail.ApplicabilityRules
		if r.detail.ApplicabilityRules.DeadlineByStructure != nil {
			rules.DeadlineByStructure = map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{}
			for k, v := range r.detail.ApplicabilityRules.DeadlineByStructure {
				rules.DeadlineByStructure[k] = v
			}
		}
		cp.ApplicabilityRules = &rules
	}
	return &cp, nil
}

func (r *resolvedRuleTypeDetailRepo) GetCompanyApplicabilityProfile(_ context.Context, companyID string) (applicability.CompanyApplicabilityProfile, error) {
	if p, ok := r.profiles[companyID]; ok {
		return p, nil
	}
	return applicability.CompanyApplicabilityProfile{}, nil
}

func qaPeriodicDetail(toggleOn bool, deadlineDays int, structure map[applicability.StructureCriterion]applicability.StructureDeadlineEntry) *disclosureapp.DisclosureTypeDTO {
	return &disclosureapp.DisclosureTypeDTO{
		TypeID:      "qa-monthly-deadline-alert-202607-1785382733",
		Periodicity: "monthly",
		DeadlineConfig: &disclosureapp.TemplateDeadlineConfig{
			DeadlineMode:   disclosureapp.DeadlineModePeriodic,
			DeadlineDays:   deadlineDays,
			FrequencyUnit:  "monthly",
			CycleAnchorDay: 1,
		},
		ApplicabilityRules: &applicability.TemplateApplicabilityRules{
			UseStructureDeadline: toggleOn,
			DeadlineDays:         deadlineDays,
			DeadlineDayType:      "working",
			DeadlineByStructure:  structure,
		},
	}
}

func TestGetTypeDetail_ResolvedDeadlineRule_ToggleOffQA(t *testing.T) {
	ctx := context.Background()
	detail := qaPeriodicDetail(false, 23, map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{
		applicability.StructureHasSubsidiaries:     {Days: 23},
		applicability.StructureHasSubordinateUnits: {Days: 23},
		applicability.StructureSimpleStructure:     {Days: 23},
	})
	repo := &resolvedRuleTypeDetailRepo{
		Repository: inmemory.NewRepository(),
		detail:     detail,
		profiles: map[string]applicability.CompanyApplicabilityProfile{
			"c_001": {HasSubsidiaries: true, HasSubordinateAccountingUnits: true},
		},
	}
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{}, disclosureapp.WithHolidayCalendarProvider(noHolidaysProvider{}))
	got, err := svc.GetTypeDetail(ctx, disclosureapp.GetTypeDetailRequest{
		Subject: disclosureapp.Subject{UserID: "u1", MembershipID: "m1", CompanyID: "c_001"},
		TypeID:  detail.TypeID,
	})
	if err != nil {
		t.Fatalf("GetTypeDetail: %v", err)
	}
	rule := got.ResolvedDeadlineRule
	if rule == nil {
		t.Fatal("expected resolved_deadline_rule")
	}
	if rule.RuleCode != applicability.RuleCodeDefault {
		t.Fatalf("rule_code=%s want DEFAULT", rule.RuleCode)
	}
	if rule.ResolutionSource != applicability.ResolutionSourceDefaultTemplateRule {
		t.Fatalf("source=%s", rule.ResolutionSource)
	}
	if rule.ResolvedDays == nil || *rule.ResolvedDays != 23 {
		t.Fatalf("days=%v", rule.ResolvedDays)
	}
	if rule.DayType != "WORKING_DAYS" {
		t.Fatalf("day_type=%s", rule.DayType)
	}
	if rule.BaseDateSource != disclosureapp.BaseDateSourceCycleStart {
		t.Fatalf("base=%s", rule.BaseDateSource)
	}
	if rule.Periodicity != "monthly" {
		t.Fatalf("periodicity=%s", rule.Periodicity)
	}
	if got.DeadlineSummary == nil || got.DeadlineSummary.DeadlineDate == nil {
		t.Fatal("expected deadline_summary.deadline_date for due_date attach")
	}
	if rule.DueDate == nil || *rule.DueDate != *got.DeadlineSummary.DeadlineDate {
		t.Fatalf("due_date=%v summary=%v", rule.DueDate, got.DeadlineSummary.DeadlineDate)
	}
	// Existing fields preserved
	if got.ApplicabilityRules == nil || got.DeadlineSummary == nil {
		t.Fatal("existing fields missing")
	}
}

func TestGetTypeDetail_ResolvedDeadlineRule_StructureOverridesAndIsolation(t *testing.T) {
	ctx := context.Background()
	structure := map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{
		applicability.StructureHasSubsidiaries:     {Days: 30},
		applicability.StructureHasSubordinateUnits: {Days: 25},
		applicability.StructureSimpleStructure:     {Days: 15},
	}
	detail := qaPeriodicDetail(true, 20, structure)
	repo := &resolvedRuleTypeDetailRepo{
		Repository: inmemory.NewRepository(),
		detail:     detail,
		profiles: map[string]applicability.CompanyApplicabilityProfile{
			"co-a": {HasSubsidiaries: true, HasSubordinateAccountingUnits: true},
			"co-b": {HasSubordinateAccountingUnits: true},
		},
	}
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{}, disclosureapp.WithHolidayCalendarProvider(noHolidaysProvider{}))

	gotA, err := svc.GetTypeDetail(ctx, disclosureapp.GetTypeDetailRequest{
		Subject: disclosureapp.Subject{CompanyID: "co-a", UserID: "u", MembershipID: "m"},
		TypeID:  detail.TypeID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotA.ResolvedDeadlineRule.RuleCode != string(applicability.StructureHasSubsidiaries) {
		t.Fatalf("co-a code=%s", gotA.ResolvedDeadlineRule.RuleCode)
	}
	if gotA.ResolvedDeadlineRule.ResolutionSource != applicability.ResolutionSourceStructureOverride {
		t.Fatalf("co-a source=%s", gotA.ResolvedDeadlineRule.ResolutionSource)
	}
	if gotA.ResolvedDeadlineRule.ResolvedDays == nil || *gotA.ResolvedDeadlineRule.ResolvedDays != 30 {
		t.Fatalf("co-a days=%v", gotA.ResolvedDeadlineRule.ResolvedDays)
	}

	gotB, err := svc.GetTypeDetail(ctx, disclosureapp.GetTypeDetailRequest{
		Subject: disclosureapp.Subject{CompanyID: "co-b", UserID: "u", MembershipID: "m"},
		TypeID:  detail.TypeID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotB.ResolvedDeadlineRule.RuleCode != string(applicability.StructureHasSubordinateUnits) {
		t.Fatalf("co-b code=%s", gotB.ResolvedDeadlineRule.RuleCode)
	}
	if gotB.ResolvedDeadlineRule.ResolvedDays == nil || *gotB.ResolvedDeadlineRule.ResolvedDays != 25 {
		t.Fatalf("co-b days=%v", gotB.ResolvedDeadlineRule.ResolvedDays)
	}
}

func TestGetTypeDetail_ResolvedDeadlineRule_StructureFallbackAndJSON(t *testing.T) {
	ctx := context.Background()
	detail := qaPeriodicDetail(true, 23, map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{
		applicability.StructureHasSubordinateUnits: {Days: 25},
	})
	repo := &resolvedRuleTypeDetailRepo{
		Repository: inmemory.NewRepository(),
		detail:     detail,
		profiles: map[string]applicability.CompanyApplicabilityProfile{
			"c_001": {HasSubsidiaries: true},
		},
	}
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{}, disclosureapp.WithHolidayCalendarProvider(noHolidaysProvider{}))
	got, err := svc.GetTypeDetail(ctx, disclosureapp.GetTypeDetailRequest{
		Subject: disclosureapp.Subject{CompanyID: "c_001", UserID: "u", MembershipID: "m"},
		TypeID:  detail.TypeID,
	})
	if err != nil {
		t.Fatal(err)
	}
	rule := got.ResolvedDeadlineRule
	if rule.ResolutionSource != applicability.ResolutionSourceStructureFallbackDefault || rule.RuleCode != applicability.RuleCodeDefault {
		t.Fatalf("got %+v", rule)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	obj, ok := envelope["resolved_deadline_rule"].(map[string]any)
	if !ok {
		t.Fatalf("missing resolved_deadline_rule in JSON: %s", string(raw))
	}
	if obj["resolution_source"] != applicability.ResolutionSourceStructureFallbackDefault {
		t.Fatalf("json source=%v", obj["resolution_source"])
	}
	if obj["rule_code"] != applicability.RuleCodeDefault {
		t.Fatalf("json code=%v", obj["rule_code"])
	}
	if _, hasDeadlineSummary := envelope["deadline_summary"]; !hasDeadlineSummary {
		t.Fatal("deadline_summary missing — backward compat break")
	}
}

func TestBuildResolvedDeadlineRuleDTO_NoDueWithoutSummary(t *testing.T) {
	// Package-level helper coverage via GetTypeDetail with DeadlineDays<=0 → summary nil path.
	ctx := context.Background()
	detail := qaPeriodicDetail(false, 0, nil)
	detail.DeadlineConfig.DeadlineDays = 0
	repo := &resolvedRuleTypeDetailRepo{
		Repository: inmemory.NewRepository(),
		detail:     detail,
		profiles:   map[string]applicability.CompanyApplicabilityProfile{"c": {}},
	}
	svc := disclosureapp.NewService(repo, nil, idgen.UUIDv7Generator{}, disclosureapp.WithHolidayCalendarProvider(noHolidaysProvider{}))
	got, err := svc.GetTypeDetail(ctx, disclosureapp.GetTypeDetailRequest{
		Subject: disclosureapp.Subject{CompanyID: "c", UserID: "u", MembershipID: "m"},
		TypeID:  detail.TypeID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ResolvedDeadlineRule == nil {
		t.Fatal("expected NO_RULE dto")
	}
	if got.ResolvedDeadlineRule.ResolutionSource != applicability.ResolutionSourceNoRule {
		t.Fatalf("source=%s", got.ResolvedDeadlineRule.ResolutionSource)
	}
	if got.ResolvedDeadlineRule.DueDate != nil {
		t.Fatalf("due_date must be null when no summary: %v", got.ResolvedDeadlineRule.DueDate)
	}
	if got.ResolvedDeadlineRule.BaseDateSource != disclosureapp.BaseDateSourceCycleStart {
		t.Fatalf("PERIODIC still reports CYCLE_START: %s", got.ResolvedDeadlineRule.BaseDateSource)
	}
}
