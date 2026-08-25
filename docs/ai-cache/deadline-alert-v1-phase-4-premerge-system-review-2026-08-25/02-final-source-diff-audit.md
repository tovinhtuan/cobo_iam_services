# 02 — Final source diff audit

## Application / test (Deadline Alert V1)

| File | Role |
|------|------|
| `internal/deadlinealerts/infra/mysql/list_rows_membership.go` | V1 membership SQL + HCM helper + mirror evaluator |
| `internal/deadlinealerts/infra/mysql/list_rows_membership_test.go` | Deterministic membership matrix + SQL shape |
| `internal/deadlinealerts/infra/mysql/repository.go` | ListRows wires membership + todayHCM bind |
| `internal/deadlinealerts/app/service.go` | Remove Go Draft membership skip |
| `internal/deadlinealerts/app/service_test.go` | Draft survives + status/confirm tests |
| `internal/deadlinealerts/app/status.go` | Remove unused `isDraftRecordStatus` |
| `internal/deadlinealerts/app/status_test.go` | Drop draft-helper unit test |

## Intentionally unchanged helpers

```text
listTaskAssigneeRecords / listCurrentStepMeta still use status <> draft
→ enrichment only; NOT membership authority (P2 note)
ORDER BY dr.created_at DESC preserved (sort, not AlertFrom)
```

## Mixed into historical commits (must NOT be in clean release candidate)

```text
deploy-artifacts/web/dist/**
deploy-artifacts/backend/bin/api|worker
Phase 3 run-*.py/mjs with default QA password
screenshots (optional evidence — policy)
FE docs/ai-cache/_tmp_dav1_browser.mjs (leftover)
```

```text
UNEXPLAINED_APPLICATION_SOURCE_FILES=0
```
