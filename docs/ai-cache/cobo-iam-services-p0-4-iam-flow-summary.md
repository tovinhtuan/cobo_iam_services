# P0.4 IAM Flow Summary

## Completed
- Implemented IAM service use-cases in `internal/iam/app/service.go`:
  - `Login`
  - `Refresh`
  - `Logout`
  - `SelectCompany`
  - `SwitchCompany`
- Added in-memory adapters for P0 bootstrap:
  - Credential verifier (`internal/iam/infra/inmemory/credentials.go`)
  - Session repository (`internal/iam/infra/inmemory/sessions.go`)
  - Token manager as issuer + inspector (`internal/iam/infra/inmemory/tokens.go`)
  - Membership query fixture service (`internal/companyaccess/infra/inmemory/membership_query.go`)
- Added HTTP handlers for auth endpoints:
  - `POST /api/v1/auth/login`
  - `POST /api/v1/auth/refresh`
  - `POST /api/v1/auth/logout`
  - `POST /api/v1/auth/select-company`
  - `POST /api/v1/auth/switch-company`
- Wired handlers into API bootstrap (`cmd/api/main.go`).

## Contract alignment
- JSON response/request DTOs follow `docs/api-contracts-json.md`.
- Login supports:
  - single active company => access token + `current_context` + `next_action=load_effective_access`
  - multiple companies => pre-company token + memberships + `next_action=select_company`
- Refresh requires existing company context; otherwise returns `COMPANY_CONTEXT_REQUIRED`.
- Select/Switch company parse bearer token context and issue new company-bound access token.

## Verification
- `go build ./...` passed.
- `go test ./...` passed (no test files yet).

## Known limitation (expected in P0)
- Token/session/membership stores are in-memory bootstrap adapters (non-persistent).
- Audit append for IAM actions will be wired in P0.7.

## Next
- P0.5: implement authorization core (`Authorize`, `AuthorizeBatch`, `GetEffectiveAccess`) and internal authorize APIs.
