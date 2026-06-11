package deadlineengine

import (
	"context"
	"fmt"
	"time"
)

// DeriveDeadlineBehavior derives the deadline resolution behavior (R-P vs R-I)
// from the template's T0 config shape.
//
// PD-DE-CUSTOM-01 (locked): TemplateCategory ("periodic" | "irregular" |
// "custom") is NEVER used here. "custom" is a template_category (ownership:
// company-created vs platform/global), orthogonal to deadline behavior.
// Behavior is derived purely from FrequencyUnit / T0Policy:
//
//	frequency_unit ∈ {monthly, quarterly, yearly} -> periodic, regardless of
//	  TemplateCategory (covers "custom" + frequency_unit, e.g. Case 3).
//	frequency_unit == "" && t0_policy != ""        -> irregular (covers
//	  "custom" + t0_policy, e.g. Case 4).
//	frequency_unit == "" && t0_policy == ""        -> ErrInvalidT0Configuration
//	  (Case 5 — never silently default to periodic or irregular).
//	frequency_unit == anything else                -> ErrUnsupportedFrequency.
func DeriveDeadlineBehavior(template TemplateInput) (DeadlineBehavior, error) {
	switch template.T0Config.FrequencyUnit {
	case "monthly", "quarterly", "yearly":
		return DeadlineBehaviorPeriodic, nil
	case "":
		if template.T0Config.T0Policy == "" {
			return "", fmt.Errorf("%w: frequency_unit and t0_policy are both empty", ErrInvalidT0Configuration)
		}
		return DeadlineBehaviorIrregular, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnsupportedFrequency, template.T0Config.FrequencyUnit)
	}
}

// ResolveT0 resolves T0 (the day #1 anchor for AddDays).
func ResolveT0(
	ctx context.Context,
	template TemplateInput,
	company CompanyInput,
	cycleCtx *PeriodicCycleContext,
	runtime RuntimeInput,
	refTime time.Time,
) (time.Time, error) {
	t0, _, err := resolveT0(ctx, template, company, cycleCtx, runtime, refTime)
	return t0, err
}

// resolveT0 is the internal core. The bool return reports whether the
// company T0 override (R-C) was applied — needed by ResolveDeadline for
// Resolution.UsedCompanyT0Override / hint suffix.
func resolveT0(
	_ context.Context,
	template TemplateInput,
	company CompanyInput,
	cycleCtx *PeriodicCycleContext,
	runtime RuntimeInput,
	refTime time.Time,
) (time.Time, bool, error) {
	behavior, err := DeriveDeadlineBehavior(template)
	if err != nil {
		return time.Time{}, false, err
	}

	switch behavior {
	case DeadlineBehaviorPeriodic:
		return resolvePeriodicT0(template, company, cycleCtx, refTime)
	case DeadlineBehaviorIrregular:
		return resolveIrregularT0(template, runtime, refTime)
	default:
		// Unreachable — DeriveDeadlineBehavior only returns the two cases above.
		return time.Time{}, false, fmt.Errorf("%w: unknown deadline behavior %q", ErrInvalidT0Configuration, behavior)
	}
}

// resolvePeriodicT0 implements R-P (+ R-C company override).
//
// Company T0 override (R-C) applies whenever behavior=periodic AND
// AllowT0Override=true — independent of TemplateCategory (PD-DE-CUSTOM-01).
// "custom" + frequency_unit=monthly is eligible for override exactly like
// "periodic" + frequency_unit=monthly.
func resolvePeriodicT0(
	template TemplateInput,
	company CompanyInput,
	cycleCtx *PeriodicCycleContext,
	refTime time.Time,
) (time.Time, bool, error) {
	// I-21 short-circuit: an already-resolved cycle start (e.g. from a
	// materialized periodic_cycles row) is taken as-is.
	if cycleCtx != nil && !cycleCtx.CycleStart.IsZero() {
		return cycleCtx.CycleStart, false, nil
	}

	anchorMonth := template.T0Config.CycleAnchorMonth
	anchorDay := template.T0Config.CycleAnchorDay
	usedOverride := false

	if template.T0Config.AllowT0Override {
		if company.CycleAnchorMonth > 0 {
			anchorMonth = company.CycleAnchorMonth
			usedOverride = true
		}
		if company.CycleAnchorDay > 0 {
			anchorDay = company.CycleAnchorDay
			usedOverride = true
		}
	}

	effMonth, effDay, err := normalizeAnchor(anchorMonth, anchorDay, template.T0Config.FrequencyUnit, refTime)
	if err != nil {
		return time.Time{}, false, err
	}

	cycleStart := computeCycleStart(effMonth, effDay, template.T0Config.FrequencyUnit, refTime)
	return cycleStart, usedOverride, nil
}

