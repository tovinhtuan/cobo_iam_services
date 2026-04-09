# P0.6 Me APIs Summary

## Completed
- Added `GET /api/v1/me`
- Added `GET /api/v1/me/companies`
- Added `GET /api/v1/me/effective-access`
- Added `GET /api/v1/me/capabilities`
- Added `GET /api/v1/me/membership`

Implementation files:
- `internal/iam/transport/http/me_handler.go`
- Updated wiring in `cmd/api/main.go`
- Extended identity contract:
  - `internal/iam/app/contracts.go` with `IdentityQueryService`
- Extended in-memory identity adapter:
  - `internal/iam/infra/inmemory/credentials.go` with `GetByUserID`
- Enhanced in-memory membership fixtures:
  - `internal/companyaccess/infra/inmemory/membership_query.go` with roles/departments/titles for `m_001`

## Behavior notes
- All `/me*` endpoints require bearer access token context.
- `/me/effective-access` is backed by authorization service `GetEffectiveAccess`.
- `/me/capabilities` is derived from effective permissions.
- `/me/membership` returns role/department/title active snapshots for current membership.

## Verification
- `go build ./...` passed.
- `go test ./...` passed.

## Known limitation
- Current source of identity/membership/access is in-memory fixture for bootstrap phase.
- MySQL repositories will replace in-memory adapters in subsequent phases.

## Next
- P0.7: audit append baseline + outbox worker skeleton integration.
