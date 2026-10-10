package projection

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"strconv"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	"github.com/redis/go-redis/v9"
)

type redisStore struct {
	rdb *redis.Client
	ttl time.Duration
}

// redisEntry is the cached value: the summary plus the company generation it was resolved at.
type redisEntry struct {
	Gen    int64                          `json:"gen"`
	Access authapp.EffectiveAccessSummary `json:"access"`
}

// NewRedisStore caches effective-access JSON in Redis with TTL. On read errors, behaves as cache miss.
func NewRedisStore(rdb *redis.Client, ttl time.Duration) SnapshotStore {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &redisStore{rdb: rdb, ttl: ttl}
}

func redisKey(companyID, membershipID string) string {
	return CacheKeyPrefix + ":" + companyID + ":" + membershipID
}

func redisGenerationKey(companyID string) string {
	return CacheGenerationKeyPrefix + ":" + companyID
}

func (s *redisStore) Get(ctx context.Context, membershipID, companyID string) (*authapp.EffectiveAccessSummary, int64, bool) {
	if s.rdb == nil {
		return nil, -1, false
	}
	vals, err := s.rdb.MGet(ctx, redisGenerationKey(companyID), redisKey(companyID, membershipID)).Result()
	if err != nil || len(vals) != 2 {
		return nil, -1, false
	}
	if vals[0] == nil {
		// No generation yet (first use, or the key was lost): seed one and do not cache this
		// read, so entries written under a lost generation can never match again.
		_ = s.rdb.SetNX(ctx, redisGenerationKey(companyID), newGenerationToken(), 0).Err()
		return nil, -1, false
	}
	gen, ok := parseRedisGeneration(vals[0])
	if !ok {
		return nil, -1, false
	}
	raw, isString := vals[1].(string)
	if !isString {
		return nil, gen, false
	}
	snap, hit := decodeRedisEntry(gen, []byte(raw))
	return snap, gen, hit
}

func (s *redisStore) Put(ctx context.Context, snapshot *authapp.EffectiveAccessSummary, gen int64) {
	if s.rdb == nil || snapshot == nil || gen < 0 {
		return
	}
	raw, err := json.Marshal(redisEntry{Gen: gen, Access: *snapshot})
	if err != nil {
		return
	}
	_ = s.rdb.Set(ctx, redisKey(snapshot.CompanyID, snapshot.MembershipID), raw, s.ttl).Err()
}

func (s *redisStore) InvalidateCompany(ctx context.Context, companyID string) error {
	if s.rdb == nil || companyID == "" {
		return nil
	}
	return s.rdb.Set(ctx, redisGenerationKey(companyID), newGenerationToken(), 0).Err()
}

// newGenerationToken returns a random generation value. Generations are only compared for
// equality, so a fresh random token (rather than a counter or a clock reading, which can
// repeat within one clock tick) means a reset or lost key never brings back an old generation.
func newGenerationToken() int64 {
	for {
		if g := rand.Int64(); g > 0 {
			return g
		}
	}
}

// parseRedisGeneration reads the MGET value of a present generation key.
func parseRedisGeneration(v any) (int64, bool) {
	str, ok := v.(string)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// decodeRedisEntry returns the cached summary only when it was written at generation gen.
func decodeRedisEntry(gen int64, raw []byte) (*authapp.EffectiveAccessSummary, bool) {
	var e redisEntry
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, false
	}
	if e.Gen != gen || e.Access.CompanyID == "" {
		return nil, false
	}
	return &e.Access, true
}
