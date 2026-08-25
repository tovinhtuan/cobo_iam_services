# 06 — Performance / index review

## Phase 3 EXPLAIN (unchanged query)

```text
dr: ALL ~17 rows, filesort (ORDER BY created_at)
dt/dtv: eq_ref PRIMARY
EXISTS pc / NOT EXISTS pc_ir: ref idx_pc_pending (record_id, due_date)
```

## Schema indexes (source)

```text
idx_disclosure_company_status (company_id, status)           — 0004
idx_disclosure_records_submitted_at (company_id, submitted_at) — 0131
idx_pc_pending (record_id, due_date)                        — 0039
```

## Reasoning

```text
PRODUCTION_CARDINALITY=UNKNOWN (no prod stats)
LOWER(TRIM(status)) may inhibit idx_disclosure_company_status use → ALL scan risk at scale
EXISTS on record_id is indexed (idx_pc_pending leading column)
filesort acceptable after company/status filter if result small
DEV ~17 rows ≠ production proof
```

```text
PERFORMANCE_RELEASE_RISK=P1_FOLLOW_UP
PERFORMANCE_REASON=
  Correctness OK; company+status functional expression may prevent index use at scale;
  EXISTS path healthy; no evidence of DEV/runtime pathological plan; index not required before commit
INDEX_CHANGE_REQUIRED_BEFORE_COMMIT=false
NEW_INDEX_CREATED=false
```
