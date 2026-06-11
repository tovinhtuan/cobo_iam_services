package applicability

import "testing"

func TestIsApplicable_ORClassMatch(t *testing.T) {
	rules := &TemplateApplicabilityRules{
		ApplicableCompanyClasses: []CompanyClass{CompanyClassListed},
		ApplicableSectors:        []BusinessSector{BusinessSectorCommercial},
	}
	sector := BusinessSectorCommercial
	profile := CompanyApplicabilityProfile{
		IsListed: true, IsLargePublic: true, BusinessSector: &sector,
	}
	if !IsApplicable(rules, profile, true) {
		t.Fatal("expected visible when listed matches OR")
	}
}

func TestIsApplicable_NoSectorHidden(t *testing.T) {
	rules := &TemplateApplicabilityRules{
		ApplicableCompanyClasses: []CompanyClass{CompanyClassListed},
		ApplicableSectors:        []BusinessSector{BusinessSectorCommercial},
	}
	profile := CompanyApplicabilityProfile{IsListed: true, BusinessSector: nil}
	if IsApplicable(rules, profile, true) {
		t.Fatal("expected hidden when sector null")
	}
}

func TestResolveStructure_Priority(t *testing.T) {
	p := CompanyApplicabilityProfile{HasSubsidiaries: true, HasSubordinateAccountingUnits: true}
	if got := ResolveStructure(p); got != StructureHasSubsidiaries {
		t.Fatalf("got %s want has_subsidiaries", got)
	}
	p = CompanyApplicabilityProfile{HasSubordinateAccountingUnits: true}
	if got := ResolveStructure(p); got != StructureHasSubordinateUnits {
		t.Fatalf("got %s want has_subordinate_units", got)
	}
	p = CompanyApplicabilityProfile{}
	if got := ResolveStructure(p); got != StructureSimpleStructure {
		t.Fatalf("got %s want simple_structure", got)
	}
}

func TestResolveDeadlineDays(t *testing.T) {
	rules := &TemplateApplicabilityRules{
		DeadlineByStructure: map[StructureCriterion]StructureDeadlineEntry{
			StructureHasSubsidiaries:     {Days: 30},
			StructureHasSubordinateUnits: {Days: 30},
			StructureSimpleStructure:     {Days: 20},
		},
	}
	days, ok := ResolveDeadlineDays(rules, CompanyApplicabilityProfile{HasSubsidiaries: true})
	if !ok || days != 30 {
		t.Fatalf("days=%d ok=%v", days, ok)
	}
	days, ok = ResolveDeadlineDays(rules, CompanyApplicabilityProfile{})
	if !ok || days != 20 {
		t.Fatalf("days=%d ok=%v", days, ok)
	}
}

func TestGraceNullRules(t *testing.T) {
	sector := BusinessSectorCommercial
	profile := CompanyApplicabilityProfile{IsListed: true, BusinessSector: &sector}
	if !IsApplicable(nil, profile, false) {
		t.Fatal("null rules should pass when strict filter off")
	}
	if IsApplicable(nil, profile, true) {
		t.Fatal("null rules should fail when strict filter on")
	}
}
