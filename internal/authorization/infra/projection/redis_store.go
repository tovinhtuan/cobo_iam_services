package projection

import (
	"context"
	"encoding/json"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	"github.com/redis/go-redis/v9"
)

type redisStore struct {
	rdb *redis.Client
	ttl time.Duration
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

func (s *redisStore) Get(ctx context.Context, membershipID, companyID string) (*authapp.EffectiveAccessSummary, bool) {
	if s.rdb == nil {
		return nil, false
	}
	raw, err := s.rdb.Get(ctx, redisKey(companyID, membershipID)).Bytes()
	if err == redis.Nil {
		return nil, false
	}
	if err != nil {
		return nil, false
	}
	var snap authapp.EffectiveAccessSummary
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, false
	}
	return &snap, true
}

func (s *redisStore) Put(ctx context.Context, snapshot *authapp.EffectiveAccessSummary) {
	if s.rdb == nil || snapshot == nil {
		return
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return
	}
	_ = s.rdb.Set(ctx, redisKey(snapshot.CompanyID, snapshot.MembershipID), raw, s.ttl).Err()
}
