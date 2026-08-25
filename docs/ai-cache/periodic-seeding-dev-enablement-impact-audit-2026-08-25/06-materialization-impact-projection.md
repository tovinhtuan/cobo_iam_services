# 06 — Materialization projection

```text
MATERIALIZATION_LOOKAHEAD=7d
ListPendingCycles: record_id IS NULL AND materialized_at IS NULL
  AND COALESCE(open_at, cycle_start, due_date) <= now+7d
LIMIT 200
```

```text
PROJECTED_MATERIALIZATION_ELIGIBLE_CYCLES=52  # all projected new cycles within lookahead
PROJECTED_NEW_DISCLOSURE_RECORDS_ON_FIRST_RUN=52  # UPPER BOUND if workflow path succeeds
CYCLE_WITH_RECORD_ID_MATERIALIZE_BEHAVIOR=SKIP
```

**P0 dependency:** running worker lacks `WORKFLOW_SNAPSHOT_ENABLED` → default false → `workflowOn=false` →
`CreateAndSubmitRecordWithPlannedDate` creates Draft then returns empty workflowInstanceID →
materialize releases claim without linking → **orphan Draft records every tick**.

Therefore projected record/workflow counts above are **only valid if** worker also has
`WORKFLOW_SNAPSHOT_ENABLED=true` (and templates have non-empty effective workflow).

```text
MATERIALIZED_RECORD_INITIAL_STATUS=Draft
MATERIALIZED_RECORD_SUBMITTED_AT=NULL
PERIODIC_RECORD_REQUIRES_COMPANY_ACTION=true
SkipCompanySubmit=true means materialize ≠ company submit
```
