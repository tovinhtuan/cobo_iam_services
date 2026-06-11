// Package deadlineengine implements Deadline Engine V2 deadline resolution
// (planned_date = AddDays(ResolveT0(...), ResolveEffectiveN(...), day_type)).
//
// This package is intentionally isolated: it does not import app/service.go,
// app/periodic.go, or any transport package. It is not wired into any runtime
// path yet (Batch 5).
package deadlineengine

import (
	"context"
	"time"

	"github.com/cobo/cobo_iam_services/internal/disclosure/app/applicability"
)

// DayType is the unit AddDays counts in.
type DayType string

const (
	DayTypeCalendar DayType = "calendar"
	DayTypeWorking  DayType = "working"
)

// DeadlineBehavior is the resolved engine behavior for a template.
//
// PD-DE-CUSTOM-01: "custom" is NOT a third deadline behavior — it is a
// template_category (ownership: company-created vs platform/global) that is
// orthogonal to deadline behavior. Every template, regardless of
// TemplateCategory ("periodic" | "irregular" | "custom"), resolves to exactly
// one of the two behaviors below via DeriveDeadlineBehavior, based on
// T0Config.FrequencyUnit / T0Policy shape — never on TemplateCategory.
type DeadlineBehavior string

const (
	DeadlineBehaviorPeriodic  DeadlineBehavior = "periodic"
	DeadlineBehaviorIrregular DeadlineBehavior = "irregular"
)

// Rules mirrors applicability.TemplateApplicabilityRules. Used directly
// (type alias) per Pre-Implementation Validation Review Section 2.1 —
// no duplicate struct, no field-by-field mapping/drift risk.
type Rules = applicability.TemplateApplicabilityRules

// TemplateInput is the subset of template configuration the engine needs.
//
// TemplateCategory is informational only ("periodic" | "irregular" | "custom").
// It MUST NOT be used to branch R-P/R-I — use DeriveDeadlineBehavior(T0Config)
// instead (PD-DE-CUSTOM-01).
type TemplateInput struct {
	TemplateCategory string
	T0Config         T0ConfigInput
	Rules            *Rules
}

// T0ConfigInput is mapped from TemplateDeadlineConfig.
type T0ConfigInput struct {
	// T0Policy: "system_date" | "user_defined" | "event_date" — used when
	// FrequencyUnit is empty (irregular behavior).
	T0Policy string
	// FrequencyUnit: "monthly" | "quarterly" | "yearly" — when set to one of
	// these values, the template resolves to periodic behavior regardless of
	// TemplateCategory (PD-DE-CUSTOM-01, Case 3).
	FrequencyUnit    string
	CycleAnchorMonth int
	CycleAnchorDay   int
	// AllowT0Override gates company T0 override (R-C). The caller (mapping
	// layer, Batch 5) MUST map a nil/missing config flag to false:
	//   AllowT0Override = config.AllowT0Override != nil && *config.AllowT0Override
	// nil/missing is treated as false (safe default — Platform Admin must opt in).
	AllowT0Override bool
}

// CompanyInput is the company-side data for R-C (T0 override) and N
// (structure) resolution.
type CompanyInput struct {
	// CycleAnchorMonth/CycleAnchorDay are field-level overrides from
	// company_type_preferences. 0 = no override for that field; >0 = override
	// that field independently (month and day need not both be set).
	CycleAnchorMonth int
	CycleAnchorDay   int
	Profile          applicability.CompanyApplicabilityProfile
}

// PeriodicCycleContext carries an already-resolved cycle start (e.g. from a
// materialized periodic_cycles row, I-21). When CycleStart is non-zero,
// ResolveT0 short-circuits and returns it as-is.
type PeriodicCycleContext struct {
	CycleStart time.Time
	CycleLabel string
}

// RuntimeInput is runtime-supplied data for irregular/manual creation.
type RuntimeInput struct {
	// T0Date is YYYY-MM-DD, required when T0Policy == "user_defined".
	T0Date string
}

// Resolution is the output of ResolveDeadline.
type Resolution struct {
	ResolvedT0            time.Time
	EffectiveDeadlineDays int
	EffectiveDayType      DayType
	PlannedDate           time.Time
	UsedStructureDeadline bool
	UsedCompanyT0Override bool
	HintLabelVI           string
}

// NonTradingDayChecker reports whether a date is a holiday (weekends are
// handled internally by AddDays and are not part of this interface). A nil
// checker means "no holiday calendar configured" — only weekends are skipped.
type NonTradingDayChecker interface {
	IsHoliday(ctx context.Context, date time.Time) (bool, error)
}
