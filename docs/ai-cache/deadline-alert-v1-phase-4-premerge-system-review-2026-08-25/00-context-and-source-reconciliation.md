# 00 — Context and source reconciliation

```text
MODE=PREMERGE_SYSTEM_REVIEW_AND_RELEASE_READINESS_ONLY
NEW_CONTEXT_HANDOFF_APPLIED=true
README_APPLIED=true
APPLICATION_SOURCE_CHANGED_BY_PHASE_4=false
NEW_DEV_DEPLOY=false
```

## Repos / branch

```text
BE=cobo_iam_services @ recovery/lost-changes-audit-20260717-153324
HEAD=76060d6 refactor deadline
FE=cobo_web_design (docs pointers; FE src unchanged for V1)
WORKTREE=clean (both repos)
LOCAL_WORKSTATION_IS_DEV=false
DEV_REMOTE=avi-server1 (Phase 3 verified)
```

## Feature commits (already on branch)

| Commit | Scope |
|--------|--------|
| `e2e1c62` | Phase 1 SQL membership + tests (+ mixed deploy-artifacts web dist + docs) |
| `76060d6` | Phase 2 service + Phase 2/3 evidence (+ mixed deploy-artifacts binaries + smoke scripts) |

## Actual symbols (reconfirmed)

```text
ACTUAL_REPOSITORY_PATH=internal/deadlinealerts/infra/mysql/repository.go
ACTUAL_LIST_FUNCTION=ListRows
MEMBERSHIP_SQL=listRowsV1ObligationMembershipSQL (list_rows_membership.go)
ACTUAL_SERVICE_PATH=internal/deadlinealerts/app/service.go
ACTUAL_SERVICE_SYMBOL=ListDeadlineAlerts
GO_DRAFT_MEMBERSHIP_FILTER=absent
```
