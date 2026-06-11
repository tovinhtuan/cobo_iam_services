package deadlineengine

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ResolveDeadline is the orchestrator:
//
//	planned_date = AddDays(ResolveT0(...), ResolveEffectiveN(...), day_type)
//
// Inclusive counting (ADR-02): T0 = day #1 of N.
func ResolveDeadline(
	ctx context.Context,
	template TemplateInput,
	company CompanyInput,
	cycleCtx *PeriodicCycleContext,
	runtime RuntimeInput,
	refTime time.Time,
	holidays NonTradingDayChecker,
) (Resolution, error) {
	if template.Rules == nil {
		return Resolution{}, ErrRulesRequired
	}

	t0, usedOverride, err := resolveT0(ctx, template, company, cycleCtx, runtime, refTime)
	if err != nil {
		return Resolution{}, err
	}

	n, usedStructure, err := resolveEffectiveN(template.Rules, company.Profile)
	if err != nil {
		if errors.Is(err, ErrStructureDeadlineUnresolved) {
			return Resolution{}, fmt.Errorf("%w: %w", ErrDeadlineRuleUnresolved, err)
		}
		return Resolution{}, err
	}

	dayType := DayType(template.Rules.DeadlineDayType)
	if dayType == "" {
		dayType = DayTypeCalendar
	}

	planned, err := AddDays(ctx, t0, n, dayType, holidays)
	if err != nil {
		return Resolution{}, err
	}

	return Resolution{
		ResolvedT0:            t0,
		EffectiveDeadlineDays: n,
		EffectiveDayType:      dayType,
		PlannedDate:           planned,
		UsedStructureDeadline: usedStructure,
		UsedCompanyT0Override: usedOverride,
		HintLabelVI:           FormatHintLabelVI(t0, n, dayType, planned, usedStructure, usedOverride),
	}, nil
}
