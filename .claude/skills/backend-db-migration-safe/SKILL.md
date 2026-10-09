---
name: backend-db-migration-safe
description: Dùng khi thay đổi schema MySQL hoặc semantics dữ liệu trong cobo_iam_services - viết migration NNNN_*.up/down.sql, thêm vào run_dev_migrations.sh, expand-migrate-contract, mixed-version deploy, backfill, lock risk, rollback.
---

# Backend DB Migration Safe

## When to use
- Add/change tables, columns, constraints, indexes.
- Backfill data; change read/write semantics.

## Project facts (verify)
- Files `migrations/NNNN_name.up.sql` + `.down.sql` (some legacy ones have no
  down). Pick the next free number.
- Runner `migrations/run_dev_migrations.sh` applies an explicit ordered
  `MIGRATIONS=` list and records `schema_migrations(file_name)`.
  **A new file not added to that list never runs** (`deploy-dev.sh` reads the
  same list). Known gaps: 0038, 0098, 0099, 0150 were not listed.
- Supporting files: `dryrun_*.sql`, `verify_*.sql`, `seed_*.sql`; some
  migrations have Go tests (`migrations/*_test.go`).
- MySQL 8; outbox uses `SKIP LOCKED`.

## Workflow
1. Current vs desired schema.
2. Compatibility between old and new code (API + worker run old/new mixed).
3. Strategy: expand → migrate → contract.
4. Old/null/default data handling; backfill in batches.
5. Lock/performance risk (`ALTER` on large tables, index builds).
6. Add to `run_dev_migrations.sh`; write `verify_*.sql` if data changes.
7. Rollback considerations (down file or forward-only rationale).
8. Affected APIs/workers and in-memory repo parity.

## Guardrails
- No schema change without mixed-version analysis.
- No risky NOT NULL / UNIQUE / FK without a backfill plan.
- No semantic data change without updated tests.
- Never edit an already-applied migration; add a new one.

## Output format
- Schema delta
- Compatibility analysis
- Rollout steps
- Risk points
- Validation queries/tests
