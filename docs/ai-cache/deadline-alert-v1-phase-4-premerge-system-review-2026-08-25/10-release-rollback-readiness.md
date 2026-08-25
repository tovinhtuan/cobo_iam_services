# 10 — Release / rollback readiness

## Delta

```text
BEFORE: Draft actionable hidden; post-submit could remain / false OVERDUE
AFTER:  Draft+unsubmitted after OpenAt visible; Submit removes alert; internal delay ≠ Company OVERDUE
```

## Deploy

```text
BE-only redeploy sufficient
FEATURE_FLAG_REQUIRED=false
DB_MIGRATION_CREATED=false
DATA_BACKFILL_CREATED=false
WORKER_CHANGED=false
PERIODICITY_CHANGED=false
REMINDER_CHANGED=false
MATERIALIZATION_STATUS_CHANGED=false
```

## Rollback

```text
ROLLBACK_MODEL=revert application code + redeploy BE
ROLLBACK_DATA_STEP_REQUIRED=false
```

## Monitoring (future production — not executed)

```text
deadline-alerts 5xx / latency / row counts / slow query / duplicates / alert-count spike
```

```text
READY_FOR_PRODUCTION_DEPLOY=false (separate gate)
```
