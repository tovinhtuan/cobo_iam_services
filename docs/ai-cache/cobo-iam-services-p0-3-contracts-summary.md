# P0.3 Contracts Summary

## Completed
- Added module boundaries and contract files for P0.3:
  - `internal/iam/domain/types.go`
  - `internal/iam/app/contracts.go`
  - `internal/companyaccess/domain/membership.go`
  - `internal/companyaccess/app/contracts.go`
  - `internal/authorization/domain/model.go`
  - `internal/authorization/app/contracts.go`
  - `internal/audit/app/contracts.go`
  - `internal/platform/events/event.go`
  - `internal/platform/outbox/contracts.go`
- Added module folder scaffolding for `iam`, `companyaccess`, `authorization`, `audit` with `domain/app/infra/transport`.

## Contract highlights
- IAM service methods defined: `Login`, `Refresh`, `Logout`, `SelectCompany`, `SwitchCompany`.
- Membership query service and repository ports defined for memberships/roles/departments/titles.
- Authorizer service methods defined: `Authorize`, `AuthorizeBatch`, `GetEffectiveAccess`.
- Authorization request/decision DTOs aligned with `docs/api-contracts-json.md`.
- Audit append service and repository ports defined.
- Outbox publisher/repository ports defined for worker-ready flow.

## Verification
- `go build ./...` passes.
- Lints for new contract packages: clean.

## Next
- P0.4: implement IAM login/refresh/logout/select-company/switch-company use-cases and transport handlers.
