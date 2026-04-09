# P0.7 Audit and Outbox Summary

## Completed
- Implemented audit append baseline:
  - `internal/audit/appimpl/service.go`
  - `internal/audit/infra/inmemory/repository.go`
- Integrated IAM transport with audit append hooks (best effort):
  - login success/failure
  - logout
  - select-company success/failure
  - switch-company success/failure

- Implemented outbox baseline:
  - `internal/platform/outbox/publisher.go`
  - `internal/platform/outbox/processor.go`
  - `internal/platform/outbox/inmemory/repository.go`
  - `internal/platform/outbox/inmemory/bootstrap.go`

- Integrated IAM transport with outbox publish hooks:
  - `iam.session.login`
  - `iam.session.logout`
  - `iam.company.selected`
  - `iam.company.switched`

- Upgraded worker skeleton to polling processor:
  - `cmd/worker/main.go` now runs outbox processor on each tick
  - includes bootstrap seeded event and registered sample handler `notification.dispatch`

- Wired API bootstrap with audit + outbox implementations in `cmd/api/main.go`.

## Verification
- `go build ./...` passed.
- `go test ./...` passed.

## Notes
- P0 implementation remains in-memory/bootstrap (non-persistent by design).
- Audit append and outbox publish are non-blocking to keep request path stable during bootstrap.

## Next
- P1 or next hardening step: replace in-memory audit/outbox with MySQL repositories + add deterministic tests.
