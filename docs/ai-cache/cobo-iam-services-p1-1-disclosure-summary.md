# P1.1 Disclosure Module Skeleton Summary

## Completed
- Added disclosure module skeleton with clean boundaries:
  - `internal/disclosure/domain/record.go`
  - `internal/disclosure/app/contracts.go`
  - `internal/disclosure/app/service.go`
  - `internal/disclosure/infra/inmemory/repository.go`
  - `internal/disclosure/transport/http/handler.go`

## APIs exposed
- `POST /api/v1/disclosures` (CreateRecord)
- `PATCH /api/v1/disclosures/{record_id}` (UpdateRecord)
- `POST /api/v1/disclosures/{record_id}/submit` (SubmitRecord)
- `POST /api/v1/disclosures/{record_id}/confirm` (ConfirmRecord)
- `GET /api/v1/disclosures` (ListRecords)
- `GET /api/v1/disclosures/{record_id}` (GetRecord)

## Access control and tenant isolation
- Subject context extracted from access token: `user_id`, `membership_id`, `company_id`.
- Every service method performs explicit authorization check via central `authorization.Service`.
- Repository keying and reads are `company_id` scoped (`company_id:record_id`) to enforce tenant isolation.

## Bootstrap behavior
- In-memory repository and state transitions:
  - create => `draft`
  - submit => `submitted`
  - confirm => `confirmed` (requires `submitted`)
- Authorization decisions currently use existing in-memory auth policy map.

## Verification
- `go build ./...` passed.
- `go test ./...` passed.

## Next
- Continue roadmap with P1.2 Workflow module skeleton.
