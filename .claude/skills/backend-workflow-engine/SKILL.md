---
name: backend-workflow-engine
description: Dùng khi sửa hoặc mở rộng workflow phê duyệt CBTT trong cobo_iam_services - các module workflow, workflowconfig, workflowdept, workflowdoctemplate, workflowfulfillment, workflowstepcomments, workflowstepevidence, adhoc; state machine, assignee, step evidence/file, comment @mention, publish vs activate version.
---

# Backend Workflow Engine

## Module map (verify; as of 2026-10)
| Package | Responsibility | Key rules |
|---|---|---|
| `internal/workflow` | Instances and tasks | instance `in_progress → approved/rejected`; task `pending → approved/rejected` with `WHERE status=?` guard |
| `internal/workflowconfig` | Global workflow versions, assignee-role catalog, readiness | publish ≠ activate; versions immutable once published |
| `internal/workflowdept` | Department binding, department email delivery | recipient emails encrypted AES-GCM with env key (`WORKFLOW_DEPARTMENT_EMAIL_RECIPIENT_KEY[_VERSION]`) |
| `internal/workflowdoctemplate` | Template file assets | scope `cms` or `company`, max 20 MB |
| `internal/workflowfulfillment` | Per-step required documents gate | files ACTIVE/SUPERSEDED/DELETED, max 10 files × 20 MB |
| `internal/workflowstepevidence` | Step evidence files | same lifecycle as fulfillment |
| `internal/workflowstepcomments` | Step comments | 4000 chars, 24h edit window, ≤20 @mentions, idempotency scope `workflow_step_comment.create.v1` |
| `internal/adhoc` | Ad-hoc proposals | `ad_hoc_draft → pending_focal_approval → pending_admin_approval → approved/rejected/cancelled` |
| `internal/platform/workflowassign` | Assignee resolution helpers | |
Frontend: `src/pages/portal/*Workflow*`, `src/features/cms-core/templates/workflow`.
Many behaviors are behind `WORKFLOW_*` env flags - check config before testing.

## Workflow
1. Read the relevant spec (`SPEC-workflow-permission.md`,
   `SPEC-alert-workflow.md` in web repo) and ai-cache notes.
2. Write the state transition table (from, event, guard, to, side effects:
   audit, outbox event, notification, reminder).
3. Every transition: status guard in SQL (`UPDATE ... WHERE id=? AND status=?`,
   check rows affected → `STATE_CONFLICT`), permission + assignee check,
   company scope.
4. Side effects through outbox in the same tx (`InsertTx`), not direct email.
5. Versioning: running instances keep the version they started with;
   activating a new version must not mutate in-flight instances.
6. Files: size limit (`MaxBytesReader`), content-type sniffing, storage path
   not user-controlled, soft-delete via lifecycle status, authorization on
   download.
7. Comments/mentions: mention only users in the same company with access to
   the instance; idempotency key on create.
8. Tests: each transition, illegal transition, concurrent approve (two
   approvers), wrong company, wrong assignee, version switch mid-flight.

## Guardrails
- Never allow a transition skip by sending a target status from the client.
- Don't change published workflow versions in place.
- Keep in-memory and MySQL implementations behaviorally identical.

## Output format
- Transition table (changed rows)
- Guards and permissions
- Side effects (outbox/notification/audit)
- Migration/flag impact
- Tests
