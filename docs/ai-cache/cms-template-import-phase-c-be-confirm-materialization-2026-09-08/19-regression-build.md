# Regression & Build Verification

## Go Package Tests
1. Targeted Confirm App Suite:
   - Command: `go test -v ./internal/disclosure/app -run "TestTemplateImportConfirm"`
   - Result: 9 sub-suites passed (0.011s) — PASS.
2. Targeted Confirm HTTP Handler Suite:
   - Command: `go test -v ./internal/disclosure/transport/http -run "TestCmsConfirmTemplateImport"`
   - Result: 8 tests passed (0.003s) — PASS.
3. Full Disclosure Module Regression:
   - Command: `go test ./internal/disclosure/...`
   - Result: ALL packages passed — PASS.
   - Packages verified:
     - `internal/disclosure/app`
     - `internal/disclosure/app/applicability`
     - `internal/disclosure/app/deadlineengine`
     - `internal/disclosure/app/legal_basis_backfill`
     - `internal/disclosure/app/legal_basis_inventory`
     - `internal/disclosure/app/periodic_oneshot`
     - `internal/disclosure/infra/inmemory`
     - `internal/disclosure/infra/mysql`
     - `internal/disclosure/infra/workflow`
     - `internal/disclosure/transport/http`

## Go Build Verification
- Command: `go build ./...` (cwd = `cobo_iam_services`)
- Exit code: `0` — PASS.

## Docker Build Verification
- Command: `docker compose -f docker-compose.dev.yml build api` (cwd = `cobo_iam_services`)
- Exit code: `0` — PASS.
- Image produced: `cobo_iam_services-api` (sha256:8973db7662e3a4be774a1c164e92f6ee79c5a1d87f45723371ecfa30717bf334).
