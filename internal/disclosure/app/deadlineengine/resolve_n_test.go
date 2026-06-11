package deadlineengine

import (
	"errors"
	"testing"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

func TestResolveEffectiveN_ToggleOff(t *testing.T) {
	rules := &Rules{
		DeadlineDays:         20,
		UseStructureDeadline: false,
	}
	n, err := ResolveEffectiveN(rules, applicability.CompanyApplicabilityProfile{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 20 {
		t.Fatalf("expected 20, got %d", n)
	}
}

func TestResolveEffectiveN_ToggleOff_InvalidDeadlineDays(t *testing.T) {
	rules := &Rules{
		DeadlineDays:         0,
		UseStructureDeadline: false,
	}
	_, err := ResolveEffectiveN(rules, applicability.CompanyApplicabilityProfile{})
	if !errors.Is(err, ErrInvalidDeadlineDays) {
		t.Fatalf("expected ErrInvalidDeadlineDays, got %v", err)
	}
}

func TestResolveEffectiveN_ToggleOn_StructureHit(t *testing.T) {
	rules := &Rules{
		DeadlineDays:         20,
		UseStructureDeadline: true,
		DeadlineByStructure: map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{
			applicability.StructureHasSubsidiaries:    {Days: 30},
			applicability.StructureHasSubordinateUnits: {Days: 25},
			applicability.StructureSimpleStructure:     {Days: 20},
		},
	}
	profile := applicability.CompanyApplicabilityProfile{HasSubsidiaries: true}

	n, used, err := resolveEffectiveN(rules, profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !used {
		t.Fatalf("expected usedStructure=true")
	}
	if n != 30 {
		t.Fatalf("expected 30, got %d", n)
	}
}

func TestResolveEffectiveN_ToggleOn_StructureMiss_FallbackDeadlineDays(t *testing.T) {
	rules := &Rules{
		DeadlineDays:         20,
		UseStructureDeadline: true,
		DeadlineByStructure:  map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{
			// no entries at all -> miss for any criterion
		},
	}
	profile := applicability.CompanyApplicabilityProfile{HasSubsidiaries: true}

	n, used, err := resolveEffectiveN(rules, profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if used {
		t.Fatalf("expected usedStructure=false (fallback)")
	}
	if n != 20 {
		t.Fatalf("expected fallback 20, got %d", n)
	}
}

func TestResolveEffectiveN_ToggleOn_StructureMiss_DeadlineDaysAlsoInvalid_E11(t *testing.T) {
	rules := &Rules{
		DeadlineDays:         0,
		UseStructureDeadline: true,
		DeadlineByStructure:  map[applicability.StructureCriterion]applicability.StructureDeadlineEntry{},
	}
	profile := applicability.CompanyApplicabilityProfile{HasSubsidiaries: true}

	_, _, err := resolveEffectiveN(rules, profile)
	if !errors.Is(err, ErrStructureDeadlineUnresolved) {
		t.Fatalf("expected ErrStructureDeadlineUnresolved, got %v", err)
	}
}

func TestResolveEffectiveN_NilRules(t *testing.T) {
	_, err := ResolveEffectiveN(nil, applicability.CompanyApplicabilityProfile{})
	if !errors.Is(err, ErrRulesRequired) {
		t.Fatalf("expected ErrRulesRequired, got %v", err)
	}
}
