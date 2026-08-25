# 00 — Source / worktree gate

```text
MODE=DEV_VERIFICATION_ONLY
APPLICATION_SOURCE_CHANGED_BY_PHASE_3=false
```

## Local fast gate (pre-deploy)

```bash
go test ./internal/deadlinealerts/... -count=1
go build -o /dev/null ./cmd/api/
```

```text
PHASE_1_REPOSITORY_REGRESSION=PASS
PHASE_2_SERVICE_TESTS=PASS
BACKEND_BUILD=PASS
LOCAL_WORKSTATION_IS_DEV=false
```

## Worktree classification (pre Phase 3 evidence)

| Path | Class |
|------|-------|
| `internal/deadlinealerts/infra/mysql/*` (Phase 1; in HEAD) | PHASE_1_APPLICATION_CHANGES |
| `internal/deadlinealerts/app/service.go` + tests/status (Phase 2 dirty) | PHASE_2_APPLICATION_CHANGES |
| Phase 1/2 ai-cache docs | AI_CACHE |
| `deploy-artifacts/web/dist/*` | GENERATED / PREEXISTING |
| FE `src/**` | unchanged |

```text
UNEXPLAINED_APPLICATION_SOURCE_FILES=0
PREEXISTING_USER_CHANGES_PRESERVED=true
PHASE_1_SQL_MEMBERSHIP_LOCKED=true
PHASE_2_SERVICE_INTEGRATION_LOCKED=true
```
