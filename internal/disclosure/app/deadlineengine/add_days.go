package deadlineengine

import (
	"context"
	"time"
)

// maxWorkingDayIterations bounds the search for the next/Nth working day.
// 366 (leap year) covers one full calendar year; +4 buffer accounts for
// consecutive multi-day holiday clusters (e.g. Tet) spanning a year boundary
// when N is large. Exceeding this indicates a misconfigured holiday calendar
// or a pathological N, not a valid business scenario.
const maxWorkingDayIterations = 370

// AddDays computes start + (n-1) using inclusive counting (T0 = day #1 of N).
//
//   - calendar: start.AddDate(0, 0, n-1)
//   - working:  walk forward from start, counting only working days
//     (skip Saturday, Sunday, and holidays per holidays), inclusive of start
//     if start itself is a working day.
//
// n must be > 0. holidays may be nil (only weekends are skipped).
func AddDays(ctx context.Context, start time.Time, n int, dayType DayType, holidays NonTradingDayChecker) (time.Time, error) {
	if n <= 0 {
		return time.Time{}, ErrInvalidDeadlineDays
	}
	switch dayType {
	case DayTypeCalendar:
		return start.AddDate(0, 0, n-1), nil
	case DayTypeWorking:
		current := start
		nonTrading, err := isNonTradingDay(ctx, current, holidays)
		if err != nil {
			return time.Time{}, err
		}
		if nonTrading {
			current, err = nextWorkingDay(ctx, current, holidays)
			if err != nil {
				return time.Time{}, err
			}
		}
		count := 1
		for count < n {
			current = current.AddDate(0, 0, 1)
			nonTrading, err := isNonTradingDay(ctx, current, holidays)
			if err != nil {
				return time.Time{}, err
			}
			if !nonTrading {
				count++
			}
		}
		return current, nil
	default:
		return time.Time{}, ErrUnsupportedDayType
	}
}

// AddDaysAfter advances n calendar/working days AFTER start (start itself is not counted).
// This matches Ad-hoc proposal shipped calendar semantics: due = T0.AddDate(0, 0, n).
//
//   - calendar: start.AddDate(0, 0, n)
//   - working:  walk forward from the day after start, counting only working days
//
// n must be > 0. holidays may be nil (only weekends are skipped).
func AddDaysAfter(ctx context.Context, start time.Time, n int, dayType DayType, holidays NonTradingDayChecker) (time.Time, error) {
	if n <= 0 {
		return time.Time{}, ErrInvalidDeadlineDays
	}
	switch dayType {
	case DayTypeCalendar:
		return start.AddDate(0, 0, n), nil
	case DayTypeWorking:
		current := start
		counted := 0
		for i := 0; i < maxWorkingDayIterations && counted < n; i++ {
			current = current.AddDate(0, 0, 1)
			nonTrading, err := isNonTradingDay(ctx, current, holidays)
			if err != nil {
				return time.Time{}, err
			}
			if nonTrading {
				continue
			}
			counted++
		}
		if counted < n {
			return time.Time{}, ErrWorkingDaySearchLimit
		}
		return current, nil
	default:
		return time.Time{}, ErrUnsupportedDayType
	}
}

// nextWorkingDay returns date if it is already a working day, otherwise the
// first working day strictly after (and including) date.
func nextWorkingDay(ctx context.Context, date time.Time, holidays NonTradingDayChecker) (time.Time, error) {
	current := date
	for i := 0; i < maxWorkingDayIterations; i++ {
		nonTrading, err := isNonTradingDay(ctx, current, holidays)
		if err != nil {
			return time.Time{}, err
		}
		if !nonTrading {
			return current, nil
		}
		current = current.AddDate(0, 0, 1)
	}
	return time.Time{}, ErrWorkingDaySearchLimit
}

// isNonTradingDay reports weekend or holiday (per holidays, if non-nil).
func isNonTradingDay(ctx context.Context, date time.Time, holidays NonTradingDayChecker) (bool, error) {
	weekday := date.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return true, nil
	}
	if holidays == nil {
		return false, nil
	}
	return holidays.IsHoliday(ctx, date)
}
