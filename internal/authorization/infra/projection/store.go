package projection

import (
	"context"
	"sync"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
)

// SnapshotStore caches resolved effective access summaries (P2 projection optimization).
//
// Every company has a cache generation. InvalidateCompany bumps it, and an entry is only
// served while it was written at the current generation. Get returns the generation it
// observed; the caller passes that value to Put, so a summary resolved from data read
// before an invalidation is never served after it (CACHE-11).
type SnapshotStore interface {
	// Get returns the cached summary when ok. gen is the company generation observed by
	// this read; a negative gen means the generation is unknown and the result must not
	// be cached.
	Get(ctx context.Context, membershipID, companyID string) (snapshot *authapp.EffectiveAccessSummary, gen int64, ok bool)
	// Put caches snapshot as written at generation gen. A negative gen is ignored.
	Put(ctx context.Context, snapshot *authapp.EffectiveAccessSummary, gen int64)
	// InvalidateCompany drops every cached summary of the company. Entries written under the
	// previous generation are never served again, even if they are written after this call.
	InvalidateCompany(ctx context.Context, companyID string) error
}

type inMemoryStore struct {
	mu    sync.RWMutex
	ttl   time.Duration
	items map[string]entry
	gens  map[string]int64
	now   func() time.Time
}

type entry struct {
	snapshot authapp.EffectiveAccessSummary
	gen      int64
	expires  time.Time
}

func NewInMemoryStore(ttl time.Duration) SnapshotStore {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &inMemoryStore{ttl: ttl, items: map[string]entry{}, gens: map[string]int64{}, now: time.Now}
}

func (s *inMemoryStore) Get(_ context.Context, membershipID, companyID string) (*authapp.EffectiveAccessSummary, int64, bool) {
	s.mu.RLock()
	gen := s.gens[companyID]
	it, ok := s.items[key(membershipID, companyID)]
	s.mu.RUnlock()
	if !ok || it.gen != gen || s.now().After(it.expires) {
		if ok {
			s.mu.Lock()
			// Re-check: a fresh entry may have been put since the read lock was released.
			if cur, still := s.items[key(membershipID, companyID)]; still && (cur.gen != s.gens[companyID] || s.now().After(cur.expires)) {
				delete(s.items, key(membershipID, companyID))
			}
			s.mu.Unlock()
		}
		return nil, gen, false
	}
	cp := it.snapshot
	return &cp, gen, true
}

func (s *inMemoryStore) Put(_ context.Context, snapshot *authapp.EffectiveAccessSummary, gen int64) {
	if snapshot == nil || gen < 0 {
		return
	}
	s.mu.Lock()
	if gen == s.gens[snapshot.CompanyID] {
		s.items[key(snapshot.MembershipID, snapshot.CompanyID)] = entry{snapshot: *snapshot, gen: gen, expires: s.now().Add(s.ttl)}
	}
	s.mu.Unlock()
}

func (s *inMemoryStore) InvalidateCompany(_ context.Context, companyID string) error {
	if companyID == "" {
		return nil
	}
	s.mu.Lock()
	s.gens[companyID]++
	for k, it := range s.items {
		if it.snapshot.CompanyID == companyID {
			delete(s.items, k)
		}
	}
	s.mu.Unlock()
	return nil
}

func key(membershipID, companyID string) string { return membershipID + "@" + companyID }

// CacheKeyPrefix is the Redis key namespace for effective-access snapshots. The v2 segment
// marks the generation envelope; binaries that cache the bare summary use the old prefix.
const CacheKeyPrefix = "cobo_iam:effective_access:v2"

// CacheGenerationKeyPrefix is the Redis key namespace for per-company cache generations.
const CacheGenerationKeyPrefix = "cobo_iam:effective_access_gen:v2"
