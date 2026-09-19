# V1.1 Phase 5 — In-app notification for @Mention (T5.1)

**Cached:** 2026-09-19  
**Status:** `IMPLEMENTED_NOT_DEPLOYED`  
**Repos:** `cobo_iam_services` (BE only; FE unchanged)

## Verdict

```text
V1_1_PHASE_5_STATUS=IMPLEMENTED_NOT_DEPLOYED
NOTIFICATION_IMPLEMENTED=true
NOTIFICATION_MECHANISM=CreateForUser_direct
NOTIFICATION_DELIVERY=AFTER_COMMIT_BEST_EFFORT
NOTIFICATION_KIND=workflow.step_comment.mentioned
EMAIL_NOTIFICATION=false
EXTERNAL_INTEGRATION=false
MIGRATION_0140_APPLIED=false
DEV_DEPLOYED=false
FE_CHANGED=false
READY_FOR_PHASE_6=true
READY_FOR_DEV_DEPLOY=false
```

## Implemented

- After successful comment+mentions **COMMIT**, call `inappnotification.CreateForUser` per newly mentioned membership.
- CREATE: all canonical mentions (deduped by membership/user); skip self; skip unresolvable; fail-closed if recipient lacks `deadline.view` / data scope / step load.
- PATCH: notify `new_set - old_set` only; body-only edit with same mentions → no notify; removals → no notify.
- Idempotent CREATE replay → **no** `CreateForUser`.
- Notification failure logged; HTTP success; comment/mentions remain committed.
- Generic title/body only — **no comment body** in notification.
- Kind constant: `workflow.step_comment.mentioned`.

## Limitations (locked)

```text
No durable retry queue
No exactly-once guarantee
Notification failure does not rollback comment
Cross-process duplicate possible if clients bypass Idempotency-Key
```

## Files

- `internal/workflowstepcomments/mentions_notify.go`
- `internal/workflowstepcomments/mentions_notify_test.go`
- `internal/workflowstepcomments/mentions_membership.go` (MembershipUserResolver)
- `internal/workflowstepcomments/service.go` (after-commit hooks)
- `internal/inappnotification/app/contracts.go` (kind const)
- `internal/httpserver/server.go` (wire notifier + user resolver)

## Verification

- `go test ./internal/workflowstepcomments/...` PASS
- `go build ./...` PASS
- `docker compose … build api` **BLOCKED:** Docker daemon not running
- MIGRATION_0140_APPLIED=false; STAGED=false; no FE change

## Premerge (Phase 5)

PASS — after-commit boundary; no body leak; recipient authz fail-closed; self-skip; edit delta; replay no duplicate; failure non-rollback; no email/outbox/migration/deploy.
