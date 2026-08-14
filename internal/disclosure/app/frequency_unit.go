package app

import (
	"strings"
	"time"
)

// NormalizeFrequencyUnit maps CMS/engine aliases onto canonical runtime units.
// Canonical: daily | weekly | monthly | quarterly | yearly.
func NormalizeFrequencyUnit(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "day", "daily":
		return PeriodicityDaily
	case "week", "weekly":
		return PeriodicityWeekly
	case "month", "monthly":
		return PeriodicityMonthly
	case "quarter", "quarterly":
		return PeriodicityQuarterly
	case "year", "yearly", "annual":
		return PeriodicityYearly
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

// IsPeriodicFrequencyUnit reports whether the unit participates in periodic
// cycle seeding / deadline R-P resolution.
func IsPeriodicFrequencyUnit(raw string) bool {
	switch NormalizeFrequencyUnit(raw) {
	case PeriodicityDaily, PeriodicityWeekly, PeriodicityMonthly, PeriodicityQuarterly, PeriodicityYearly:
		return true
	default:
		return false
	}
}

func asiaHoChiMinh() *time.Location {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	}
	return loc
}

// weekStartSunday returns 00:00 of the Sunday on or before t in t's location.
// Go's time.Weekday encodes Sunday=0; this is not ISO week numbering and not Monday.
func weekStartSunday(t time.Time) time.Time {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return day.AddDate(0, 0, -int(day.Weekday()))
}
