package app

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

// Cache TTL and size constants.
const (
	cachePositiveTTL = 60 * time.Minute  // positive entries (found) expire after 1 hour
	cacheNegativeTTL = 10 * time.Minute  // negative entries (not found) expire after 10 minutes
	cacheMaxSize     = 2000               // covers ~1700 listed companies with headroom
)

// cacheEntry holds a looked-up result and its expiry time.
// A nil result pointer indicates a negative cache entry (company not found).
type cacheEntry struct {
	result *ListedCompanyDetail
	expiry time.Time
}

// businessCodeCache is an in-process TTL cache for business-code lookups.
// Uses stdlib sync.RWMutex — no external dependencies.
type businessCodeCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
	maxSize int
}

// get returns (result, cacheHit). result is nil for a negative cache hit (ErrNotFound case).
func (c *businessCodeCache) get(key string) (*ListedCompanyDetail, bool) {
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiry) {
		return nil, false // expired
	}
	return entry.result, true
}

// set writes an entry under the write lock, evicting to stay within maxSize.
// Pass a nil result to store a negative cache entry.
func (c *businessCodeCache) set(key string, result *ListedCompanyDetail, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.maxSize {
		c.evictExpiredLocked()
		if len(c.entries) >= c.maxSize {
			c.evictOldestLocked()
		}
	}
	c.entries[key] = cacheEntry{result: result, expiry: time.Now().Add(ttl)}
}

// evictExpiredLocked removes all entries whose TTL has lapsed. Caller must hold write lock.
func (c *businessCodeCache) evictExpiredLocked() {
	now := time.Now()
	for k, e := range c.entries {
		if now.After(e.expiry) {
			delete(c.entries, k)
		}
	}
}

// evictOldestLocked removes the single entry with the earliest expiry. Caller must hold write lock.
func (c *businessCodeCache) evictOldestLocked() {
	var oldestKey string
	var oldestExpiry time.Time
	first := true
	for k, e := range c.entries {
		if first || e.expiry.Before(oldestExpiry) {
			oldestKey = k
			oldestExpiry = e.expiry
			first = false
		}
	}
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// Service exposes read-only listed company reference operations for CMS.
type Service struct {
	repo  ListedCompanyReader
	ping  func(context.Context) error
	cache *businessCodeCache
}

// NewService wires a vnstock reader with optional DB ping (503 when ping fails).
func NewService(repo ListedCompanyReader, ping func(context.Context) error) *Service {
	return &Service{
		repo:  repo,
		ping:  ping,
		cache: &businessCodeCache{entries: make(map[string]cacheEntry), maxSize: cacheMaxSize},
	}
}

// NewDisabledService returns a service that always responds with ErrUnavailable.
func NewDisabledService() *Service {
	return &Service{cache: &businessCodeCache{entries: make(map[string]cacheEntry), maxSize: cacheMaxSize}}
}

// List returns a filtered page of listed companies.
func (s *Service) List(ctx context.Context, p ListParams) (ListResult, error) {
	if err := s.checkAvailable(ctx); err != nil {
		return ListResult{}, err
	}
	p, err := NormalizeListParams(p)
	if err != nil {
		return ListResult{}, err
	}
	out, err := s.repo.List(ctx, p)
	if err != nil {
		return ListResult{}, mapRepoError(err)
	}
	return out, nil
}

// GetDetail returns company detail; partial when equity exists without profile.
func (s *Service) GetDetail(ctx context.Context, symbol string) (ListedCompanyDetail, error) {
	if err := s.checkAvailable(ctx); err != nil {
		return ListedCompanyDetail{}, err
	}
	symbol = NormalizeSymbol(symbol)
	if symbol == "" {
		return ListedCompanyDetail{}, ErrInvalidRequest
	}
	detail, err := s.repo.GetBySymbol(ctx, symbol)
	if err != nil {
		return ListedCompanyDetail{}, mapRepoError(err)
	}
	return detail, nil
}

// GetByBusinessCode looks up a listed company by its business registration code (mã ĐKKD).
//
// Returns (detail, cacheHit=true, nil)   on cache hit (positive).
// Returns (zero,   cacheHit=false, ErrNotFound) on cache hit (negative) or repo miss.
// Returns (zero,   cacheHit=false, ErrUnavailable) when service is disabled or DB unreachable.
// Returns (zero,   cacheHit=false, ErrInvalidRequest) when businessCode is empty after trimming.
//
// Uses double-checked locking: RLock for read, then Lock+recheck before writing to cache.
func (s *Service) GetByBusinessCode(ctx context.Context, businessCode string) (ListedCompanyDetail, bool, error) {
	if err := s.checkAvailable(ctx); err != nil {
		return ListedCompanyDetail{}, false, err
	}
	businessCode = strings.TrimSpace(businessCode)
	if businessCode == "" {
		return ListedCompanyDetail{}, false, ErrInvalidRequest
	}

	// Fast path: cache read under RLock.
	if cached, hit := s.cache.get(businessCode); hit {
		if cached == nil {
			return ListedCompanyDetail{}, true, ErrNotFound // negative cache hit
		}
		return *cached, true, nil
	}

	// Slow path: call repository.
	detail, err := s.repo.GetByBusinessCode(ctx, businessCode)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return ListedCompanyDetail{}, false, mapRepoError(err)
	}

	// Write to cache under write lock with double-check.
	s.cache.mu.Lock()
	// Double-check: another goroutine may have populated the cache while we called the repo.
	if existing, ok := s.cache.entries[businessCode]; ok && time.Now().Before(existing.expiry) {
		s.cache.mu.Unlock()
		if existing.result == nil {
			return ListedCompanyDetail{}, false, ErrNotFound
		}
		return *existing.result, false, nil
	}
	if errors.Is(err, ErrNotFound) {
		s.cache.entries[businessCode] = cacheEntry{result: nil, expiry: time.Now().Add(cacheNegativeTTL)}
		if len(s.cache.entries) > s.cache.maxSize {
			s.cache.evictExpiredLocked()
			if len(s.cache.entries) > s.cache.maxSize {
				s.cache.evictOldestLocked()
			}
		}
		s.cache.mu.Unlock()
		return ListedCompanyDetail{}, false, ErrNotFound
	}
	copied := detail
	s.cache.entries[businessCode] = cacheEntry{result: &copied, expiry: time.Now().Add(cachePositiveTTL)}
	if len(s.cache.entries) > s.cache.maxSize {
		s.cache.evictExpiredLocked()
		if len(s.cache.entries) > s.cache.maxSize {
			s.cache.evictOldestLocked()
		}
	}
	s.cache.mu.Unlock()
	return detail, false, nil
}

func (s *Service) checkAvailable(ctx context.Context) error {
	if s.repo == nil {
		return ErrUnavailable
	}
	if s.ping != nil {
		if err := s.ping(ctx); err != nil {
			return ErrUnavailable
		}
	}
	return nil
}

func mapRepoError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrInvalidRequest) || errors.Is(err, ErrNotFound) {
		return err
	}
	return ErrUnavailable
}
