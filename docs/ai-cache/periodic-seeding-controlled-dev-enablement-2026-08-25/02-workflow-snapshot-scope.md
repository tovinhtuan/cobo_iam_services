# 02 — WORKFLOW_SNAPSHOT_ENABLED scope

```text
WORKFLOW_SNAPSHOT_FLAG_SCOPE_RECONCILED=PASS
Loaded: config.Load boolEnv WORKFLOW_SNAPSHOT_ENABLED default false (startup only)
Worker: cmd/worker/main.go — only when PeriodicSeedingEnabled, wires workflowSvc with SnapshotEnabled=true
API: httpserver also uses flag for workflow service (already true on DEV API)
Does NOT: enable reminders/outbox by itself; send notifications; mutate existing records
Creation path only for new workflow instances during materialization

WORKFLOW_SNAPSHOT_UNRELATED_SIDE_EFFECTS=NONE for worker beyond workflow instance creation on materialize
SNAPSHOT_FLAG_SCOPE_DRIFT=false
```
