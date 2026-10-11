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
	// ttl > 0 reloads a cached year after ttl; 0 keeps it until InvalidateYear (API process,
	// which invalidates on upload). The worker cannot see that invalidation (PER-06).
	ttl time.Duration
	now func() time.Time

	mu    sync.RWMutex
	cache map[int]yearCache
}

type yearCache struct {
	m        map[string]string
	loadedAt time.Time
}

func NewDBProvider(repo *Repository) *DBProvider {
	return NewDBProviderWithTTL(repo, 0)
}

// NewDBProviderWithTTL is NewDBProvider with cached years reloaded after ttl.
func NewDBProviderWithTTL(repo *Repository, ttl time.Duration) *DBProvider {
	return &DBProvider{
		repo:  repo,
		ttl:   ttl,
		now:   time.Now,
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
	if ok && p.ttl > 0 && p.now().Sub(yc.loadedAt) >= p.ttl {
		ok = false
	}
	if ok {
		reason, hit := yc.m[key]
		if !hit {
			return false, "", nil
		}
		return true, reason, nil
	}

	m, err := p.repo.GetHolidayMapForYear(ctx, year)
	if err != nil {
		// Do not cache failures: a transient DB error must not break the year until restart.
		return false, "", err
	}

	p.mu.Lock()
	p.cache[year] = yearCache{m: m, loadedAt: p.now()}
	p.mu.Unlock()

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
