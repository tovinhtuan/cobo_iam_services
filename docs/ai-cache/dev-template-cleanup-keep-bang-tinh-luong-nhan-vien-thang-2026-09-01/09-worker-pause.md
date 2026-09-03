# 09 — Worker pause

## Service

```text
PERIODIC_WORKER_SERVICE=worker
CONTAINER=cobo-iam-worker
COMPOSE_FILE=/root/cobo_project/docker-compose.artifacts.yml
```

## Pause

```bash
cd /root/cobo_project
docker compose -f docker-compose.artifacts.yml stop worker
```

## Result

```text
PERIODIC_WORKER_PAUSED=true
PERIODIC_WORKER_INSTANCE_COUNT_BEFORE_DELETE=0
```

## Runtime flags (unchanged)

```text
PERIODIC_SEEDING_ENABLED=true
WORKFLOW_SNAPSHOT_ENABLED=true
DEV_PERIODIC_RUNTIME_CONFIG_CHANGED=false
```

No config file edits. Process pause only.
