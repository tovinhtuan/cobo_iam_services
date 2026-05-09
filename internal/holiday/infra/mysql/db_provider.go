package mysql

import (
	"context"
	"sync"
	"time"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// DBProvider loads non-trading holidays from MySQL (per-year cache).
type DBProvider struct {
	repo *Repository

	mu    sync.RWMutex
	cache map[int]yearCache
}

type yearCache struct {
	m   map[string]string
	err error
}

func NewDBProvider(repo *Repository) *DBProvider {
	return &DBProvider{
		repo:  repo,
		cache: map[int]yearCache{},
	}
}

var _ disclosureapp.HolidayCalendarProvider = (*DBProvider)(nil)

// IsNonTradingDay implements disclosureapp.HolidayCalendarProvider.
func (p *DBProvider) IsNonTradingDay(ctx context.Context, date time.Time) (bool, string, error) {
	select {
	case <-ctx.Done():
		return false, "", ctx.Err()
	default:
	}
	year := date.Year()
	key := date.Format("2006-01-02")

	p.mu.RLock()
	yc, ok := p.cache[year]
	p.mu.RUnlock()
	if ok {
		if yc.err != nil {
			return false, "", yc.err
		}
		reason, hit := yc.m[key]
		if !hit {
			return false, "", nil
		}
		return true, reason, nil
	}

	m, err := p.repo.GetHolidayMapForYear(ctx, year)

	p.mu.Lock()
	p.cache[year] = yearCache{m: m, err: err}
	p.mu.Unlock()

	if err != nil {
		return false, "", err
	}
	reason, hit := m[key]
	if !hit {
		return false, "", nil
	}
	return true, reason, nil
}

// InvalidateYear drops cached rows for a civil year (call after ReplaceCalendar).
func (p *DBProvider) InvalidateYear(year int) {
	p.mu.Lock()
	delete(p.cache, year)
	p.mu.Unlock()
}
