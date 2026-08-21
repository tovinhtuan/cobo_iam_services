package app

import (
	"fmt"
	"time"
)

// TSource identifies Effective T authority.
const (
	TSourceCMS     = "CMS"
	TSourceCompany = "COMPANY"
)

// AnchorConfig is structured T (month/day) from CMS or company override.
type AnchorConfig struct {
	Month int // 1-12; 0 = unset
	Day   int // 1-31; 0 = unset
}

// HasOverride reports whether company provided a T override.
func (a AnchorConfig) HasOverride() bool {
	return a.Month > 0 || a.Day > 0
}

// EffectiveSchedule is the canonical resolved schedule for one logical slot.
type EffectiveSchedule struct {
	FrequencyUnit string
	CycleLabel    string
	EffectiveT    time.Time
	TSource       string
	OpenAt        time.Time
	DueDate       time.Time
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
// WEEKLY: Sunday week boundary (weekday residual OUT_OF_SCOPE for V1).
// QUARTERLY: first day of calendar quarter (structured month_in_quarter residual).
// DAILY: the slot calendar day.
func ResolveOccurrenceT(frequencyUnit string, cycleLabel string, anchor AnchorConfig, loc *time.Location) (time.Time, error) {
	if loc == nil {
		loc = asiaHoChiMinh()
	}
	month := anchor.Month
	day := anchor.Day
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
		return weekStartSunday(t), nil
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
		startMonth := time.Month((q-1)*3 + 1)
		return ClampDayOfMonth(y, startMonth, 1, loc), nil
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
