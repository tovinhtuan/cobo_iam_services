# 14 — Worker resume health

## Resume

```bash
cd /root/cobo_project
docker compose -f docker-compose.artifacts.yml start worker
```

```text
PERIODIC_WORKER_RESUMED=true
PERIODIC_WORKER_INSTANCE_COUNT_AFTER_RESUME=1
WORKER_DUPLICATE_INSTANCE=false
PERIODIC_SEEDING_ENABLED=true
WORKFLOW_SNAPSHOT_ENABLED=true
DEV_PERIODIC_RUNTIME_CONFIG_CHANGED=false
```

## Observation

- Worker ticks reference **only** `bang-tinh-luong-nhan-vien-ban-sao-2`
- `periodic cycles seeded` messages are KEEP-scoped (idempotent; KEEP cycle count remained 8)
- No panic / foreign key / FATAL in worker or API after commit
- Deleted-root cycles/records count after resume: **0**
- Template roots after resume: **1**

```text
DELETED_TEMPLATE_WORKER_ERRORS=0
POST_RESUME_DELETED_ROOT_ACTIVITY=0
NEW_SERVER_P0_P1=0
```

Note: KEEP immutability was proven **before** worker resume. Post-resume KEEP cycle count may increase later if new slots become eligible — that is legitimate runtime, not cleanup corruption.
