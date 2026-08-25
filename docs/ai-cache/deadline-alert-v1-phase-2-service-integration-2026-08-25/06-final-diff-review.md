# 06 — Final diff review

## Phase 2 application files

```text
M  internal/deadlinealerts/app/service.go       — remove Draft membership skip
M  internal/deadlinealerts/app/service_test.go  — Draft survives + status matrix
M  internal/deadlinealerts/app/status.go        — remove unused isDraftRecordStatus
M  internal/deadlinealerts/app/status_test.go   — drop draft helper test
A  docs/ai-cache/deadline-alert-v1-phase-2-service-integration-2026-08-25/*
M  docs/ai-cache/README.md
```

## Classification

| Path | Class |
|------|-------|
| app/service*.go, status*.go | PHASE_2_NEW |
| infra/mysql/* | PHASE_1_EXISTING (untouched by Phase 2) |
| phase-1 evidence | PHASE_1_EXISTING |
| phase-2 evidence | AI_CACHE / PHASE_2 |
| deploy-artifacts/* | GENERATED / PREEXISTING |

## Hard gates

```text
REPOSITORY_SQL_CHANGED_BY_PHASE_2=false
WORKER_CHANGED=false
PERIODICITY_CHANGED=false
REMINDER_CHANGED=false
FE_CHANGED=false
MIGRATION_CREATED=false
API_SCHEMA_CHANGED=false
SCOPE_DRIFT=false
SECRET_SCAN=PASS
UNEXPLAINED_SOURCE_FILES=0
```
