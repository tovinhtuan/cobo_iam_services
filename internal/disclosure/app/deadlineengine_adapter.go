package app

import (
	"context"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
	"github.com/cobo/cobo_iam_services/internal/disclosure/app/deadlineengine"
)

// DeadlineEngineAdapterInput bundles everything deadlineengine.ResolveDeadline
// (Source C — the locked Cycle Start SoT, see
// deadline-engine-cycle-start-sot-review-2026-06-12.md) needs, mapped from the
// existing v1 data shapes (TemplateDeadlineConfig, CompanyDeadlineContext,
// applicability rules/profile). Batch 5B-5E call sites construct this from
// data they already read today — no new repository queries required.
type DeadlineEngineAdapterInput struct {
	// TemplateCategory is informational only (PD-DE-CUSTOM-01) — never used to
	// branch R-P/R-I, but threaded through for parity with TemplateInput.
	TemplateCategory string
	// Config is the template's deadline_config_json shape (T0Config source).
	Config *TemplateDeadlineConfig
	// Rules carries deadline_days/deadline_day_type/structure overrides (N
	// source, ADR-06). Required — nil returns ErrRulesRequired.
	Rules *applicability.TemplateApplicabilityRules
	// Company carries the per-company cycle_anchor_* override (R-C input).
	Company CompanyDeadlineContext
	// CompanyProfile is the company self-declaration used for structure-based
	// N resolution (ResolveEffectiveN).
	CompanyProfile applicability.CompanyApplicabilityProfile
	// CycleCtx carries an already-resolved cycle_start (I-21 short-circuit),
	// e.g. from a materialized periodic_cycles row. nil = no existing cycle.
	CycleCtx *deadlineengine.PeriodicCycleContext
	// Runtime carries t0_date for t0_policy=user_defined (manual create path).
	Runtime deadlineengine.RuntimeInput
	// RefTime is "now". Zero value defaults to time.Now().
	RefTime time.Time
}

// DeadlineEngineAdapter is the single Batch 5B-5E integration point for
// Deadline Engine V2 (Source C / deadlineengine.ResolveDeadline — the locked
// Cycle Start SoT).
//
// Implementations MUST delegate to deadlineengine.ResolveDeadline /
// deadlineengine.ResolveT0 and MUST NOT reimplement cycle_start, AddDays, or
// N-resolution logic (no copy/paste from deadline_calculator.go or
// periodic.go — see deadline-engine-cycle-start-sot-review-2026-06-12.md).
//
// Batch 5A note: this adapter is additive-only. It is not called from any
// runtime path yet (Portal Preview, Manual Create, Periodic Worker, Reminder
// all remain on their current implementations — see READY_FOR_5B markers).
type DeadlineEngineAdapter interface {
	ResolveDeadline(ctx context.Context, in DeadlineEngineAdapterInput) (deadlineengine.Resolution, error)
}

type deadlineEngineAdapter struct {
	holidays HolidayCalendarProvider
}

// NewDeadlineEngineAdapter builds a DeadlineEngineAdapter backed by the given
// holiday calendar provider (may be nil — only weekends are skipped, mirrors
// DeadlineCalculator's existing nil-holidays behavior).
func NewDeadlineEngineAdapter(holidays HolidayCalendarProvider) DeadlineEngineAdapter {
	return &deadlineEngineAdapter{holidays: holidays}
}

func (a *deadlineEngineAdapter) ResolveDeadline(ctx context.Context, in DeadlineEngineAdapterInput) (deadlineengine.Resolution, error) {
	if in.Config == nil {
		return deadlineengine.Resolution{}, ErrMissingDeadlineConfig
	}

	allowOverride := in.Config.AllowT0Override != nil && *in.Config.AllowT0Override

	template := deadlineengine.TemplateInput{
		TemplateCategory: in.TemplateCategory,
		T0Config: deadlineengine.T0ConfigInput{
			T0Policy:         in.Config.T0Policy,
			FrequencyUnit:    in.Config.FrequencyUnit,
			CycleAnchorMonth: in.Config.CycleAnchorMonth,
			CycleAnchorDay:   in.Config.CycleAnchorDay,
			AllowT0Override:  allowOverride,
		},
		Rules: normalizeRulesForEngine(in.Rules),
	}

	company := deadlineengine.CompanyInput{
		CycleAnchorMonth: in.Company.CycleAnchorMonth,
		CycleAnchorDay:   in.Company.CycleAnchorDay,
		Profile:          in.CompanyProfile,
	}

	refTime := in.RefTime
	if refTime.IsZero() {
		refTime = time.Now()
	}

	var holidays deadlineengine.NonTradingDayChecker
	if a.holidays != nil {
		holidays = &nonTradingDayCheckerAdapter{provider: a.holidays}
	}

	return deadlineengine.ResolveDeadline(ctx, template, company, in.CycleCtx, in.Runtime, refTime, holidays)
}

// normalizeRulesForEngine adapts legacy applicability_rules_json (authored
// before the V2 use_structure_deadline flag existed) to Source C's
// N-resolution contract (resolve_n.go / ResolveEffectiveN).
//
// CMS validation (E01-E06, applicability/validate.go) requires every periodic
// template to populate deadline_by_structure, but deadline_days and
// use_structure_deadline are new V2-only fields that existing DEV/prod rows
// leave at their zero values (0 / false). ResolveEffectiveN treats
// DeadlineDays<=0 && !UseStructureDeadline as ErrInvalidDeadlineDays, even
// though deadline_by_structure is fully populated.
//
// The legacy reader (applicability.ResolveDeadlineDays, used by Source A/B)
// consults deadline_by_structure unconditionally whenever it is populated —
// there is no opt-in flag on that path. Mirror that behavior here: when
// deadline_days is unset/zero, use_structure_deadline is false, and
// deadline_by_structure is non-empty, opt into the structure-based N path so
// Source C resolves the same N that Source A/B already derive from this row.
// This is read-only normalization of the adapter input — no DB write, no
// mutation of the caller's rules value.
func normalizeRulesForEngine(rules *applicability.TemplateApplicabilityRules) *applicability.TemplateApplicabilityRules {
	if rules == nil {
		return nil
	}
	if rules.DeadlineDays <= 0 && !rules.UseStructureDeadline && len(rules.DeadlineByStructure) > 0 {
		cp := *rules
		cp.UseStructureDeadline = true
		return &cp
	}
	return rules
}

// nonTradingDayCheckerAdapter adapts the existing app.HolidayCalendarProvider
// (IsNonTradingDay, returns a reason string) to deadlineengine's
// NonTradingDayChecker (IsHoliday, bool only). Weekend handling is performed
// independently by both deadlineengine.AddDays and HolidayCalendarProvider
// implementations, so no double-counting occurs.
type nonTradingDayCheckerAdapter struct {
	provider HolidayCalendarProvider
}

func (h *nonTradingDayCheckerAdapter) IsHoliday(ctx context.Context, date time.Time) (bool, error) {
	isNonTrading, _, err := h.provider.IsNonTradingDay(ctx, date)
	return isNonTrading, err
}
