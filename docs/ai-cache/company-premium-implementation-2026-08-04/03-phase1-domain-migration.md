# Phase 1 — Domain + migration foundation

## Locked implementation decisions (user 2026-08-04)

1. **Reader returns full commercial plan** covering `at` with actual `status`; `nil` only when no covering record. Badge filtering is FE-only (`PREMIUM`+`ACTIVE`+`COMPANY_SUBSCRIPTION`).
2. **Overlap = reject** for occupying statuses (`ACTIVE`|`TRIAL`|`SUSPENDED`) with intersecting half-open windows. Enforcement: `BEGIN` → `SELECT … FOR UPDATE` → validate → `INSERT` → `COMMIT`. No partial unique index.
3. **DEV fixtures:** migration `0126_dev_company_subscription_fixtures` — `c_001` PREMIUM ACTIVE (`origin=dev_fixture`); `c_002` no row. Cleanup: `.down.sql` or `DELETE … WHERE origin='dev_fixture'`.
4. **Migration number:** verified latest on HEAD was `0124` → schema `0125_company_subscriptions`.

## Delivered

### Package `internal/subscription/companyplan/`
- `types.go` — codes/statuses/source/origin
- `overlap.go` — window overlap + `SelectEffectivePlan`
- `validate.go` — create validation
- `repository.go` — Reader/Writer/Repository interfaces
- `mysql_repository.go` — MySQL Case C SoT (not wired to HTTP yet)
- `memory_repository.go` — unit-test double
- `companyplan_test.go` — targeted tests

### Migrations
- `0125_company_subscriptions.up.sql` / `.down.sql`
- `0126_dev_company_subscription_fixtures.up.sql` / `.down.sql`

## Explicitly not done
- API exposure / handler wiring
- DEV migrate apply / deploy
- Frontend
- CompanyTierResolver / user_subscription_tiers usage
