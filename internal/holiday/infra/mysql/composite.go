package mysql

import (
	"context"
	"sync"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// CompositeProvider uses DB-backed holidays when a calendar exists for that year; otherwise JSON files.
type CompositeProvider struct {
	Repo *Repository
	DB   *DBProvider
	File *disclosureapp.HolidayCalendarFileProvider
	// YearCheckTTL > 0 caches "a DB calendar exists for the year" for that long, so a
	// long-lived worker provider does not query per checked day (PER-06). 0 = query each call.
	YearCheckTTL time.Duration

	mu    sync.Mutex
	years map[int]yearCheck
	now   func() time.Time
}

type yearCheck struct {
	exists  bool
	checked time.Time
}

func (c *CompositeProvider) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// hasDBCalendar reports whether a DB calendar exists for year, cached for YearCheckTTL.
func (c *CompositeProvider) hasDBCalendar(ctx context.Context, year int) (bool, error) {
	if c.YearCheckTTL > 0 {
		c.mu.Lock()
		yc, ok := c.years[year]
		c.mu.Unlock()
		if ok && c.clock().Sub(yc.checked) < c.YearCheckTTL {
			return yc.exists, nil
		}
	}
	exists, err := c.Repo.HasCalendarForYear(ctx, year)
	if err != nil {
		return false, err
	}
	if c.YearCheckTTL > 0 {
		c.mu.Lock()
		if c.years == nil {
			c.years = map[int]yearCheck{}
		}
		c.years[year] = yearCheck{exists: exists, checked: c.clock()}
		c.mu.Unlock()
	}
	return exists, nil
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
		ok, err := c.hasDBCalendar(ctx, year)
		if err != nil {
			return false, "", err
		}
		if ok {
			return c.DB.IsNonTradingDay(ctx, date)
		}
	}
	return c.File.IsNonTradingDay(ctx, date)
}
