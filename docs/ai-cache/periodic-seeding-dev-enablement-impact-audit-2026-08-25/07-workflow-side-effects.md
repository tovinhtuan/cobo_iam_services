# 07 — Workflow / task / notification side effects

When `WORKFLOW_SNAPSHOT_ENABLED=true` and creator wired with workflow:

```text
MATERIALIZATION_SIDE_EFFECTS=
1. CreateRecord (Draft, planned_date from cycle due)
2. CreateWorkflowInstanceInternal (snapshot + tasks for steps)
3. UpdateCycleRecord (link record_id + materialized_at)
4. Claim/release via materialized_at

WORKFLOW_CREATED_ON_MATERIALIZATION=true  # if snapshot enabled + non-empty workflow
TASKS_CREATED_ON_MATERIALIZATION=true     # via workflow instance steps
NOTIFICATIONS_TRIGGERED_ON_MATERIALIZATION=false  # no email in materialize path itself
```

Reminders remain separate worker jobs (already enabled); new records may later enter reminder pipelines — not immediate email blast from Seed.

Empty effective workflow: fail **before** CreateRecord when workflowOn=true (safe skip).
