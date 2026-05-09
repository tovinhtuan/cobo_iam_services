package mysql

import (
	"context"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// CompositeProvider uses DB-backed holidays when a calendar exists for that year; otherwise JSON files.
type CompositeProvider struct {
	Repo *Repository
	DB   *DBProvider
	File *disclosureapp.HolidayCalendarFileProvider
}

var _ disclosureapp.HolidayCalendarProvider = (*CompositeProvider)(nil)

// IsNonTradingDay implements disclosureapp.HolidayCalendarProvider.
func (c *CompositeProvider) IsNonTradingDay(ctx context.Context, date time.Time) (bool, string, error) {
	if c == nil || c.File == nil {
		return false, "", nil
	}
	select {
	case <-ctx.Done():
		return false, "", ctx.Err()
	default:
	}
	year := date.Year()
	if c.Repo != nil && c.DB != nil {
		ok, err := c.Repo.HasCalendarForYear(ctx, year)
		if err != nil {
			return false, "", err
		}
		if ok {
			return c.DB.IsNonTradingDay(ctx, date)
		}
	}
	return c.File.IsNonTradingDay(ctx, date)
}
