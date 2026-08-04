# Phase 2 — Shared companyplan.Reader + repository hardening

## Scope (confirmed start by user 2026-08-04)

Keep Phase 0–1 contracts. No handler/DTO/API, no FE, no DEV migrate apply, no deploy, no CompanyTierResolver.

## Mandatory pre-conclusion items

### 1) Migration 0126 audit → DEV-only seed

| Before | After |
|--------|--------|
| Numbered `0126_dev_company_subscription_fixtures.{up,down}.sql` | **Removed** from migrations tree |
| Risk: any tool treating `NNNN_*.up.sql` as shared chain | Mitigated |

**DEV-only mechanism (repo convention):**
- `migrations/seed_dev_company_subscriptions.sql` — idempotent `INSERT … ON DUPLICATE KEY UPDATE`
- Wired **only** in `migrations/run_dev_migrations.sh` after `0125_company_subscriptions.up.sql`
- Cleanup documented in seed header: `DELETE … WHERE origin='dev_fixture'`
- Does **not** alter production-like company data; fixtures target DEV ids `c_001` / absence on `c_002`

`0125` remains the sole numbered schema migration for `company_subscriptions`.

### 2) Overlap concurrency audit

| Item | Finding |
|------|---------|
| Target DB | MySQL 8.0 (compose `mysql:8.0`) |
| Isolation | Explicit `sql.LevelRepeatableRead` on Create TX |
| Supporting index | `idx_company_subscriptions_lookup (company_id, status, effective_from, expires_at)` |
| Gap when zero rows | `SELECT … FOR UPDATE` on `company_subscriptions` alone **does not** serialize first insert for a company |
| Fix (convention-compatible) | Lock parent `companies.company_id FOR UPDATE`, then lock occupying subscription rows, then overlap check + INSERT |
| Global advisory lock | **Not** used |

**Proof status this environment:** MySQL not reachable (`127.0.0.1:3306` connection refused).

```
MYSQL_CONCURRENCY_VALIDATION_PENDING_PHASE_5
```

Integration test present: `TestMySQLCreate_ConcurrentOverlap_EmptyCompany` (2 goroutines; expects 1 success + 1 `ErrOverlap`). Skips with the marker above when DSN/ping/schema unavailable. Re-run in Phase 5 with DEV MySQL after `0125` applied.

### 3–4) Shared Reader / batch

- `Service` (`NewService(Reader)`) — public shared façade; **no cache**
- Reuses Phase 1 MySQL/Memory repositories (not reimplemented)
- `GetEffectivePlan` / `GetEffectivePlans`: no-plan=`nil`/omit; non-ACTIVE keeps real status; unknown covering code still returned (FE fail-close); DB errors propagated
- Batch: dedupe IDs; empty/blank → empty map **without** reader call; keyed by `company_id`; no fake plans; same `SelectEffectivePlan` rule

## Explicitly not done
- Handler/route/DTO / GetOwnCompany / `/me/companies`
- DEV migrate apply / deploy / FE
- CompanyTierResolver / `user_subscription_tiers`

## Quality gates
- `go test ./internal/subscription/...` PASS (concurrency SKIP + marker)
- `go vet ./internal/subscription/companyplan/` PASS
- `gofmt` applied
- `git diff --check` clean
- Docker: see `07-phase2-verification.md`
