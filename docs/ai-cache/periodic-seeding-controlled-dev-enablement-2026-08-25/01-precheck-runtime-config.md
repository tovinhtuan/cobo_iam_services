# 01 — Precheck runtime

```text
DEV_HOST=avi-server1 (via deploy-dev.local.env)
DEV_PROJECT_PATH=/root/cobo_project
WORKER_SERVICE_NAME=worker
WORKER_CONTAINER_NAME=cobo-iam-worker
WORKER_INSTANCE_COUNT=1
API_INSTANCE_COUNT=1
FE_INSTANCE_COUNT=1

ENABLEMENT_PRECHECK_HCM_TIME=2026-08-25 17:05:53 +0700
HCM_TIME_GATE=PASS

PRE_PERIODIC_SEEDING_ENABLED=false
PRE_WORKFLOW_SNAPSHOT_ENABLED=ABSENT (default false in process)
EFFECTIVE_WORKER_CONFIG_SOURCE=/root/cobo_project/.env via docker-compose.artifacts.yml env_file
```
