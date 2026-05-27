# Batch 2 — Periodic worker, claim, cycle_start, prefs RBAC

**Date:** 2026-05-27  
**Contract:** business-contract-workflow-deadline-final.md v1.1-final (AC-1, AC-7, AC-9, AC-10 worker path, §6.3)

## Delivered

- Migration `0080_periodic_cycles_cycle_start` (`cycle_start DATE`, no P0 backfill).
- Seed persists `cycle_start`; materialize uses `CycleStart` as T0 (not `now`).
- Optimistic claim via `materialized_at` single-winner UPDATE; release on failure.
- Materialize completes only when `record_id` + `workflow_instance_id` both set.
- Worker wires `workflowSvc` with `SnapshotEnabled: true` when `WORKFLOW_SNAPSHOT_ENABLED`.
- Preferences: GET `disclosure.view`, PATCH `disclosure.auto_create.manage`.
- `workflow/errs`: `ErrEmptyWorkflowSnapshot`, `IsEmptyEffectiveWorkflow` (avoids import cycle).

## Claim strategy

`TryClaimPeriodicCycle`: `UPDATE ... SET materialized_at = NOW(3) WHERE record_id IS NULL AND materialized_at IS NULL` → `RowsAffected == 1`.  
`ListPendingCycles` excludes `materialized_at IS NOT NULL`.  
`ReleasePeriodicCycleClaim` clears claim on failure.

## Not in Batch 2

- BE-PER-02b `error_count` (P1).
- P0 backfill for legacy cycles without `cycle_start` (skipped if `CycleStart` zero).
- FE / Batch 3.

## Verify

```bash
go test ./internal/disclosure/app/... ./internal/workflow/... ./internal/adhoc/...
go build -o /dev/null ./cmd/api ./cmd/worker
```
