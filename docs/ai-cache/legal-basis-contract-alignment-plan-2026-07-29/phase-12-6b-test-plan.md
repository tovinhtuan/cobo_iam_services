# Phase 12.6B — Future tool test plan

**Not executed in Plan phase.** Synthetic fixtures only; no DEV mutation in CI unit tests.

## Guard tests

- Default path without `--apply` → 0 writes
- Wrong environment / DB name → refuse
- Allowlist count ≠ 6 / missing snapshot / bad token → refuse
- Stale allowlist hash → refuse
- Group D / malformed / overflow present → refuse

## Transformation tests

- 6 synthetic Group A rows → wrap + OD-7 flat
- Unicode / multiline summary preserved exactly
- Projection equals stored `legal_basis`
- UUID unique / not `*-lb-legacy-1`

## Transaction tests

- All 6 success → commit
- Fail on record 3 → full rollback (0 lingering writes)
- RowsAffected 0 or >1 → rollback
- Read-back mismatch → rollback

## Rollback tests

- Exact restore from snapshot
- Stale post-state refuses rollback
- Verification idempotent

## Inventory regression

- Analyzer still partitions A–E; after simulate wrap → Group C
