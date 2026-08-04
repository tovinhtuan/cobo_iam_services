# Phase 4 — Migration 0125 static validation

**No DEV apply / no DEV seed / no disposable MySQL available** (`127.0.0.1:3306` refused).

## 0125 up

| Check | Result |
|-------|--------|
| Syntax `CREATE TABLE company_subscriptions` | OK |
| FK `REFERENCES companies (company_id)` | OK |
| `company_id VARCHAR(36)` matches `companies.company_id` | OK |
| Indexes `idx_company_subscriptions_lookup`, `idx_company_subscriptions_origin` | OK |
| `effective_from NOT NULL`, `expires_at NULL` (open-ended) | OK |
| `origin VARCHAR(64)` | OK |
| No fixture INSERTs in numbered up | OK |

## 0125 down

- `DROP TABLE IF EXISTS company_subscriptions` — child table dropped first; OK rollback order.

## Runner / seed

| Check | Result |
|-------|--------|
| `run_dev_migrations.sh` lists `0125` then `seed_dev_company_subscriptions.sql` | OK |
| Numbered `0126*` absent | OK |
| Seed idempotent `ON DUPLICATE KEY UPDATE` | OK |
| Cleanup docs: `DELETE … WHERE origin='dev_fixture'` | OK |
| Seed does not mutate companies beyond fixture insert target | OK |

## Occupying / half-open

Enforced in application domain (`ACTIVE|TRIAL|SUSPENDED`, half-open windows); schema supports NULL `expires_at`. Static test: `TestMigration0125_StaticValidation`.

## Disposable MySQL apply/inspect/down

**Not run** — no local MySQL. Do **not** claim schema live-validated. Phase 5 DEV apply remains the live gate.
