# Phase 2 verification

## Commands

```bash
gofmt -w internal/subscription/companyplan/*.go
go test ./internal/subscription/...
go test ./internal/subscription/companyplan/ -count=1 -v
go vet ./internal/subscription/companyplan/
git diff --check
docker compose -f docker-compose.dev.yml build api
```

## Results (2026-08-04)

| Gate | Result |
|------|--------|
| `go test ./internal/subscription/...` | PASS |
| companyplan unit/service tests | PASS |
| `TestMySQLCreate_ConcurrentOverlap_EmptyCompany` | SKIP — `MYSQL_CONCURRENCY_VALIDATION_PENDING_PHASE_5` |
| `go vet` companyplan | PASS |
| `git diff --check` | clean |
| `docker compose … build api` | PASS (exit 0) |

## Diff scope (expected)

- `internal/subscription/companyplan/service.go` (new)
- `internal/subscription/companyplan/service_test.go` (new)
- `internal/subscription/companyplan/mysql_repository.go` (parent FOR UPDATE)
- `internal/subscription/companyplan/repository.go` (`ErrCompanyNotFound`)
- `migrations/seed_dev_company_subscriptions.sql` (new)
- `migrations/run_dev_migrations.sh` (+0125 + seed)
- delete `migrations/0126_dev_company_subscription_fixtures.*`
- docs under `docs/ai-cache/company-premium-implementation-2026-08-04/`

Out of scope if present in working tree: unrelated dirty files — do not discard; do not include in Phase 2 commit.
