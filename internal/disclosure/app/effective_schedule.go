package app

import (
	"fmt"
	"net/http"
	"time"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// TSource identifies Effective T authority.
const (
	TSourceCMS     = "CMS"
	TSourceCompany = "COMPANY"
)

// Cycle anchor day-of-month configuration bounds (CMS + Company write contract).
const (
	CycleAnchorDayMin = 1
	CycleAnchorDayMax = 31

	CycleAnchorWeekdayMin = 0 // time.Sunday
	CycleAnchorWeekdayMax = 6 // time.Saturday

	MonthInQuarterMin = 1
	MonthInQuarterMax = 3
)

// ValidateCycleAnchorDay enforces an explicitly configured T day-of-month.
// day == 0 means unset/omitted (inherit CMS or use defaults) and is allowed.
// Invalid values must fail — do not clamp configuration to a valid day.
func ValidateCycleAnchorDay(day int) error {
	if day == 0 {
		return nil
	}
	if day < CycleAnchorDayMin || day > CycleAnchorDayMax {
		return perr.NewHTTPError(
			http.StatusBadRequest,
			perr.CodeInvalidRequest,
			fmt.Sprintf("cycle_anchor_day must be between %d and %d (got %d)", CycleAnchorDayMin, CycleAnchorDayMax, day),
			nil,
		)
	}
	return nil
}

// ValidateCycleAnchorWeekday enforces an explicitly configured weekly T weekday.
// weekday == nil means unset/legacy (Sunday at resolve) and is allowed.
// Encoding: Go time.Weekday 0=Sunday .. 6=Saturday. Invalid values are rejected (no modulo).
func ValidateCycleAnchorWeekday(weekday *int) error {
	if weekday == nil {
		return nil
	}
	if *weekday < CycleAnchorWeekdayMin || *weekday > CycleAnchorWeekdayMax {
		return perr.NewHTTPError(
			http.StatusBadRequest,
			perr.CodeInvalidRequest,
			fmt.Sprintf("cycle_anchor_weekday must be between %d and %d (got %d)", CycleAnchorWeekdayMin, CycleAnchorWeekdayMax, *weekday),
			nil,
		)
	}
	return nil
}

// ValidateMonthInQuarter enforces an explicitly configured quarterly month-in-quarter.
// miq == nil means unset/legacy (1 at resolve) and is allowed.
// Encoding: 1..3 (first/second/third month of calendar quarter). No clamp.
func ValidateMonthInQuarter(miq *int) error {
	if miq == nil {
		return nil
	}
	if *miq < MonthInQuarterMin || *miq > MonthInQuarterMax {
		return perr.NewHTTPError(
			http.StatusBadRequest,
			perr.CodeInvalidRequest,
			fmt.Sprintf("month_in_quarter must be between %d and %d (got %d)", MonthInQuarterMin, MonthInQuarterMax, *miq),
			nil,
		)
	}
	return nil
}

// ValidateScheduleAnchorFields validates optional schedule-anchor fields on write.
// Missing/nil fields remain valid (legacy). Explicit out-of-range values fail.
func ValidateScheduleAnchorFields(day int, weekday *int, monthInQuarter *int) error {
	if err := ValidateCycleAnchorDay(day); err != nil {
		return err
	}
	if err := ValidateCycleAnchorWeekday(weekday); err != nil {
		return err
	}
	return ValidateMonthInQuarter(monthInQuarter)
}

// AnchorConfig is structured T from CMS or company override.
// Weekday and MonthInQuarter use pointers so nil = unset (legacy defaults at normalize).
type AnchorConfig struct {
	Month          int  // 1-12; 0 = unset
	Day            int  // 1-31; 0 = unset
	Weekday        *int // 0=Sun..6=Sat; nil = unset → Sunday
	MonthInQuarter *int // 1..3; nil = unset → 1
}

// HasOverride reports whether company provided a T override (month/day Phase-1 fields).
// Weekday/MonthInQuarter company override is Phase 3 (not enabled here).
func (a AnchorConfig) HasOverride() bool {
	return a.Month > 0 || a.Day > 0
}

// EffectiveSchedule is the canonical resolved schedule for one logical slot.
type EffectiveSchedule struct {
	FrequencyUnit  string
	CycleLabel     string
	EffectiveT     time.Time
	TSource        string
	OpenAt         time.Time
	DueDate        time.Time
	OpenDaysBefore int
	DeadlineDays   int
}

// ClampDayOfMonth returns date(year, month, day) clamped to last day of month (PO).
// Never Go-normalizes into the next month.
func ClampDayOfMonth(year int, month time.Month, day int, loc *time.Location) time.Time {
	if loc == nil {
		loc = asiaHoChiMinh()
	}
	if month < 1 {
		month = 1
	}
	if month > 12 {
		month = 12
	}
	if day < 1 {
		day = 1
	}
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()
	if day > last {
		day = last
	}
	return time.Date(year, month, day, 0, 0, 0, 0, loc)
}

// ResolveEffectiveAnchor returns company override if set, else CMS default.
// Phase 0/1: only Month/Day participate in company override merge.
// Weekday/MonthInQuarter company override deferred to Phase 3.
func ResolveEffectiveAnchor(cms, company AnchorConfig) (AnchorConfig, string) {
	if company.Month > 0 || company.Day > 0 {
		out := cms
		if company.Month > 0 {
			out.Month = company.Month
		}
		if company.Day > 0 {
			out.Day = company.Day
		}
		return out, TSourceCompany
	}
	return cms, TSourceCMS
}

// NormalizeScheduleAnchor applies frequency-aware legacy defaults for resolution.
// Does not mutate persisted configuration; returns a copy-style struct for T resolve.
//
//	daily:     no anchor
//	weekly:    weekday = explicit || Sunday (0)
//	monthly:   day left to ResolveOccurrenceT (0 → 1)
//	quarterly: month_in_quarter = explicit || 1; day = explicit || 1
//	yearly:    month/day left to ResolveOccurrenceT
func NormalizeScheduleAnchor(frequencyUnit string, raw AnchorConfig) AnchorConfig {
	out := raw
	switch NormalizeFrequencyUnit(frequencyUnit) {
	case PeriodicityWeekly:
		if out.Weekday == nil {
			sun := int(time.Sunday) // 0
			out.Weekday = &sun
		}
	case PeriodicityQuarterly:
		if out.MonthInQuarter == nil {
			one := 1
			out.MonthInQuarter = &one
		}
		if out.Day <= 0 {
			out.Day = 1
		}
	}
	return out
}

// ResolveLogicalSlot returns cycle_label for the current period (slot ≠ resolved T).
func ResolveLogicalSlot(frequencyUnit string, now time.Time, loc *time.Location) string {
	if loc == nil {
		loc = asiaHoChiMinh()
	}
	n := now.In(loc)
	switch NormalizeFrequencyUnit(frequencyUnit) {
	case PeriodicityDaily:
		return n.Format("2006-01-02")
	case PeriodicityWeekly:
		return weekStartSunday(n).Format("2006-01-02")
	case PeriodicityMonthly:
		return n.Format("2006-01")
	case PeriodicityQuarterly:
		q := (int(n.Month())-1)/3 + 1
		return fmt.Sprintf("%d-Q%d", n.Year(), q)
	case PeriodicityYearly:
		return fmt.Sprintf("%d", n.Year())
	default:
		return n.Format("2006-01-02")
	}
}

// ResolveOccurrenceT resolves Effective T for a logical slot using effective anchors.
// YEARLY/MONTHLY use structured month/day with last-day clamp.
// WEEKLY: Sunday-based slot identity; T = Sunday + configured weekday offset (legacy Sunday).
// QUARTERLY: YYYY-Qn slot; T = Clamp(Q_start_month + MiQ - 1, day) (legacy MiQ=1 day=1).
// DAILY: the slot calendar day (month/day/weekday ignored).
func ResolveOccurrenceT(frequencyUnit string, cycleLabel string, anchor AnchorConfig, loc *time.Location) (time.Time, error) {
	if loc == nil {
		loc = asiaHoChiMinh()
	}
	norm := NormalizeScheduleAnchor(frequencyUnit, anchor)
	month := norm.Month
	day := norm.Day
	if month <= 0 || month > 12 {
		month = 1
	}
	if day <= 0 {
		day = 1
	}

	switch NormalizeFrequencyUnit(frequencyUnit) {
	case PeriodicityDaily:
		t, err := time.ParseInLocation("2006-01-02", cycleLabel, loc)
		if err != nil {
			return time.Time{}, err
		}
		return stripTime(t), nil
	case PeriodicityWeekly:
		t, err := time.ParseInLocation("2006-01-02", cycleLabel, loc)
		if err != nil {
			return time.Time{}, err
		}
		sunday := weekStartSunday(t)
		weekday := int(time.Sunday)
		if norm.Weekday != nil {
			weekday = *norm.Weekday
		}
		return stripTime(sunday.AddDate(0, 0, weekday)), nil
	case PeriodicityMonthly:
		var y int
		var m time.Month
		if _, err := fmt.Sscanf(cycleLabel, "%d-%d", &y, &m); err != nil {
			return time.Time{}, fmt.Errorf("invalid monthly cycle_label %q: %w", cycleLabel, err)
		}
		return ClampDayOfMonth(y, m, day, loc), nil
	case PeriodicityQuarterly:
		var y, q int
		if _, err := fmt.Sscanf(cycleLabel, "%d-Q%d", &y, &q); err != nil {
			return time.Time{}, fmt.Errorf("invalid quarterly cycle_label %q: %w", cycleLabel, err)
		}
		if q < 1 || q > 4 {
			q = 1
		}
		miq := 1
		if norm.MonthInQuarter != nil {
			miq = *norm.MonthInQuarter
		}
		startMonth := time.Month((q-1)*3 + 1)
		targetMonth := startMonth + time.Month(miq-1)
		return ClampDayOfMonth(y, targetMonth, day, loc), nil
	case PeriodicityYearly:
		var y int
		if _, err := fmt.Sscanf(cycleLabel, "%d", &y); err != nil {
			return time.Time{}, fmt.Errorf("invalid yearly cycle_label %q: %w", cycleLabel, err)
		}
		return ClampDayOfMonth(y, time.Month(month), day, loc), nil
	default:
		return ClampDayOfMonth(time.Now().In(loc).Year(), time.Month(month), day, loc), nil
	}
}

// ResolveOpenAt returns OpenAt = EffectiveT − openDaysBefore (calendar). openDaysBefore<=0 → OpenAt=T.
func ResolveOpenAt(effectiveT time.Time, openDaysBefore int) time.Time {
	if openDaysBefore <= 0 {
		return stripTime(effectiveT)
	}
	return stripTime(effectiveT.AddDate(0, 0, -openDaysBefore))
}

// SubmissionCompliance classifies company submission vs due date (end-of-business-date HCM).
// Returns: PENDING | OVERDUE | SUBMITTED_ON_TIME | SUBMITTED_LATE
func SubmissionCompliance(dueDateYYYYMMDD string, submittedAt *time.Time, now time.Time, loc *time.Location) string {
	if loc == nil {
		loc = asiaHoChiMinh()
	}
	due, err := time.ParseInLocation("2006-01-02", dueDateYYYYMMDD, loc)
	if err != nil {
		return "PENDING"
	}
	dueDay := time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, loc)
	if submittedAt == nil {
		today := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 0, 0, 0, 0, loc)
		if today.After(dueDay) {
			return "OVERDUE"
		}
		return "PENDING"
	}
	s := submittedAt.In(loc)
	subDay := time.Date(s.Year(), s.Month(), s.Day(), 0, 0, 0, 0, loc)
	if !subDay.After(dueDay) {
		return "SUBMITTED_ON_TIME"
	}
	return "SUBMITTED_LATE"
}
