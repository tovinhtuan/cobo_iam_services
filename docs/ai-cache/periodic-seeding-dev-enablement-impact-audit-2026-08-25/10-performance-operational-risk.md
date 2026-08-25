# 10 — Load + ops

```text
PROJECTED_FIRST_RUN_LOAD=MEDIUM
  scan 14 templates × 20 companies; ~52 upserts; ≤52 materializations (+ workflows)
STEADY_STATE_QUERY_LOAD=MEDIUM
  every 5s: ListActivePeriodicTypes + ListAllActiveCompanyIDs + prefs + per-company profile + Upsert NOOP×52
  + ListPendingCycles (empty after catch-up)
FIVE_SECOND_CADENCE_RISK=MEDIUM
EXPECTED_PERIODIC_LOG_VOLUME=HIGH
  seeded return counts NOOPs → Info "periodic cycles seeded" likely every 5s with seeded≈52
ENABLEMENT_OBSERVABILITY=LIMITED
  logs on seeded>0 / materialized>0 / errors; no metrics counters
HEALTH_ENDPOINT_DETECTS_PERIODIC_JOB_FAILURE=false
```

Baselines (pre-enable, read-only):

```text
periodic_cycles=5
disclosure_records=17
workflow_instances=6
```
