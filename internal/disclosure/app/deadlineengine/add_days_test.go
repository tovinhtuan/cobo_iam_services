package deadlineengine

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeHolidayChecker reports holidays from a fixed set, or returns errYear
// for any date in a year present in errYears.
type fakeHolidayChecker struct {
	holidays map[string]bool // "2006-01-02" -> true
	errYears map[int]error
}

func (f *fakeHolidayChecker) IsHoliday(_ context.Context, date time.Time) (bool, error) {
	if f.errYears != nil {
		if err, ok := f.errYears[date.Year()]; ok {
			return false, err
		}
	}
	if f.holidays == nil {
		return false, nil
	}
	return f.holidays[date.Format("2006-01-02")], nil
}

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// --- Calendar ---------------------------------------------------------------

func TestAddDays_Calendar_N1(t *testing.T) {
	got, err := AddDays(context.Background(), date(2026, 4, 1), 1, DayTypeCalendar, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(date(2026, 4, 1)) {
		t.Fatalf("expected 2026-04-01, got %v", got)
	}
}

func TestAddDays_Calendar_N2(t *testing.T) {
	got, err := AddDays(context.Background(), date(2026, 4, 1), 2, DayTypeCalendar, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(date(2026, 4, 2)) {
		t.Fatalf("expected 2026-04-02, got %v", got)
	}
}

func TestAddDays_Calendar_MonthBoundary(t *testing.T) {
	// 31/03 + N=1 (calendar) -> 31/03 (T0 itself, inclusive); N=2 -> 01/04.
	got, err := AddDays(context.Background(), date(2026, 3, 31), 2, DayTypeCalendar, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(date(2026, 4, 1)) {
		t.Fatalf("expected 2026-04-01, got %v", got)
	}
}

func TestAddDays_Calendar_LeapYear(t *testing.T) {
	// T0=2024-02-28 (leap year), N=2 (calendar) -> 2024-02-29.
	got, err := AddDays(context.Background(), date(2024, 2, 28), 2, DayTypeCalendar, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(date(2024, 2, 29)) {
		t.Fatalf("expected 2024-02-29, got %v", got)
	}

	// T0=2024-02-28, N=3 -> 2024-03-01.
	got, err = AddDays(context.Background(), date(2024, 2, 28), 3, DayTypeCalendar, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(date(2024, 3, 1)) {
		t.Fatalf("expected 2024-03-01, got %v", got)
	}
}

// --- Working — basic ----------------------------------------------------------

func TestAddDays_Working_FridayN1(t *testing.T) {
	// 2026-04-03 is a Friday. N=1 -> stays Friday (inclusive).
	got, err := AddDays(context.Background(), date(2026, 4, 3), 1, DayTypeWorking, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(date(2026, 4, 3)) {
		t.Fatalf("expected 2026-04-03 (Friday), got %v", got)
	}
}

func TestAddDays_Working_FridayN2_SkipsWeekend(t *testing.T) {
	// 2026-04-03 Friday, N=2 -> skip Sat/Sun -> Monday 2026-04-06.
	got, err := AddDays(context.Background(), date(2026, 4, 3), 2, DayTypeWorking, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(date(2026, 4, 6)) {
		t.Fatalf("expected 2026-04-06 (Monday), got %v", got)
	}
}

func TestAddDays_Working_T0OnSaturday(t *testing.T) {
	// 2026-04-04 is a Saturday. N=1 -> first working day from T0 = Monday 2026-04-06.
	got, err := AddDays(context.Background(), date(2026, 4, 4), 1, DayTypeWorking, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(date(2026, 4, 6)) {
		t.Fatalf("expected 2026-04-06 (Monday), got %v", got)
	}
}

// --- Working — holiday cluster -------------------------------------------------

func TestAddDays_Working_HolidayCluster(t *testing.T) {
	// Tet-like cluster: 2026-02-16..2026-02-20 (Mon-Fri) all holidays.
	// T0 = 2026-02-13 (Friday, working day). N=2 -> next working day after
	// skipping the full holiday week + weekend = 2026-02-23 (Monday).
	checker := &fakeHolidayChecker{holidays: map[string]bool{
		"2026-02-16": true,
		"2026-02-17": true,
		"2026-02-18": true,
		"2026-02-19": true,
		"2026-02-20": true,
	}}
	got, err := AddDays(context.Background(), date(2026, 2, 13), 2, DayTypeWorking, checker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(date(2026, 2, 23)) {
		t.Fatalf("expected 2026-02-23 (Monday), got %v", got)
	}
}

func TestAddDays_Working_T0OnHoliday(t *testing.T) {
	// T0 = 2026-04-30 (Thursday, holiday). N=1 -> next working day = 2026-05-01
	// unless that's also a holiday; use a single-day holiday for clarity.
	checker := &fakeHolidayChecker{holidays: map[string]bool{
		"2026-04-30": true,
	}}
	got, err := AddDays(context.Background(), date(2026, 4, 30), 1, DayTypeWorking, checker)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(date(2026, 5, 1)) {
		t.Fatalf("expected 2026-05-01, got %v", got)
	}
}

// --- Working — year boundary ----------------------------------------------------

func TestAddDays_Working_YearBoundary(t *testing.T) {
	// T0 = 2026-12-29 (Tuesday). N=3 working days, no holidays defined for
	// 2027 (checker returns false, not error, in this test) ->
	// 2026-12-29 (1), 2026-12-30 (2), 2026-12-31 (3) -> stays in 2026.
	got, err := AddDays(context.Background(), date(2026, 12, 29), 3, DayTypeWorking, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(date(2026, 12, 31)) {
		t.Fatalf("expected 2026-12-31, got %v", got)
	}
}

func TestAddDays_Working_YearBoundary_HolidayProviderErrorPropagated(t *testing.T) {
	// T0 = 2026-12-30 (Wednesday). N=5 working days requires crossing into
	// 2027, but the holiday calendar for 2027 is "missing" (checker errors).
	// AddDays must propagate the error, not silently fall back.
	wantErr := errors.New("missing holiday config for year 2027")
	checker := &fakeHolidayChecker{errYears: map[int]error{2027: wantErr}}

	_, err := AddDays(context.Background(), date(2026, 12, 30), 5, DayTypeWorking, checker)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected propagated error %v, got %v", wantErr, err)
	}
}

// --- Errors -----------------------------------------------------------------

func TestAddDays_InvalidN(t *testing.T) {
	_, err := AddDays(context.Background(), date(2026, 4, 1), 0, DayTypeCalendar, nil)
	if !errors.Is(err, ErrInvalidDeadlineDays) {
		t.Fatalf("expected ErrInvalidDeadlineDays, got %v", err)
	}

	_, err = AddDays(context.Background(), date(2026, 4, 1), -1, DayTypeWorking, nil)
	if !errors.Is(err, ErrInvalidDeadlineDays) {
		t.Fatalf("expected ErrInvalidDeadlineDays, got %v", err)
	}
}

func TestAddDays_UnsupportedDayType(t *testing.T) {
	_, err := AddDays(context.Background(), date(2026, 4, 1), 1, DayType("biweekly"), nil)
	if !errors.Is(err, ErrUnsupportedDayType) {
		t.Fatalf("expected ErrUnsupportedDayType, got %v", err)
	}
}

func TestAddDays_WorkingDaySearchLimit(t *testing.T) {
	// Checker reports every day as a holiday -> nextWorkingDay search exhausts
	// maxWorkingDayIterations.
	_, err := AddDays(context.Background(), date(2026, 4, 1), 1, DayTypeWorking, allHolidaysChecker{})
	if !errors.Is(err, ErrWorkingDaySearchLimit) {
		t.Fatalf("expected ErrWorkingDaySearchLimit, got %v", err)
	}
}

// allHolidaysChecker reports every weekday as a holiday too — used to force
// the working-day search to exceed maxWorkingDayIterations.
type allHolidaysChecker struct{}

func (allHolidaysChecker) IsHoliday(_ context.Context, _ time.Time) (bool, error) {
	return true, nil
}
