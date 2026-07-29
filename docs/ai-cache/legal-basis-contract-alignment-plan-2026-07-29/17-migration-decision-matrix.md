# 17 — Migration decision matrix (Groups A–E)

**Timing:** OD-4 Hybrid — dry-run before mutate; **DEV only** in Phase 12; **no production**.

| Group | Predicate | Auto mutate? | Action | Idempotency |
| --- | --- | --- | --- | --- |
| **A** | flat ≠ empty ∧ structured empty | **Yes** (after dry-run) | Wrap one item: title=`Cơ sở pháp lý`, summary=full flat, other fields `""`, generate `id` | Skip if `legal_bases` already non-empty |
| **B** | flat empty ∧ structured non-empty | **Yes** | Keep structured; derive flat via OD-7 | Safe re-derive |
| **C** | both populated ∧ flat == projection(structured) | **Yes** | Keep structured; normalize flat/block sync | Safe |
| **D** | both populated ∧ flat ≠ projection | **No** | Report only; runtime structured wins; manual BA review | N/A auto |
| **E** | both empty | **No-op** | — | — |

## Dry-run output (LOCKED contract)

```text
type_id, version_no, group, flat_len, structured_count, flat_hash, projection_hash, action_proposed
```

No full legal text in logs.

## Ops contract (implementation)

| Item | Lock |
| --- | --- |
| batch size | configurable; default **100** rows/version |
| retry | transient DB errors; max 3; exponential backoff |
| idempotency key | `(type_id, version_no, group, algorithm_version=v1)` |
| snapshot | mysqldump / table export before mutate batch |
| rollback | restore snapshot; document in runbook |
| audit counters | processed / skipped / failed / group_d_reported |

## Group D manual path

1. Export dry-run D rows.
2. BA decides: keep structured (overwrite flat) OR keep flat (rewrite structured from flat — **rare**, explicit).
3. Apply one-off tool with explicit type_id list.
4. Never silent overwrite.


## Phase 12.6B-Plan addendum (DEV Controlled Backfill)

| Item | Lock |
| --- | --- |
| Dataset | **LOCKED_ALL_6** exact Group A allowlist |
| Flat after wrap | OD-7 projection (`Cơ sở pháp lý`); full text in `summary` → expected Group **C** |
| Transaction | Single txn, 6 rows, all-or-nothing + CAS |
| Migration 0122 / `is_released` | **Out of scope** for this apply |
| Apply execution | Blocked until Approvals 3–4 + explicit user phrase |
| Evidence | `phase-12-6b-*` |

## Phase 12.6B-I (2026-07-29)

Guarded backfill tooling implemented (`legal_basis_backfill` + cmds). Verdict **TOOL_READY_FOR_CONTROLLED_EXECUTION**. DEV apply **not** executed; SQL DEV wiring deferred to 12.6B-E.
