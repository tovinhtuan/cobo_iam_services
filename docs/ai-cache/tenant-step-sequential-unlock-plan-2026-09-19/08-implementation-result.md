# 08 — Implementation result — sequential unlock + alert button rename (2026-09-19)

## Status

IMPLEMENTED locally. DEV smoke **not** requested / not run. No stage/commit/push/merge/production deploy.

## Product contract applied

| Flag | Value |
|---|---|
| EARLY_START_FIRST_STEP | false |
| SUCCESSOR_UNLOCK_AFTER_PREDECESSOR_COMPLETION | true |
| ALERT_BUTTON_DECISION | RENAME_TO_DISCLOSURE_EDIT |

## BE changes

- `internal/deadlinealerts/app/steps_algorithm.go`
  - `resolveCurrentStepIndex`: sequential unlock after predecessor completion; first step still calendar-gated before `planned_start`.
  - `IsCurrentByTime`: sequential current ∩ calendar window (early unlock → `false` while `status=current`).
  - Actions still gated by `canManage && isCurrent && !isFuture`.
- `internal/deadlinealerts/app/steps_algorithm_test.go` — matrix for early unlock / first-step lock / incomplete block / no-permission.
- `internal/workflowstepevidence/service_g2b_test.go` — deterministic `baseWF` T0 + ProcessingDays (fixes flaky `WORKFLOW_STEP_NOT_CURRENT` when `WithNow` ≠ `time.Now()`).

`EvaluateStepAuthority` / `ClassifyWorkflowStepStatus` consume `ComputeDeadlineSteps` — no duplicate algorithm.

## FE changes

- `DeadlineDetail.tsx`: label `Cập nhật cảnh báo` → `Chỉnh sửa tin công bố`; navigate `/app/disclosures/:id/edit` unchanged; bottom `Kết thúc bước` (publish/confirm) unchanged.
- `DeadlineWorkflowCard.tsx`: lock copy mentions predecessor + schedule; complete still `available_actions.includes('complete')`.
- Tests: step-actions early unlock, view-model mapping, chrome geometry label/route.

## Verification

| Check | Result |
|---|---|
| `go test ./internal/deadlinealerts/...` | PASS |
| `go test ./internal/workflowfulfillment/...` | PASS |
| `go test ./internal/workflowstepevidence/...` | PASS |
| `go build ./...` | PASS |
| `go test ./...` | FAIL preexisting only: `TestLoad_UserAvatarEnvOverride`, `TestContract_VariableParity/workflow.approved` |
| `npm run build` | PASS |
| Focused vitest (step-actions, deadlineStepsApi, portalChromeGeometry) | PASS |
| `npm run lint` | FAIL preexisting (unrelated TS errors) |
| `npm run test -- --run` | 64 failed / 2288 passed — failures in cms-core templates etc., not feature files |
| `docker compose … build api` | BLOCKED: Docker daemon not running |

## Cross-repo flags

```text
API_CONTRACT_CHANGE=semantic_existing_fields_only
DB_MIGRATION_REQUIRED=false
AUTHZ_CHANGED=false
TENANT_ISOLATION_CHANGED=false
AUDIT_CONTRACT_CHANGED=false
NOTIFICATION_CONTRACT_CHANGED=false
MIGRATION_CREATED=false
DEV_DEPLOYED=false
```

## Remaining

- DEV smoke checklist (T6) when PO requests DEV deploy.
- User commit when ready (`READY_FOR_USER_COMMIT=true`).
