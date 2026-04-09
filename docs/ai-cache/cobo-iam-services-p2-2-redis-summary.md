# P2.2 Redis caching — summary

## Scope

- Optional Redis-backed `SnapshotStore` for effective-access projection (`internal/authorization/infra/projection`).
- `SnapshotStore` uses `context.Context` on `Get`/`Put` for Redis I/O.
- Key pattern: `cobo_iam:effective_access:{company_id}:{membership_id}`; value: JSON of `EffectiveAccessSummary` (existing struct tags).

## Config (env)

- `REDIS_ADDR` — if unset, in-memory store only.
- `REDIS_PASSWORD`, `REDIS_DB` (default 0).
- `EFFECTIVE_ACCESS_CACHE_TTL` (default `5m`) — applies to both in-memory and Redis TTL.

## Wiring

- `internal/platform/redis.Open` — ping with 2s timeout; on failure returns error (caller falls back).
- `cmd/api/main.go` — if `REDIS_ADDR` set and Open OK → `NewRedisStore`; else warn + `NewInMemoryStore`.

## Invalidation

- Current: TTL only (no explicit delete on admin mutations). Add `DEL` hooks when access-model writes are centralized.

## Worker

- API-only cache today; worker does not need Redis unless future projection recompute shares the same cache.
