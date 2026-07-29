# Phase 12.6A — Diff audit

## Code (cobo_iam_services)

- `cmd/legal-basis-inventory/main.go` — `--docker-dev`, allowlisted Open, SET SESSION READ ONLY + RR inventory tx, schema probe for `is_released`, Groups A–E dry-run reports
- `internal/disclosure/app/legal_basis_inventory/sql_allowlist.go` (+ tests) — fail-closed SQL allowlist interceptor
- Existing analyzer: `classify.go` / `classify_test.go`

## Not touched

- No write repository / migration / API handler mutation
- No Phase 12.6B apply tool
- Docker Compose files **not** modified; containers **not** recreated for inventory (MySQL was already running / previously started with approval)

## Evidence

See `phase-12-6a-*.json` / `.md` in this directory (mirrored under cobo_web_design plan folder).
