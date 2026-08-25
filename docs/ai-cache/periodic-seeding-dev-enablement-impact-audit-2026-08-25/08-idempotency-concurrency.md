# 08 — Idempotency + concurrency

```text
CYCLE_IDEMPOTENCY_KEY=(type_id, company_id, cycle_label)
CYCLE_UPSERT_IDEMPOTENT=true
RECORD_MATERIALIZATION_IDEMPOTENT=true  # TryClaimPeriodicCycle single-winner; skip if record_id set
WORKFLOW_MATERIALIZATION_IDEMPOTENT=true  # tied to successful claim + UpdateCycleRecord; empty-workflow skips before create when snapshot on

DUPLICATE_CURRENT_SLOT_CYCLE_RISK=LOW

DEV_WORKER_INSTANCE_COUNT=1
MULTI_WORKER_CONCURRENCY=false

CYCLE_CREATE_TRANSACTIONAL=false  # single INSERT upsert per combo; errors continue
RECORD_CREATE_TRANSACTIONAL=partial  # claim → create → update; release on failure
CYCLE_RECORD_LINK_TRANSACTIONAL=false  # separate statements
PARTIAL_FAILURE_RECOVERY=ReleasePeriodicCycleClaim; retry next tick if claim released

ONE_TEMPLATE_FAILURE_ABORTS_BATCH=false  # continue remaining companies/types (seed); materialize aborts batch only on claim/update hard errors
FAILED_ITEM_RETRIED_NEXT_TICK=true
```

**Orphan risk when snapshot off:** CreateRecord succeeds then materialize rejects empty workflow ID → claim released → **new Draft every tick** (not idempotent). P0.
