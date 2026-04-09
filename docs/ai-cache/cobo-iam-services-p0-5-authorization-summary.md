# P0.5 Authorization Core Summary

## Completed
- Implemented authorization application service:
  - `internal/authorization/app/service.go`
  - Methods: `Authorize`, `AuthorizeBatch`, `GetEffectiveAccess`
- Implemented in-memory authorization adapters:
  - `internal/authorization/infra/inmemory/repository.go`
  - `internal/authorization/infra/inmemory/resolver.go`
  - `internal/authorization/infra/inmemory/checker.go`
- Implemented internal transport handlers:
  - `internal/authorization/transport/http/handler.go`
  - Endpoints:
    - `POST /internal/v1/authorize`
    - `POST /internal/v1/authorize/batch`
- Wired authorization stack into API bootstrap in `cmd/api/main.go`.

## Behavior baseline
- Tenant boundary check first: mismatch `membership_id/company_id` => deny with `COMPANY_SCOPE_MISMATCH`.
- Permission check via action mapping:
  - `disclosure.view` -> `view_disclosure`
  - `disclosure.approve` -> `approve_disclosure`
  - `dashboard.view` -> `view_dashboard`
- Data scope check:
  - allow by `company_wide_access` OR department match OR direct assignment match.
- Responsibility check:
  - approve action requires `workflow_approver:disclosure`, else `RESPONSIBILITY_REQUIRED`.

## Verification
- `go build ./...` passed.
- `go test ./...` passed.

## Known limitation
- Resolver/repository/checker currently use in-memory fixtures for bootstrap.
- Detailed policy mapping and persistent MySQL repositories will be expanded in later phases.

## Next
- P0.6: implement `/api/v1/me`, `/api/v1/me/companies`, `/api/v1/me/effective-access`, `/api/v1/me/capabilities`, `/api/v1/me/membership`.
