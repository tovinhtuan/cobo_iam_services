# 01 — DEV environment / deploy

```text
DEV_REMOTE=avi-server1 (88.216.208.0:21239)
DEV_PATH=/root/cobo_project
DEV_COMPOSE_FILE=docker-compose.artifacts.yml
DEV_API_BASE=http://88.216.208.0:8080
DEV_FE_BASE=http://88.216.208.0:3000
DEV_DEPLOY_COMMAND=sh deploy-dev.sh be --skip-tests
DEV_DEPLOY_SCOPE=BE_ONLY
FE_SOURCE_CHANGED=false
FE_DEPLOY_ONLY=false
DB_MIGRATION_REQUIRED=false
NEW_DEADLINE_ALERT_MIGRATION_APPLIED=false
```

## Result

```text
DEPLOY_EXIT_CODE=0
API_CONTAINER_STATUS=Up (cobo-iam-api :8080)
WORKER_CONTAINER_STATUS=Up (cobo-iam-worker) — observe only
DEPLOY_RESULT=PASS
```

Cross-compile: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64` api+worker → SCP → force-recreate api/worker.
