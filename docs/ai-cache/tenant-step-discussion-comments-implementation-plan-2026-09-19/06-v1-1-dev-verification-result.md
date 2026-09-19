# V1.1 DEV verification result — Phase 6 (2026-09-19)

```text
PHASE=V1.1_PHASE_6
TASKS=T6.1,T6.2
IMPLEMENTATION_STATUS=COMPLETE

DEV_TARGET_CONFIRMED=true
DEV_HOST=88.216.208.0
DEV_SSH_PORT=21239
DEV_HOSTNAME=avi-server1
DEV_PATH=/root/cobo_project
DEV_API_BASE_URL=http://88.216.208.0:8080
DEV_FRONTEND_URL=http://88.216.208.0:3000
DEV_DATABASE_TARGET=cobo-iam-mysql / cobo_iam (compose service mysql)
DEV_COMPOSE_FILE=docker-compose.artifacts.yml

MIGRATION_0140=PASS
SCHEMA_VERIFY=PASS
INDEX_VERIFY=PASS
SCHEMA_DRIFT=NONE

BACKEND_BUILD=PASS
WORKER_BUILD=PASS
FRONTEND_BUILD=PASS
DEV_DEPLOY_BUILD=PASS
LOCAL_DOCKER_BUILD=BLOCKED

API_DEPLOY=PASS
WORKER_DEPLOY=PASS
FE_DEPLOY=PASS
HEALTHZ=PASS
READYZ=PASS

FEATURE_FLAG_OFF_SMOKE=PASS
CANDIDATE_SMOKE=PASS
CREATE_MENTION_SMOKE=PASS
PATCH_MENTION_SMOKE=PASS
IDEMPOTENCY_SMOKE=PASS
NOTIFICATION_SMOKE=PASS
V1_REGRESSION=PASS
EVIDENCE_REGRESSION=PASS
HISTORY_REGRESSION=PASS
B2_B3_REGRESSION=PASS

MIGRATION_0140_APPLIED=true
DEV_DEPLOYED=true
DEPLOYED_TO_PRODUCTION=false
STAGED=false
COMMITTED=false
PUSHED=false
MERGED=false

READY_FOR_ROLLOUT=false
READY_FOR_USER_COMMIT=true
```

## Phase 0 — DEV target

| Check | Evidence |
|---|---|
| Hostname | `avi-server1` |
| Path | `/root/cobo_project` + `docker-compose.artifacts.yml` |
| Public URLs | `PUBLIC_API_BASE_URL=http://88.216.208.0:8080`, `PUBLIC_WEB_BASE_URL=http://88.216.208.0:3000` |
| DB | `cobo-iam-mysql` / database `cobo_iam` |
| Not Production | `docs/deploy-dev-guide.md` + `deploy-dev.ps1` defaults |

## Phase 1 — Preflight

- `go test ./internal/workflowstepcomments/...` PASS
- `go test ./internal/inappnotification/...` PASS (no test files; compile OK)
- `go build ./...` PASS
- `npm run build` PASS
- `TestContract_VariableParity/workflow.approved` **FAIL preexisting** — unused `workflow_instance_id` in email template meta; **not** introduced by Phase 5 kind `workflow.step_comment.mentioned` (inapp package, separate from `notification/app`)

## Phase 2–3 — Migration 0140

Applied once via `deploy-artifacts/push-migration.ps1 -File 0140_workflow_step_comment_mentions.up.sql` (official Windows push path).  
`deploy-dev.ps1 -Mode migrate` aborted early on PowerShell treating MySQL stderr warning as terminating; apply still used the same push-migration workflow.

| Check | Result |
|---|---|
| `schema_migrations` | `0140_workflow_step_comment_mentions.up.sql` @ `2026-09-19 09:36:59` |
| Table | `workflow_step_comment_mentions` exists |
| Columns | `id/company_id/comment_id/mentioned_membership_id` VARCHAR(36) NOT NULL; offsets INT; `created_at` DATETIME(3) |
| Indexes | `idx_wscm_company_comment`, `idx_wscm_company_mentioned_created`, `idx_wscm_company_id` (company_id leading) |
| Charset | utf8mb4 / utf8mb4_unicode_ci |
| FK | none |
| Drift vs source | NONE |
| 0138/0139 | untouched |

