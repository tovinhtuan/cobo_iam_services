# Phase 2 diff-scope audit

| Path | Classification | Notes |
|------|----------------|-------|
| `internal/subscription/companyplan/service.go` | REQUIRED | Shared Reader façade |
| `internal/subscription/companyplan/service_test.go` | REQUIRED | Semantics + concurrency skip marker |
| `internal/subscription/companyplan/mysql_repository.go` | REQUIRED | Parent company FOR UPDATE |
| `internal/subscription/companyplan/repository.go` | REQUIRED | `ErrCompanyNotFound` |
| `migrations/seed_dev_company_subscriptions.sql` | REQUIRED_DEV_SEED | Replaces 0126 |
| `migrations/run_dev_migrations.sh` | REQUIRED | Register 0125 + seed |
| `migrations/0126_dev_company_subscription_fixtures.*` | REMOVED | Must not stay on shared numbered chain |
| `docs/ai-cache/company-premium-implementation-2026-08-04/06–08*` | REQUIRED_DOCS | Evidence |
| `docs/ai-cache/README.md` / `reusable-task-updates.md` | REQUIRED_DOCS | Pointers |
| Handlers / DTO / FE / deploy scripts | FORBIDDEN | Not touched |

## Exact exclusion check

- No `internal/companyaccess` handler edits
- No `cmd/api` wiring
- No Frontend
- No `CompanyTierResolver` / `user_subscription_tiers` usage for company plan
