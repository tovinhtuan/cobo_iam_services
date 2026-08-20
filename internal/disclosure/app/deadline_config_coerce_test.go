package app

import (
	"testing"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

func TestCoercePeriodicDeadlineEngineMode_PromotesNoneWhenApplicabilityHasDays(t *testing.T) {
	item := &DisclosureTypeDTO{
		TemplateCategory: TemplateCategoryPeriodic,
		Periodicity:      "quarterly",
		ApplicabilityRules: &applicability.TemplateApplicabilityRules{
			DeadlineDays: 30,
		},
		DeadlineConfig: &TemplateDeadlineConfig{
			DeadlineMode: DeadlineModeNone,
		},
	}
	coercePeriodicDeadlineEngineMode(item)
	if item.DeadlineConfig.DeadlineMode != DeadlineModePeriodic {
		t.Fatalf("expected PERIODIC, got %s", item.DeadlineConfig.DeadlineMode)
	}
	if item.DeadlineConfig.DeadlineDays != 30 {
		t.Fatalf("expected deadline_days=30, got %d", item.DeadlineConfig.DeadlineDays)
	}
}

func TestCoercePeriodicDeadlineEngineMode_LeavesIrregularUntouched(t *testing.T) {
	item := &DisclosureTypeDTO{
		TemplateCategory: TemplateCategoryIrregular,
		Periodicity:      "event_based",
		ApplicabilityRules: &applicability.TemplateApplicabilityRules{
			DeadlineDays: 30,
		},
		DeadlineConfig: &TemplateDeadlineConfig{
			DeadlineMode: DeadlineModeNone,
		},
	}
	coercePeriodicDeadlineEngineMode(item)
	if item.DeadlineConfig.DeadlineMode != DeadlineModeNone {
		t.Fatalf("expected NONE for irregular, got %s", item.DeadlineConfig.DeadlineMode)
	}
}
