# 06 — Worker-only restart

```text
Command: docker compose -f docker-compose.artifacts.yml up -d --force-recreate --no-deps worker
WORKER_ONLY_RESTART=true
WORKER_RESTART=PASS
RUNNING_PERIODIC_SEEDING_ENABLED=true
RUNNING_WORKFLOW_SNAPSHOT_ENABLED=true
WORKER_INSTANCE_COUNT=1
FIRST_PERIODIC_TICK_AT≈2026-08-25T10:07:42Z (UTC)
```
