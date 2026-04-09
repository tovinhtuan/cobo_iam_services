# P2.1 Effective Access Projection Optimization Summary

## Completed
- Added projection optimization schema migrations:
  - `migrations/0003_effective_access_projection.up.sql`
  - `migrations/0003_effective_access_projection.down.sql`
- New projection tables:
  - `membership_effective_permissions`
  - `membership_effective_departments`
  - `membership_effective_responsibilities`
  - `effective_access_snapshots`

- Added cached projection resolver in authorization module:
  - `internal/authorization/infra/projection/store.go`
  - `internal/authorization/infra/projection/cached_resolver.go`

- Wired API authorization stack to use read-through cached resolver:
  - `cmd/api/main.go`
  - flow: cache hit -> return snapshot, cache miss -> resolve base -> cache store.

## Optimization behavior
- Snapshot cache key: `membership_id + company_id`
- In-memory TTL default configured to 5 minutes in API bootstrap.
- No contract changes to `/me/effective-access` or internal authorize APIs.

## Verification
- `go build ./...` passed.
- `go test ./...` passed.

## Notes
- Current projection store is in-memory bootstrap implementation for safe incremental rollout.
- Next hardening step: add MySQL projection repository + outbox-driven recompute events for persistent projection freshness.
