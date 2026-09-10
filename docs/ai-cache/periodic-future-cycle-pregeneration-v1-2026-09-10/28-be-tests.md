# BE tests

Package: internal/disclosure/app + infra/mysql
Command: `go test ./internal/disclosure/app/ ./internal/disclosure/infra/mysql/ -count=1`
Result: PASS (2026-09-10)

Coverage includes: lead validation, GenerateAt monthly/quarterly/yearly, AF skip, AT none, company T, next-only, idempotency, preopen/at/after OpenAt materializer, becomes-current, no backfill, immutability