## Phase 4–5 — Deploy

- Compose updated with `WORKFLOW_STEP_COMMENT_MENTIONS_ENABLED: "true"` (api)
- `deploy-dev.ps1 -Mode be -SkipTests` → api+worker recreate; `/healthz` ok; `/readyz` ready
- `deploy-dev.ps1 -Mode fe -SkipTests` with `VITE_STEP_COMMENT_MENTIONS_V11=true` → asset `index-CS8SZzRE.js`; web healthy
- Final runtime flag after OFF smoke restore: `WORKFLOW_STEP_COMMENT_MENTIONS_ENABLED=true`

```text
GENERATED_ARTIFACTS_EXCLUDED=true
COMMIT_EXCLUSIONS=deploy-artifacts/backend/bin/api, deploy-artifacts/backend/bin/worker, cobo_web_design/dist/
```

## Smoke matrices

Account: `admin.dn@example.com` @ `c_001` / `m_102` (tokens not logged).  
Record: `01a0b575-f97d-7c39-a3b5-7c348751bfde` / step `step-001`.  
Mention peers: UUID memberships (seed `m_*` IDs are not valid mention targets per UUID contract).

### Feature flag OFF

| CASE | EXPECTED | ACTUAL | RESULT |
|---|---|---|---|
| LIST can_mention | false | false | PASS |
| candidates | 404 FEATURE_DISABLED | 404 FEATURE_DISABLED | PASS |
| POST with mentions | 400 FEATURE_DISABLED | 400 FEATURE_DISABLED | PASS |
| POST V1 no mentions | 201 | 201 | PASS |

Flag restored to ON after matrix.

### Feature flag ON

| CASE | RESULT |
|---|---|
| can_mention=true | PASS |
| candidate q&lt;2 → 400 | PASS |
| candidate q&gt;100 → 400 | PASS |
| candidate page_size 50 OK / 51 → 400 | PASS |
| candidate no email/phone | PASS |
| CREATE V1 no mentions + idem replay | PASS |
| CREATE one mention + DTO + UUID36 | PASS |
| mention row persisted | PASS |
| idem replay same comment | PASS |
| idem conflict different target → 409 | PASS |
| invalid membership → 400 | PASS |
| PATCH body same mention / replace / clear | PASS |
| self-mention with seed `m_102` → 400 (not UUID) | PASS (contract); unit tests cover self-skip notify |
| notification fail-closed without scope | PASS (`recipient_cannot_view`) |
| notification success after scoped grant | PASS — kind `workflow.step_comment.mentioned`, generic title/body, resource `disclosure`/`record_id`, **no comment body** |
| Evidence list / steps list / History FE 200 | PASS |
| FE asset mention + V1 empty copy | PASS |

## Limitations / notes

```text
No durable retry queue
No exactly-once guarantee
Notification failure does not rollback comment
Seed membership IDs (m_102/m_103) cannot be mention targets (UUID required)
Full interactive FE keyboard/@ popover smoke deferred to Phase 4 vitest + asset presence (no Playwright session this cycle)
```

Temporary smoke grants (`deadline.view`/`rbac.manage` direct + role on UUID peers) were **removed** after notification success proof.

## Preexisting failures (unchanged)

- `notification/app` `TestContract_VariableParity/workflow.approved`
- `workflowstepevidence` G2C suite
- `platform/config` avatar env override
- `httpserver` CMS/disclosure integration tests
- Local Docker daemon unavailable → `LOCAL_DOCKER_BUILD=BLOCKED`

## Premerge (Phase 6)

PASS — DEV target confirmed; 0140 applied/verified; BE/FE deploy + health; flag OFF/ON; mentions create/patch/idempotency; notification privacy + fail-closed + success path; V1 regression; no Production; no stage/commit.

## Decision

V1.1 Phase 6 DEV verification **COMPLETE**. Safe for **user** commit of product sources (exclude binaries/dist). Agent did not stage/commit. Production rollout remains **out of scope**.
