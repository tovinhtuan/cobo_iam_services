package projection

import (
	"context"
	"sync"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
)

// SnapshotStore caches resolved effective access summaries (P2 projection optimization).
type SnapshotStore interface {
	Get(ctx context.Context, membershipID, companyID string) (*authapp.EffectiveAccessSummary, bool)
	Put(ctx context.Context, snapshot *authapp.EffectiveAccessSummary)
}

type inMemoryStore struct {
	mu    sync.RWMutex
	ttl   time.Duration
	items map[string]entry
	now   func() time.Time
}

type entry struct {
	snapshot authapp.EffectiveAccessSummary
	expires  time.Time
}

func NewInMemoryStore(ttl time.Duration) SnapshotStore {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &inMemoryStore{ttl: ttl, items: map[string]entry{}, now: time.Now}
}

func (s *inMemoryStore) Get(_ context.Context, membershipID, companyID string) (*authapp.EffectiveAccessSummary, bool) {
	s.mu.RLock()
	it, ok := s.items[key(membershipID, companyID)]
	s.mu.RUnlock()
	if !ok || s.now().After(it.expires) {
		if ok {
			s.mu.Lock()
			delete(s.items, key(membershipID, companyID))
			s.mu.Unlock()
		}
		return nil, false
	}
	cp := it.snapshot
	return &cp, true
}

func (s *inMemoryStore) Put(_ context.Context, snapshot *authapp.EffectiveAccessSummary) {
	if snapshot == nil {
		return
	}
	s.mu.Lock()
	s.items[key(snapshot.MembershipID, snapshot.CompanyID)] = entry{snapshot: *snapshot, expires: s.now().Add(s.ttl)}
	s.mu.Unlock()
}

func key(membershipID, companyID string) string { return membershipID + "@" + companyID }

// CacheKeyPrefix is the Redis key namespace for effective-access snapshots.
const CacheKeyPrefix = "cobo_iam:effective_access"
