# Phase 12.6A — Inventory plan

## Goal

Read-only DEV inventory + in-memory dry-run Groups A–E; no DB mutation; stop before 12.6B.

## Steps

1. Prove RO grants + READ ONLY transaction.
2. Schema verify (`information_schema`).
3. Keyset SELECT required columns only (batch 100–500).
4. Local classify via OD-7 helpers (reuse `disclosure/app`).
5. SQL approx A/B/E + BOTH(C+D) via JSON_TABLE — reconcile with analyzer.
6. Emit redacted reports + idempotency run2.
7. Decision package for 12.6B (no execute).

## Dry-run actions

| Group | Action |
| --- | --- |
| A | WRAP_LEGACY_FLAT (`<NEW_UUID>` simulated) |
| B | PROJECT_STRUCTURED |
| C | NORMALIZE_MATCHED |
| D | MANUAL_REVIEW |
| E | NO_OP |
| Malformed/overflow | BLOCKED_* |

## Current execution

**Stopped at step 1** — no RO DSN in agent environment (`BLOCKED_READ_ONLY_ACCESS`).
