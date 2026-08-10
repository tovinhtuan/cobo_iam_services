package deadlineengine

import (
	"context"
	"time"
)

// IsHolidayFunc adapts a function to NonTradingDayChecker without importing
// disclosure HolidayCalendarProvider (avoids package cycles).
type IsHolidayFunc func(ctx context.Context, date time.Time) (bool, error)

// IsHoliday implements NonTradingDayChecker.
func (f IsHolidayFunc) IsHoliday(ctx context.Context, date time.Time) (bool, error) {
	if f == nil {
		return false, nil
	}
	return f(ctx, date)
}
