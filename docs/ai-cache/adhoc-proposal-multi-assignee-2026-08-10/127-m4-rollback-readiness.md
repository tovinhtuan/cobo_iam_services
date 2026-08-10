# Rollback readiness

Pre-rollout captured: prior api/worker containers, FE assets BbV9fZ0F/JvPESSmx, migration 0127.

After v3 tasks materialized with assignee_membership_id=NULL + relation rows:

- Do **not** blindly roll back to pre-M2 runtime (assumes singular NOT NULL).
- Safe options: keep new BE; FE-only rollback OK; or drain/disable v3 creation.
- Marker: ROLLBACK_REQUIRES_V3_TASK_DRAIN_OR_COMPATIBILITY_PLAN
- ROLLBACK_READY = false