// resolveIrregularT0 implements R-I.
func resolveIrregularT0(
	template TemplateInput,
	runtime RuntimeInput,
	refTime time.Time,
) (time.Time, bool, error) {
	switch template.T0Config.T0Policy {
	case "system_date":
		return stripTime(refTime), false, nil
	case "user_defined":
		if runtime.T0Date == "" {
			return time.Time{}, false, ErrT0DateRequired
		}
		t, err := time.ParseInLocation("2006-01-02", runtime.T0Date, refTime.Location())
		if err != nil {
			return time.Time{}, false, fmt.Errorf("%w: %v", ErrInvalidT0Date, err)
		}
		return t, false, nil
	case "event_date":
		return time.Time{}, false, ErrEventDateNotImplemented
	default:
		return time.Time{}, false, fmt.Errorf("%w: unsupported t0_policy %q", ErrInvalidT0Configuration, template.T0Config.T0Policy)
	}
}

// normalizeAnchor validates (cycle_anchor_month, cycle_anchor_day) and returns
// the effective 1-12 / 1-31 values used by computeCycleStart.
//
//   - 0 means "unset" and defaults to 1 — NOT an error.
//   - Out-of-range month (<0 or >12) or day (<0 or >31) is an error.
//   - A day that does not exist in the month it will actually be applied to
//     — e.g. month=2, day=31 — is an error (ErrInvalidT0Configuration). Go's
//     time.Date would otherwise silently roll over into the next month.
//
// The "month it will actually be applied to" depends on frequencyUnit, per
// computeCycleStart:
//   - monthly:   anchor_day is applied to refTime's current month
//   - yearly:    anchor_day is applied to anchor_month
//   - quarterly: anchor_day is not used at all (always day 1) — no day check
func normalizeAnchor(month, day int, frequencyUnit string, refTime time.Time) (int, int, error) {
	if month < 0 || month > 12 {
		return 0, 0, fmt.Errorf("%w: cycle_anchor_month=%d out of range", ErrInvalidT0Configuration, month)
	}
	if day < 0 || day > 31 {
		return 0, 0, fmt.Errorf("%w: cycle_anchor_day=%d out of range", ErrInvalidT0Configuration, day)
	}

	effMonth := month
	if effMonth == 0 {
		effMonth = 1
	}
	effDay := day
	if effDay == 0 {
		effDay = 1
	}

	var dayContextMonth time.Month
	var dayContextYear int
	switch frequencyUnit {
	case "monthly":
		dayContextMonth = refTime.Month()
		dayContextYear = refTime.Year()
	case "yearly":
		dayContextMonth = time.Month(effMonth)
		dayContextYear = refTime.Year()
	default:
		// quarterly (and any other periodic frequency): anchor_day is not
		// used by computeCycleStart — skip day-of-month validation.
		return effMonth, effDay, nil
	}

	daysInMonth := time.Date(dayContextYear, dayContextMonth+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if effDay > daysInMonth {
		return 0, 0, fmt.Errorf("%w: cycle_anchor_day=%d invalid for month=%d/%d", ErrInvalidT0Configuration, day, int(dayContextMonth), dayContextYear)
	}

	return effMonth, effDay, nil
}

// computeCycleStart returns the start date of the current period for periodic
// templates. anchorMonth/anchorDay are pre-validated/normalized (1-12 / valid
// day-of-month) by normalizeAnchor.
//
// Ported 1:1 from app.DeadlineCalculator.computeCycleStart:
//   - monthly:   anchor_day of the current calendar month
//   - quarterly: first day of the current fiscal quarter, based on anchor_month
//   - yearly:    most recent past (or current) occurrence of anchor_month/day
func computeCycleStart(anchorMonth, anchorDay int, frequencyUnit string, refTime time.Time) time.Time {
	loc := refTime.Location()

	switch frequencyUnit {
	case "monthly":
		return time.Date(refTime.Year(), refTime.Month(), anchorDay, 0, 0, 0, 0, loc)

	case "quarterly":
		monthOffset := (int(refTime.Month()) - anchorMonth + 12) % 12
		quarterIndex := monthOffset / 3
		cycleStartMonth := anchorMonth + quarterIndex*3
		year := refTime.Year()
		if cycleStartMonth > 12 {
			cycleStartMonth -= 12
		}
		if cycleStartMonth > int(refTime.Month()) {
			year--
		}
		return time.Date(year, time.Month(cycleStartMonth), 1, 0, 0, 0, 0, loc)

	case "yearly":
		anchorThisYear := time.Date(refTime.Year(), time.Month(anchorMonth), anchorDay, 0, 0, 0, 0, loc)
		if anchorThisYear.After(refTime) {
			return time.Date(refTime.Year()-1, time.Month(anchorMonth), anchorDay, 0, 0, 0, 0, loc)
		}
		return anchorThisYear

	default:
		// Unreachable from resolveT0 (DeriveDeadlineBehavior already validated
		// frequencyUnit ∈ {monthly, quarterly, yearly} for periodic behavior).
		return stripTime(refTime)
	}
}

func stripTime(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
