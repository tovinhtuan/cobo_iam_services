package app

import (
	"context"
	"testing"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

type filterTestRepo struct {
	Repository
	profile applicability.CompanyApplicabilityProfile
}

func (r *filterTestRepo) GetCompanyApplicabilityProfile(_ context.Context, _ string) (applicability.CompanyApplicabilityProfile, error) {
	return r.profile, nil
}

func TestFilterTypesByApplicability_PF1_PF6(t *testing.T) {
	sector := applicability.BusinessSectorCommercial
	svc := &service{
		repo: &filterTestRepo{
			profile: applicability.CompanyApplicabilityProfile{
				IsListed:       true,
				BusinessSector: &sector,
			},
		},
		templateApplicabilityStrictFilter: true,
	}
	rules := &applicability.TemplateApplicabilityRules{
		ApplicableCompanyClasses: []applicability.CompanyClass{applicability.CompanyClassListed},
		ApplicableSectors:        []applicability.BusinessSector{applicability.BusinessSectorCommercial},
		UseStructureDeadline:     true,
		DeadlineByStructure: map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{
			applicability.StructureSimpleStructure: {Days: 20},
		},
	}
	items := []DisclosureTypeSummaryDTO{
		{TypeID: "global-1", Scope: "global", ApplicabilityRules: rules},
		{TypeID: "company-1", Scope: "company"},
	}
	out, err := svc.filterTypesByApplicability(context.Background(), "c1", items)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
	if out[0].ResolvedStructureDeadlineDays == nil || *out[0].ResolvedStructureDeadlineDays != 20 {
		t.Fatalf("resolved days=%v", out[0].ResolvedStructureDeadlineDays)
	}
}

func TestFilterTypesByApplicability_PF2(t *testing.T) {
	svc := &service{
		repo: &filterTestRepo{
			profile: applicability.CompanyApplicabilityProfile{},
		},
		templateApplicabilityStrictFilter: true,
	}
	rules := &applicability.TemplateApplicabilityRules{
		ApplicableCompanyClasses: []applicability.CompanyClass{applicability.CompanyClassListed},
		ApplicableSectors:        []applicability.BusinessSector{applicability.BusinessSectorCommercial},
	}
	items := []DisclosureTypeSummaryDTO{{TypeID: "g1", Scope: "global", ApplicabilityRules: rules}}
	out, err := svc.filterTypesByApplicability(context.Background(), "c1", items)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("expected hidden, got %d", len(out))
	}
}
