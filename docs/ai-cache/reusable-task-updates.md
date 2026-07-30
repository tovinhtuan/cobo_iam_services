## 2026-07-30 - Guarded periodic one-shot materialization (DEV)

- task type: BE implement + DEV CLI apply + paired FE E2E
- package: `internal/disclosure/app/periodic_oneshot` + `cmd/periodic-materialize-one`
- scope: type `qa-monthly-deadline-alert-202607-1785382733` / company `c_001` / period `2026-07`
- verified: MATERIALIZED 1 cycle+1 record due 2026-07-31; API UPCOMING; UI Sắp tới; idempotent NO_OP; PERIODIC_SEEDING_ENABLED=false; MySQL ID unchanged; direct SQL=0
- evidence: `docs/ai-cache/guarded-periodic-one-shot-materialization-2026-07-30/` (mirrored FE)
- verdict: **PASS_ONE_SHOT_MATERIALIZATION_AND_ALERT_E2E**

## 2026-07-30 - Current-month periodic deadline alert DEV E2E

- paired FE: `../cobo_web_design/docs/ai-cache/current-month-periodic-deadline-alert-e2e-2026-07-30/`
- QA `qa-monthly-deadline-alert-202607-1785382733` active PERIODIC monthly N=23; no materialize (PERIODIC_SEEDING_ENABLED=false)
- verdict: **BLOCKED_PERIODIC_MATERIALIZATION**; no seeding flip / DB write / migration / production

## 2026-07-30 - Q2 Financial Report Deadline Alert DEV E2E

- paired FE evidence: `../cobo_web_design/docs/ai-cache/q2-financial-report-deadline-alert-e2e-2026-07-30/`
- blocker: PERIOD_END + 30 CALENDAR_DAYS unsupported; calculator/CMS lack PERIOD_END; PERIODIC_SEEDING_ENABLED=false
- verdict: **BLOCKED_DEADLINE_CONTRACT**; no code/DB/migration/backfill/production

## 2026-07-30 - Deadline alert active-report audit (DEV)

- task type: audit-only (cross-repo); paired FE evidence `../cobo_web_design/docs/ai-cache/deadline-alert-active-report-audit-2026-07-30/`
- discovered: alerts from `disclosure_records` (not template-active alone); `bao-cao-tai-chinh-quy-2` Q2/2026 Completed → `PENDING_CONFIRM` due 2026-05-12; Overdue filter empty by design
- flags: `PERIODIC_SEEDING_ENABLED=false` (existing cycles already present)
- verdict: **EXPECTED_BEHAVIOR_CONFIRMED** RC-1; no code/DB/migration/deploy

## 2026-07-30 - Workflow Configuration 404 remediation (DEV)

- paired FE evidence: `../cobo_web_design/docs/ai-cache/workflow-config-load-404-remediation-2026-07-30/`
- DEV: `WORKFLOW_VERSIONING_ENABLED=true` via `docker-compose.artifacts.yml`; API recreate `--no-deps`; configuration 200 empty; no materialize; isolation PASS
- verdict: **PASS_DEV_REMEDIATION_WITH_NON_BLOCKING_GAPS**

## 2026-07-30 - Workflow Configuration load 404 audit (DEV)

- task type: audit-only (cross-repo); paired evidence in FE `docs/ai-cache/workflow-config-load-404-audit-2026-07-30/`
- discovered: `WORKFLOW_VERSIONING_ENABLED` unset on DEV → `wfchttp.Register` skipped → `GET .../workflow/configuration` → Go `404 page not found`; ConfigService would 200 empty if registered; effective-workflow separate source
- verdict: **ROOT_CAUSE_CONFIRMED** RC-4; awaiting user confirm before env enable / FE gate
- scope: no code/DB/migration/deploy

## 2026-07-20 - Portal profile/audit UX + back navigation
- task type: cross-repo UX (audit labels + personalops activity_log copy)
- implemented: timeline knownActions (`login_success`, `select_company`, `cms_workflow.*`); FriendlyDescription; Normalize no UUID display; activity_log optional action/resource fields
- paired FE: ../cobo_web_design/docs/ai-cache/portal-profile-audit-ux-back-navigation-2026-07-17/
- verify: go timeline/personalops PASS; docker api build PASS; DEV deploy all PASS; security 401/401

## 2026-07-15 - CMS Workflow step reorder + description/instructions
- BE: mig 0121 global_workflow_steps.description; wired in cms_repository/cms_service
- FE evidence: ../cobo_web_design/docs/ai-cache/cms-workflow-step-reorder-description-instructions-2026-07-10/
- deploy: push-migration 0121 applied; docker compose build api PASS

﻿## 2026-07-15 - CMS Alert Email default ON
- BE alert_config_service force enabled; Get missing defaults ON; paired FE hide UI

## 2026-07-15 - CMS file types free-text validation
- BE: remove PDF/DOCS/XML whitelist; free-text file_types; tests PASS; deploy DEV api
- paired FE: cobo_web_design/docs/ai-cache/cms-file-types-text-to-portal-tags-2026-07-10/

## 2026-07-14 - Deadline rule display refine for Portal
- task type: BE contract/display honesty
- implemented: RefineDeadlineRuleDisplay + time_calculation_basis on DisclosureTypeDTO
- verify: go test disclosure/app Deadline/Refine PASS; docker compose build api PASS; deploy be
- sibling FE evidence: cobo_web_design/docs/ai-cache/deadline-rule-cms-to-portal-flow-audit-fix-2026-07-10/ï»¿## 2026-07-13 - Personal Ops outcome/on_time Option B
- migration 0120 completed_at forward-only; ConfirmRecord stamp; personalops ComputeOnTimeRate
- DEV: sample_size=0 unavailable; docker build api PASS; deploy migrate+be
- evidence (FE): personal-ops-outcome-ontime-due-parity-rebaseline-2026-07-10/

## 2026-07-13 - Personal Ops finalization due/ad-hoc/index
- due ResolveDeadline + ListApprovedAdHocDues; migration 0119
- on_time remains unavailable
- evidence (FE): personal-ops-finalization-mock1-e2e-2026-07-10/

ï»¿# Reusable Task Updates

## 2026-07-13 - Personal Ops GET /api/v1/me/operational-overview
- task type: implement BE aggregate API
- objective: mine-scoped personal operational overview for Portal /app/profile
- implemented: `internal/personalops` (domain/app/mysql/http); mine = open workflow_tasks.assignee_membership_id OR active assignments; wired in server.go
- never expand via rbac.manage / CanViewAll
- verification: `go test ./internal/personalops/...` PASS; `docker compose -f docker-compose.dev.yml build api` PASS; deploy-dev be PASS
- evidence (FE): `docs/ai-cache/personal-info-operational-overview-api-wire-2026-07-10/`

## 2026-07-12 - Subscription upgrade Phase 1 QR instruction (IAM)
- task type: implement platform CMS payment-instruction config
- objective: global QR/hotline config + portal instruction API; no entitlement automation
- implemented: migration 0118; `subscription_upgrade_payment` service/repo/handlers; audit actions; admin instruction routes
- verification: `go test` SubscriptionUpgrade PASS; linux deploy DEV; migration applied; healthz/readyz OK
- `BLOCKED:` local docker compose build api (daemon down)
- evidence (FE repo): docs/ai-cache/subscription-upgrade-phase1-qr-instruction-2026-07-10/
- soak: Phase 7D superseded

## 2026-07-11 - Deadline steps department_name enrichment
- task type: bugfix
- objective: steps API returns display department names (not raw `d1`)
- implemented: `enrichStepDepartmentNames` in `ListDeadlineSteps` via `DepartmentDict`
- tests: `go test ./internal/deadlinealerts/...` PASS
- deploy DEV: `deploy-dev.ps1 -Mode be`; apiStartedAt 2026-07-11T13:44:38Z
- evidence (FE repo): docs/ai-cache/deadline-detail-progress-marker-department-name-fix-2026-07-10/
- soak: Phase 7D superseded

## 2026-07-11 - Deadlines department Unicode mojibake fix
- task type: data cleanup + BE defensive dict
- root cause: workflow_template_departments mojibake from 0114 charset
- implemented: migration 0117; DepartmentDict company-first / byCode prefer
- deploy DEV: migrate + be; apiStartedAt 2026-07-11T09:44:50Z
- evidence (FE repo): docs/ai-cache/deadlines-department-name-unicode-bugfix-2026-07-10/
- verdict: PASS
## 2026-05-20 - Fix local Docker Redis port conflict for Cobo stack

- task type: implement
- objective: eliminate recurring local `6379` host-port conflict that blocked `cobo-iam-redis` during `docker compose up`
- what was implemented:
  - removed host port publishing from the `redis` service in `docker-compose.dev.yml`
  - kept Redis accessible internally via compose service name `redis:6379`, which is sufficient for `api` and `worker`
- affected repos/files/modules:
  - `docker-compose.dev.yml`
- contracts/behaviors/constraints/decisions:
  - this is a local/dev compose change only
  - external host access to Cobo Redis via `localhost:6379` is no longer provided by this compose file
  - no application runtime contract between compose services changed; internal dependency remains `redis:6379`
- build/verification result:
  - `docker compose -f docker-compose.dev.yml build api` => exit 0
  - `docker compose -f docker-compose.dev.yml up -d --force-recreate` => exit 0
  - resulting stack status:
    - `cobo-iam-api` up on `8080`
    - `cobo-web-design` up on `3000`
    - `cobo-iam-mysql` healthy
    - `cobo-iam-redis` healthy (internal only, `6379/tcp`)
    - `cobo-iam-worker` up
    - `cobo-iam-mailpit` healthy
- remaining gaps/risks/next steps:
  - if a developer explicitly needs host-side Redis inspection, use `docker exec` into `cobo-iam-redis` or temporarily add a non-conflicting host port mapping

## 2026-05-20 - Plan deadline alerts tab adjustment (cross-repo analysis)

- task type: understand / plan
- objective: recheck current UI/contract boundaries and prepare ticket-ready implementation plan for adjusting the `CÃ¡ÂºÂ£nh bÃƒÂ¡o thÃ¡Â»Âi hÃ¡ÂºÂ¡n` tab without affecting other flows
- discovered:
  - FE route `/app/deadlines` renders `DeadlineList` and contains two internal tabs: `CÃ¡ÂºÂ£nh bÃƒÂ¡o thÃ¡Â»Âi hÃ¡ÂºÂ¡n` and `LÃ¡Â»â€¹ch sÃ¡Â»Â­ CBTT`.
  - current alert cards are mock-driven from `mockDeadlines` and still render `owner` plus CTA buttons `Chi tiÃ¡ÂºÂ¿t` and `TÃ¡ÂºÂ¡o cÃ¡ÂºÂ£nh bÃƒÂ¡o mÃ¡Â»â€ºi`.
  - current FE `DeadlineAlert` type does not contain workflow-derived department state; therefore requirement 4.2 cannot be implemented correctly by simple label changes.
  - BE workflow contract already exposes `workflow instance` data with `snapshot`, `current_step_code`, `t0_date`, `t0_policy`; snapshot rows include `department`, `step_code`, and `processing_days`.
  - current BE workflow task DTO is additive-safe but not yet shaped specifically for the deadline list card view; deriving active department from workflow instance is the safest boundary.
- affected repos/files/modules:
  - `cobo_web_design/src/pages/portal/DeadlineList.tsx`
  - `cobo_web_design/src/types.ts`
  - `cobo_web_design/src/mockData.ts`
  - `cobo_web_design/src/services/*` (new adapter likely needed)
  - `cobo_iam_services/internal/workflow/app/contracts.go`
  - `cobo_iam_services/internal/workflow/transport/http/handler.go`
- contracts/behaviors/constraints/decisions:
  - phase 1 should stay FE-only and isolate changes to the deadline alert card rendering path.
  - phase 2 should be additive: FE consumes workflow-backed department data without changing existing workflow action flows or history behavior.
  - source of truth for `phÃƒÂ²ng Ã„â€˜ang thÃ¡Â»Â±c hiÃ¡Â»â€¡n` should come from workflow state / snapshot mapping, not from the current `owner` field.
- build/verification result:
  - planning-only task; no runtime code changes; docker/build verification not required
- remaining gaps/risks/next steps:
  - clarify whether multi-department display can be derived from one active step or requires backend support for multiple simultaneous active steps
  - next step: produce ticket-ready plan with phase split, acceptance criteria, and test scope

## 2026-05-20 - Review ticket-ready plan for deadline alerts tab

- task type: review / understand
- objective: review the proposed 2-phase implementation plan and identify high-conflict assumptions before implementation
- findings:
  - current FE alert list has no stable link from `DeadlineAlert` to `workflow_instance_id`, so phase 2 cannot reliably fetch workflow-backed active departments until the source-of-truth join is defined.
  - current BE workflow contract exposes only singular `current_step_code`; this does not by itself satisfy the requirement to show multiple departments at the same time if more than one step can be active concurrently.
  - current requirement wording says active department is based on `T0 + ngÃƒÂ y`, while the plan recommends backend `current_step_code` as source of truth; this is a deliberate architectural choice but needs explicit business/technical confirmation to avoid contract mismatch.
  - the current page also has a top header CTA `TÃ¡ÂºÂ¡o cÃ¡ÂºÂ£nh bÃƒÂ¡o mÃ¡Â»â€ºi` for the `Deadlines` tab, not only per-item CTAs; requirement wording only mentions removing the button in each item, so header CTA scope must be confirmed before FE work starts.
- affected repos/files/modules:
  - `cobo_web_design/src/pages/portal/DeadlineList.tsx`
  - `cobo_web_design/src/types.ts`
  - `cobo_web_design/src/mockData.ts`
  - `cobo_iam_services/internal/workflow/app/contracts.go`
- contracts/behaviors/constraints/decisions:
  - phase 2 should not start until the alert -> workflow instance join and multi-department semantics are explicit.
  - phase 1 remains safe because it can isolate UI-only changes and local adapter boundaries.
- build/verification result:
  - review-only task; no runtime code changes; docker/build verification not required
- remaining gaps/risks/next steps:
  - confirm whether header CTA stays or goes
  - confirm whether one active step or multiple active steps are valid in the domain
  - confirm where `workflow_instance_id` for deadline cards comes from in the eventual live data flow

## 2026-05-20 - Debug local Docker stack ports for Cobo IAM services

- task type: understand
- objective: determine why local Cobo Docker containers were all exited and why they do not come back up cleanly
- discovered:
  - all Cobo containers (`cobo-iam-*`, `cobo-web-design`) share the same `FinishedAt` timestamp around `2026-05-19T01:23:28Z`, which indicates an external stop/daemon restart pattern rather than independent app crashes.
  - historical container logs show MySQL, Redis, Mailpit, API, worker, and web all started successfully before that stop event.
  - re-running `docker compose -f docker-compose.dev.yml up -d` restarts infra containers successfully, but `cobo-iam-api` fails to bind host port `8080` because another container `go-api` is already using `0.0.0.0:8080`.
  - manually starting `cobo-web-design` fails to bind host port `3000` because another container `nextjs-app` is already using `0.0.0.0:3000`.
  - current partial stack state after diagnosis: MySQL, Redis, Mailpit, worker are up; API and web remain blocked by host port conflicts.
- affected repos/files/modules:
  - `docker-compose.dev.yml`
  - local Docker host container set (`go-api`, `nextjs-app`, `cobo-iam-*`, `cobo-web-design`)
- contracts/behaviors/constraints/decisions:
  - this is an environment/runtime issue, not evidence of an application startup bug in current logs.
  - the compose file expects exclusive use of host ports `8080` and `3000` for `api` and `web`.
- build/verification result:
  - `docker compose -f docker-compose.dev.yml up -d` => failed at `cobo-iam-api` with `Bind for 0.0.0.0:8080 failed: port is already allocated`
  - `docker start cobo-web-design` => failed with `Bind for 0.0.0.0:3000 failed: port is already allocated`
- remaining gaps/risks/next steps:
  - either stop/remove the conflicting containers (`go-api`, `nextjs-app`) before starting Cobo, or remap Cobo host ports in `docker-compose.dev.yml`

## 2026-05-20 - Verify Codex global skill loading vs local `agent-skills` repo

- task type: understand
- objective: verify whether skills under `/home/icom/go/src/myself/backend_api_cobo/agent-skills` are missing from Codex global skills
- discovered:
  - Codex user-level skills are loaded from `~/.codex/skills`, not directly from arbitrary local repos.
  - The 23 lifecycle/meta skills under `/home/icom/go/src/myself/backend_api_cobo/agent-skills/skills` match the 23 user-level skill directories under `~/.codex/skills` exactly at `SKILL.md` level.
  - The only extra directory under `~/.codex/skills` is `.system`, which is expected for preinstalled system skills.
  - Therefore there is no current gap where those repo skills are absent from Codex global scope; the remaining difference is that the local repo is not the live source of truth for future auto-sync unless linked/copied again.
- affected repos/files/modules:
  - `~/.codex/skills/*`
  - `/home/icom/go/src/myself/backend_api_cobo/agent-skills/skills/*`
  - `docs/ai-cache/reusable-task-updates.md`
- contracts/behaviors/constraints/decisions:
  - Codex does not auto-discover skills from `/home/icom/go/src/myself/backend_api_cobo/agent-skills`; it discovers skills from `~/.codex/skills`.
  - New or changed skills require syncing into `~/.codex/skills` and typically a Codex restart to be picked up by fresh sessions.
- build/verification result:
  - analysis-only task; no runtime code changes; docker/build verification not required
- remaining gaps/risks/next steps:
  - if the goal is live reuse from the local repo, convert `~/.codex/skills/<name>` entries to symlinks pointing at `/home/icom/go/src/myself/backend_api_cobo/agent-skills/skills/<name>` or introduce a repeatable sync script

## 2026-05-06 - Tenant workflow override skeleton (migration + IAM contracts/service/repo + API docs)

- task type: implement
- objective: implement first sprint skeleton for enterprise-scoped workflow override on template; include DB migration 0020, backend skeleton layers, and API contract docs for team handoff
- what was implemented:
  - added migration:
    - `migrations/0020_company_template_workflow_overrides.up.sql`
    - `migrations/0020_company_template_workflow_overrides.down.sql`
    - updated `migrations/run_dev_migrations.sh` to include `0020...up.sql`
  - extended disclosure module contracts in `internal/disclosure/app/contracts.go`:
    - DTOs for workflow step/document and company override header/version/view
    - new service/repository methods for:
      - get override view
      - upsert draft
      - approve
      - delete draft version
      - reset active override
      - list versions
      - get effective workflow
  - implemented service skeleton + validation/authz gates in `internal/disclosure/app/service.go`
  - wired in-memory repo behavior for new methods in `internal/disclosure/infra/inmemory/repository.go`
  - wired MySQL repo skeleton SQL behavior for new methods in `internal/disclosure/infra/mysql/repository.go`
  - exposed new HTTP endpoints in `internal/disclosure/transport/http/handler.go`:
    - `GET /api/v1/company/disclosure-types/{type_id}/workflow-override`
    - `PUT /api/v1/company/disclosure-types/{type_id}/workflow-override/draft`
    - `POST /api/v1/company/disclosure-types/{type_id}/workflow-override/approve`
    - `DELETE /api/v1/company/disclosure-types/{type_id}/workflow-override/draft/{version_no}`
    - `DELETE /api/v1/company/disclosure-types/{type_id}/workflow-override/active`
    - `GET /api/v1/company/disclosure-types/{type_id}/workflow-override/versions`
    - `GET /api/v1/disclosure-types/{type_id}/effective-workflow`
  - updated `docs/api-contracts-json.md` with detailed request/response examples + error matrix section for tenant workflow override
- affected repos/files/modules:
  - `cobo_iam_services/migrations/*0020*`
  - `cobo_iam_services/internal/disclosure/app/contracts.go`
  - `cobo_iam_services/internal/disclosure/app/service.go`
  - `cobo_iam_services/internal/disclosure/infra/inmemory/repository.go`
  - `cobo_iam_services/internal/disclosure/infra/mysql/repository.go`
  - `cobo_iam_services/internal/disclosure/transport/http/handler.go`
  - `cobo_iam_services/docs/api-contracts-json.md`
- contracts/behaviors/constraints/decisions:
  - tenant override data is keyed by `(company_id, type_id)` and kept separate from global template catalog rows
  - effective source resolution returns `company_override` when active approved version exists; otherwise `global_template`
  - service enforces `type_id`, `version_no`, and minimum workflow validation before repository call
- build/verification result:
  - `go test ./internal/disclosure/...` => exit 0
  - `go test ./internal/httpserver -count=1` => exit 0
  - `docker compose -f docker-compose.dev.yml build api` => exit 0
- remaining gaps/risks/next steps:
  - permission names currently reuse disclosure action gates; should align to dedicated `template.workflow.override.*` permissions in next slice
  - MySQL skeleton stores workflow JSON but still needs stricter schema/rule validation parity with FE editor
  - add integration tests for tenant isolation + approve/reset state transitions + conflict paths

## 2026-05-06 - Implement-ready spec: tenant workflow override

- task type: design / understand
- objective: convert tenant workflow customization idea into sprint-ready package: DB schema + detailed API request/response + error matrix
- discovered / designed:
  - proposed new aggregate `company_template_workflow_overrides` + `company_template_workflow_override_versions` for tenant-scoped workflow without mutating global template rows.
  - defined effective workflow resolution order: active approved company override -> global template workflow.
  - finalized endpoint set for CRUD draft + approve + reset + versions/history under company-scoped API paths.
  - standardized error matrix with stable `error.code` + `details.field_errors` for FE mapping.
- affected repos/files/modules (design scope):
  - `cobo_iam_services`: migrations + disclosure handler/service/repository contracts + authorization checks + audit log append
  - `cobo_web_design`: template workflow customization UI + API client + route states + permission mapping
- contracts/behaviors/constraints/decisions:
  - admin enterprise actions are hard-scoped by token `company_id`; request payload must not carry mutable tenant selector.
  - approve is tenant-local and side-effect free for other tenants and global template.
  - optional maker-checker policy can be enabled as strict mode.
- build/verification result:
  - design-only task; no runtime code changes; docker/build verification not required
- remaining gaps/risks/next steps:
  - align permission naming with IAM policy table rollout plan.
  - decide whether draft overwrite uses optimistic lock (`base_version_no`) or server auto-rebase.
  - implement migration + API in phased rollout with regression tests for tenant isolation.

## 2026-05-06 - Design tenant-level workflow customization for template

- task type: design / understand
- objective: design logic so enterprise admin can CRUD customized workflow for template, approve for own company only, without affecting global template or other companies
- discovered / designed:
  - keep global template immutable as shared source (`company_id IS NULL`) and introduce company-scoped workflow override as separate aggregate keyed by `(company_id, type_id)`.
  - add draft -> approved state machine for company override, with explicit versioning and audit trail to avoid hidden overwrite.
  - runtime read contract resolves effective workflow by priority:
    1) approved company override
    2) global template workflow
  - enforce tenant isolation by token-derived `company_id` in every query/update path; reject cross-company access at service layer.
- affected repos/files/modules (design scope):
  - `cobo_iam_services`: disclosure app contracts/service/repository + http handlers + mysql migrations
  - `cobo_web_design`: `src/features/cms-core/pages.tsx` + `services/cmsApi.ts` + template/workflow UI state mapping
- contracts/behaviors/constraints/decisions:
  - enterprise admin CRUD only edits company override draft, never writes to global template rows.
  - approve action only activates override for current `company_id`; no side effect on other tenants.
  - optional reset endpoint removes active override to fallback to global workflow.
- build/verification result:
  - design-only task; no runtime code changes; docker/build verification not required
- remaining gaps/risks/next steps:
  - align permission matrix for create/edit/approve/reset override.
  - decide if 4-eyes approval is required (`maker != checker`) for regulated tenants.
  - implement migration + API + FE route states in small phases.

## 2026-05-06 - Understand CRUD template feature (cross-repo)

- task type: understand
- objective: understand current CRUD template flow across `cobo_web_design` and `cobo_iam_services`
- discovered:
  - FE template CRUD UI lives in `src/features/cms-core/pages.tsx` (`CmsTemplatesPage`) and calls `createCmsApi` methods in `src/features/cms-core/services/cmsApi.ts`.
  - Current FE flow supports:
    - read reference data: `GET /api/v1/admin/disclosure-types/reference-data`
    - read groups: `GET /api/v1/disclosure-groups`
    - read active template detail: `GET /api/v1/disclosure-types/{type_id}`
    - upsert template version (create/update): `PUT /api/v1/admin/disclosure-types/{type_id}`
    - list versions: `GET /api/v1/admin/disclosure-types/{type_id}/versions`
    - activate version: `POST /api/v1/admin/disclosure-types/{type_id}/activate`
    - read specific version: `GET /api/v1/admin/disclosure-types/{type_id}/versions/{version_no}`
  - FE does not expose delete/archive template action; behavior is versioned upsert + activate.
  - FE normalizes and validates block schema locally (`templateBlockSchema.ts`) and enforces six mandatory `block_key` values when `blocks` is non-empty.
  - BE handler routes for template flow are in `internal/disclosure/transport/http/handler.go`; service permission gate for template mutations is `rbac.manage`.
  - BE persistence model is versioned (`disclosure_type_versions` + `disclosure_types.active_version_no`) with transactional upsert/activate in `internal/disclosure/infra/mysql/repository.go`.
  - BE validation in `internal/disclosure/app/service.go` + template schema validators enforces matrix compatibility (`template_category` vs `periodicity` vs `deadline_strategy`) and mandatory template blocks.
  - Catalog read endpoints (`GET /api/v1/disclosure-types*`, `GET /api/v1/disclosure-groups`) are scoped by `company_id` plus global (`company_id IS NULL`) rows.
- affected repos/files/modules:
  - `cobo_web_design`: `src/features/cms-core/pages.tsx`, `src/features/cms-core/services/cmsApi.ts`, `src/features/cms-core/templateBlockSchema.ts`
  - `cobo_iam_services`: `internal/disclosure/transport/http/handler.go`, `internal/disclosure/app/contracts.go`, `internal/disclosure/app/service.go`, `internal/disclosure/infra/mysql/repository.go`, `docs/api-contracts-json.md`
- contracts/behaviors/constraints/decisions:
  - "CRUD template" in current implementation = Read + Upsert (versioned create/update) + Activate version; no hard delete endpoint.
  - Permission-denied and validation errors are expected to return stable API errors; FE maps `error.details.field_errors` into UI message text.
  - Mandatory template blocks are canonicalized to lowercase keys and must include:
    `legal_basis`, `disclosure_content`, `deadline`, `channels_and_format`, `legal_risks`, `enterprise_workflow`.
- build/verification result:
  - analysis-only task; no runtime code changes; docker/build verification not required
- remaining gaps/risks/next steps:
  - If true Delete/Archive is required by product semantics, add explicit backend endpoints and FE actions (with audit + compatibility policy).
  - Tighten FE handling for `loadVersions` error to clear stale version list and prevent mismatch with current `typeId`.

## 2026-04-27 - Mandatory Prompt Policy (2 repos)

- task type: understand
- objective: enforce a mandatory reusable prompt policy for all tasks touching `cobo_web_design` and `cobo_iam_services`
- discovered/implemented:
  - added a mandatory policy block to `docs/ai-cache/README.md` to require:
    - read `docs/ai-cache/README.md` + reusable cache first
    - conflict priority order
    - skill selection and mandatory `integration-cross-repo` for cross-repo tasks
    - contract-first for new features
    - premerge review + fresh Docker rebuild after code changes
    - reusable update writeback into `docs/ai-cache/` after each task
- affected repos/files/modules:
  - `cobo_web_design/docs/ai-cache/README.md`
  - `cobo_iam_services/docs/ai-cache/README.md`
- important contracts/behaviors/constraints/decisions:
  - this policy is process-level guidance; it does not change runtime product behavior
  - do not overwrite source-of-truth content unless task explicitly requires
- build/verification result:
  - no runtime code changed; docker build not required for this documentation-only update
- remaining gaps/risks/next steps:
  - ensure future prompts consistently include/adhere to this mandatory block

## 2026-04-27 - Full CMS End-to-End Screen Plan

- task type: understand
- objective: define a full CMS screen plan so admin web can crawl an end-to-end, production-like CMS flow
- discovered/implemented:
  - current state remains CMS stub-first (`/cms` shell) with login redirect and platform access guard in place
  - planning output defines complete IA, screen groups, API contract-first rollout, state mapping, and phased validation gates
- affected repos/files/modules:
  - `cobo_web_design`: `/cms` route tree, CMS shell/layout/sidebar, feature slices per module, route-level states/tests
  - `cobo_iam_services`: platform CMS API surface, permission/authz matrix, audit/session endpoints, integration contract tests
- important contracts/behaviors/constraints/decisions:
  - CMS remains independent from selected company UI context, but API scope must be explicit per endpoint
  - contract-first required before implementation: request/response/error matrix + FE state mapping + BE expectations + failure modes
  - rollout should be phased with backward-safe endpoints and explicit error model for FE UX states
- build/verification result:
  - planning-only task; no runtime code changes; docker rebuild not required
- remaining gaps/risks/next steps:
  - next: produce per-screen API matrix and acceptance checklist, then implement by phase (shell -> content -> governance -> audit/ops)

## 2026-04-27 - CMS Screen-by-Screen Contract Matrix

- task type: understand
- objective: export route-level CMS contract matrix (request/response/error + permission + FE state + test cases) for direct implementation
- discovered/implemented:
  - defined route-by-route CMS matrix covering dashboard, content, publishing, governance, operations, and settings modules
  - standardized error envelope and FE state mapping per route for code-ready execution
- affected repos/files/modules:
  - `cobo_web_design`: `/cms/*` route guards, screen states, hooks/services contract adapters, route smoke tests
  - `cobo_iam_services`: `/api/v1/platform/cms/*` handler/service/repository contracts, authz checks, integration matrix tests
- important contracts/behaviors/constraints/decisions:
  - CMS routes use platform permission checks independent from company selection UI flow
  - each route must declare list/detail/mutation endpoints, authz gates, and retry/fallback behavior
  - permission-denied state must be distinguishable from generic errors in FE
- build/verification result:
  - planning-only task; no runtime code changes; docker rebuild not required
- remaining gaps/risks/next steps:
  - implement Phase A route shell + contract stubs first, then execute matrix-driven delivery per module

## 2026-04-27 - CMS Contract Matrix CSV Export

- task type: understand
- objective: export the CMS screen-by-screen matrix into CSV import format (Route | Endpoint | Permission | FE States | Test Cases | Owner)
- discovered/implemented:
  - exported 20 CMS routes as one-line-per-route CSV rows with endpoint scope, permission gate, FE states, and route-level test cases
  - kept field values implementation-ready for FE/BE task assignment
- affected repos/files/modules:
  - planning artifacts for `cobo_web_design` `/cms/*` routes and `cobo_iam_services` `/api/v1/platform/cms/*` contracts
- important contracts/behaviors/constraints/decisions:
  - each route row preserves explicit permission + forbidden-path behavior
  - FE states standardized to loading/success/empty/error/forbidden
- build/verification result:
  - planning-only output; no code changes; docker rebuild not required
- remaining gaps/risks/next steps:
  - optionally split CSV into separate FE and BE import sheets with assignee/date fields

## 2026-04-27 - Phase A CMS Shell Implementation Sync

- task type: implement
- objective: sync cross-repo progress after implementing Phase A CMS shell in frontend and validating compose integration
- what was implemented/discovered:
  - no backend code changes in this step
  - compose-integrated `web` rebuild and restart succeeded on latest frontend code
- affected repos/files/modules:
  - `cobo_web_design`: `/cms/*` route shell implementation (`features/cms-core`, `src/App.tsx`, CMS landing tests)
  - `cobo_iam_services`: compose environment validation only (`docker-compose.dev.yml` runtime verification)
- important contracts/behaviors/constraints/decisions:
  - Phase A remains contract-scaffold first; platform CMS API implementation is deferred to next backend phase
  - platform guard continuity maintained through existing permission model
- build/verification result:
  - fresh docker rebuild succeeded for `web`; compose services remained healthy (`api`, `web`, `mysql`, `redis`)
- remaining gaps/risks/next steps:
  - implement `/api/v1/platform/cms/*` handlers and authz matrix to replace scaffold-only screens

## 2026-04-27 - Phase B1 CMS Live Wiring Sync

- task type: implement
- objective: sync cross-repo B1 after wiring live data for first CMS routes in frontend
- what was implemented/discovered:
  - no backend code changed in this step
  - frontend now consumes existing live IAM APIs (`/api/v1/me/capabilities`, `/api/v1/disclosures`) for CMS dashboard/collections/entries
- affected repos/files/modules:
  - `cobo_web_design`: `features/cms-core` service/guard/pages updates for B1 live wiring
  - `cobo_iam_services`: runtime compose verification only
- important contracts/behaviors/constraints/decisions:
  - temporary bridge approach: use current APIs until `/api/v1/platform/cms/*` is implemented
  - permission aliases in FE preserve behavior for existing canonical permission set
- build/verification result:
  - fresh docker rebuild succeeded for `web`; compose services healthy
- remaining gaps/risks/next steps:
  - implement platform-prefixed CMS APIs in backend to remove FE bridge and lock long-term contract

## 2026-04-27 - CMS Prefix Cutover (Backend + FE Rollout)

- task type: implement
- objective: implement real backend platform CMS prefix (`/api/v1/platform/cms/*`) and migrate FE to target contract with safe fallback rollout
- what was implemented:
  - backend:
    - added new handler `internal/platformcms/transport/http/handler.go`
    - exposed endpoints:
      - `GET /api/v1/platform/cms/dashboard/summary`
      - `GET /api/v1/platform/cms/collections`
      - `GET /api/v1/platform/cms/entries`
    - registered handler in `internal/httpserver/server.go`
    - enforced permission gates using effective-access checks:
      - dashboard: `platform.cms.view` or `rbac.manage` or `system.settings`
      - collections/entries: `disclosure.view|disclosure.create|disclosure.edit|rbac.manage|system.settings`
    - added integration test coverage in `internal/httpserver/server_test.go` for success + forbidden paths
  - frontend:
    - switched B1 data reads to target prefix endpoints in `cmsApi`
    - added safe fallback to legacy endpoints when prefix returns `404/501`
    - updated CMS landing test mocks for new prefix endpoints
    - route specs for dashboard/collections/entries now point to target prefix contract
- affected repos/files/modules:
  - `cobo_iam_services/internal/platformcms/transport/http/handler.go`
  - `cobo_iam_services/internal/httpserver/server.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
  - `cobo_web_design/src/features/cms-core/services/cmsApi.ts`
  - `cobo_web_design/src/features/cms-core/routeSpecs.ts`
  - `cobo_web_design/src/App.cms-landing.test.tsx`
- important contracts/behaviors/constraints/decisions:
  - prefix APIs return lightweight CMS-ready payloads while preserving current tenant-token auth model
  - FE fallback keeps rollout safe across mixed environments during deployment window
- build/verification result:
  - backend: `go test ./internal/httpserver -count=1` passed
  - frontend: `npm run lint` and `npm run test -- App.cms-landing.test.tsx` passed
  - fresh docker rebuild passed for both `api` and `web`; compose healthy (`api`, `web`, `mysql`, `redis`)
- remaining gaps/risks/next steps:
  - temporary fallback should be removed after all environments confirm prefix availability
  - next: implement route-specific platform permissions (`cms.content.read`, etc.) in IAM authz tables and remove alias bridge in FE

## 2026-04-27 - CMS Strict Contract Cutover Sync

- task type: implement
- objective: sync strict-contract rollout update after FE fallback removal
- what was implemented/discovered:
  - no backend code changes in this step
  - FE fallback removed and strict prefix-only calls enabled for first CMS routes
- affected repos/files/modules:
  - `cobo_web_design/src/features/cms-core/services/cmsApi.ts`
  - `cobo_iam_services`: compose/runtime verification only
- important contracts/behaviors/constraints/decisions:
  - strict mode means `/api/v1/platform/cms/*` availability is mandatory in all environments
- build/verification result:
  - frontend lint/test passed
  - fresh docker rebuild passed for `web`; compose healthy
- remaining gaps/risks/next steps:
  - keep backend prefix endpoints backward-compatible while expanding additional CMS modules

## 2026-04-27 - Phase Checklist Recheck (A1..F)

- task type: understand
- objective: recheck and export actionable Done/Partial/Pending checklist by sub-phase A1..F for team tracking
- discovered:
  - completed: A1, A2, A3, B1, C1, CMS Phase A, CMS B1, CMS prefix cutover, strict no-fallback FE contract
  - partial: B2/B3, F hardening
  - pending: C2/C3, D1/D2, E1/E2/E3
- affected repos/files/modules:
  - status synthesis based on `docs/cross_repo_step_by_step_implementation_plan.md` and cross-repo reusable updates
- important contracts/behaviors/constraints/decisions:
  - CMS first 3 routes are now strict on `/api/v1/platform/cms/*`
  - unresolved plan items focus on remaining vertical slices and system hardening gates
- build/verification result:
  - analysis-only task; no runtime code changes
- remaining gaps/risks/next steps:
  - convert checklist statuses into sprint tickets for C2/C3 -> D -> E -> F closure path

## 2026-04-27 - CMS Flow Recheck + Next Steps

- task type: understand
- objective: recheck current CMS FE/BE flow and propose concrete next steps to complete CMS end-to-end
- discovered:
  - FE flow is stable for entry path: login -> post-login redirect -> `/cms` guarded by `RequirePlatformAccess`
  - FE strict contract is active for first CMS routes (`dashboard`, `collections`, `entries`) via `/api/v1/platform/cms/*`
  - BE prefix endpoints exist for first 3 routes in `internal/platformcms/transport/http/handler.go` with permission checks
  - remaining CMS routes are still scaffold-only in FE and have no corresponding backend prefix endpoints yet
- affected repos/files/modules:
  - FE: `src/App.tsx`, `src/features/cms-core/pages.tsx`, `src/features/cms-core/services/cmsApi.ts`
  - BE: `internal/platformcms/transport/http/handler.go`, `internal/iam/transport/http/me_handler.go`
- important contracts/behaviors/constraints/decisions:
  - strict no-fallback FE contract must be preserved
  - endpoint availability gaps must be fixed in backend/env, not by FE rollback
- build/verification result:
  - analysis-only task; no runtime code changes
- remaining gaps/risks/next steps:
  - implement next CMS backend prefixes (collection detail, entry detail/save, review queue, schedule) then wire FE screens from scaffold to live hooks

## 2026-04-27 - CMS Core API Expansion (Prefix Contract Envelope)

- task type: implement
- objective: expand `/api/v1/platform/cms/*` with collection detail, entries CRUD, reviews, schedules and standardize success envelope + error behavior
- what was implemented:
  - extended `platformcms` handler endpoints:
    - `GET /api/v1/platform/cms/collections/{collection_id}`
    - `GET /api/v1/platform/cms/entries/{entry_id}`
    - `POST /api/v1/platform/cms/entries`
    - `PUT /api/v1/platform/cms/entries/{entry_id}`
    - `GET /api/v1/platform/cms/reviews`
    - `POST /api/v1/platform/cms/reviews/{entry_id}` (`decision=approve|reject`)
    - `GET /api/v1/platform/cms/schedules`
    - `POST /api/v1/platform/cms/schedules`
    - `DELETE /api/v1/platform/cms/schedules/{entry_id}`
  - standardized prefix CMS success responses with envelope:
    - `{ "data": <payload>, "meta"?: { ... } }`
  - kept platform error handling via existing `httpx.WriteError` + `perr` codes (invalid request, unauthorized, forbidden, not found, etc.)
  - wired constructor update in `internal/httpserver/server.go` (inject `disclosureSvc`)
  - expanded integration tests for new entries/reviews/schedules flow and envelope assertions
- affected repos/files/modules:
  - `cobo_iam_services/internal/platformcms/transport/http/handler.go`
  - `cobo_iam_services/internal/httpserver/server.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
- important contracts/behaviors/constraints/decisions:
  - prefix CMS now has one unified response envelope contract for all successful responses
  - review approval reuses disclosure confirm flow; review reject maps to disclosure status rollback (`Draft`) through repository update
  - schedule APIs operate on disclosure `planned_date` with strict `YYYY-MM-DD` validation
- build/verification result:
  - `go test ./internal/httpserver -run TestIntegration_platformCMSPrefix -count=1` passed
  - `go test ./internal/httpserver -count=1` passed
  - fresh docker rebuild succeeded for `api` and `web`; compose status healthy (`api`, `web`, `mysql`, `redis`)
- remaining gaps/risks/next steps:
  - frontend should continue migrating additional CMS routes to new prefix endpoints beyond dashboard/collections/entries
  - next: add dedicated FE services/pages for collection detail, entry editor/detail, review queue, schedule calendar using the new envelope contract

## 2026-04-27 - CMS FE Wiring Sync (No Backend Change)

- task type: implement
- objective: sync backend repo cache after FE wired additional CMS pages to already-implemented prefix APIs
- what was implemented/discovered:
  - no backend source code changes in this cycle
  - FE now consumes existing backend endpoints for:
    - collection detail
    - entry detail/create/update
    - review queue list/action
    - schedule list/create/delete
- affected repos/files/modules:
  - `cobo_web_design/src/features/cms-core/pages.tsx`
  - `cobo_web_design/src/features/cms-core/services/cmsApi.ts`
  - `cobo_web_design/src/features/cms-core/permissionGuards.ts`
- important contracts/behaviors/constraints/decisions:
  - backend prefix contract and envelope remain unchanged in IAM repo
  - FE strict mode remains enabled (no fallback path)
- build/verification result:
  - frontend lint/test passed
  - fresh docker rebuild succeeded for `web`; compose service running
- remaining gaps/risks/next steps:
  - if FE surfaces new API field mismatches in UAT, adjust backend payload compatibility without breaking existing prefix envelope

## 2026-04-27 - CMS Prefix Smoke Script (Core Flow)

- task type: implement
- objective: add runnable DB/compose smoke for CMS prefix core flow to validate new endpoints together
- what was implemented:
  - added `docs/scripts/cms-core-prefix-smoke.ps1` covering:
    - login/select-company token acquisition (user + admin)
    - create/get/update entry via `/api/v1/platform/cms/entries*`
    - collection detail read via `/api/v1/platform/cms/collections/{id}`
    - schedule create/list/delete via `/api/v1/platform/cms/schedules*`
    - submit disclosure then review list/action via `/api/v1/platform/cms/reviews*`
    - envelope assertions for key responses (`data` object required)
  - executed smoke script successfully against local compose API
- affected repos/files/modules:
  - `cobo_iam_services/docs/scripts/cms-core-prefix-smoke.ps1`
- important contracts/behaviors/constraints/decisions:
  - script validates strict prefix contract end-to-end without FE fallback assumptions
  - review action uses unified endpoint with `decision` payload
- build/verification result:
  - `powershell -ExecutionPolicy Bypass -File docs/scripts/cms-core-prefix-smoke.ps1` passed
  - fresh docker rebuild succeeded for `api` + `web`; compose healthy (`api`, `web`, `mysql`, `redis`)
- remaining gaps/risks/next steps:
  - optional: integrate this smoke into wrapper cycle script for one-command QA gate

## 2026-04-27 - CMS Forbidden/Error Matrix Sync (FE-only)

- task type: implement
- objective: sync cache after FE added forbidden/error regression matrix for 4 CMS core pages
- what was implemented/discovered:
  - no backend code changes in this cycle
  - FE test suite now includes explicit forbidden/error cases before releases phase handoff
- affected repos/files/modules:
  - `cobo_web_design/src/features/cms-core/pages.regression.test.tsx`
- important contracts/behaviors/constraints/decisions:
  - backend prefix contract unchanged; validations are FE guard/error mapping focused
- build/verification result:
  - frontend lint and tests passed (15 tests)
  - fresh docker rebuild succeeded for `web`; compose service running
- remaining gaps/risks/next steps:
  - backend remains ready for next releases endpoint expansion; FE can proceed with release page wiring

## 2026-04-27 - Compose Web Proxy Target Sync

- task type: implement
- objective: ensure web container proxy forwards `/api/*` to IAM API service in docker network
- what was implemented:
  - added `API_PROXY_TARGET: http://api:8080` to `web` service environment in compose
  - reran `web` (and dependent startup sequence) to apply new env
- affected repos/files/modules:
  - `cobo_iam_services/docker-compose.dev.yml`
- important contracts/behaviors/constraints/decisions:
  - service-name routing (`api:8080`) is required for container-to-container proxy
  - previous `500` observed on `:3000` was startup/proxy connectivity issue; post-fix calls succeed with valid token
- build/verification result:
  - through `:3000`:
    - no token -> `401` expected
    - valid admin token -> `200` for dashboard/collections/entries
- remaining gaps/risks/next steps:
  - when starting stack cold, call CMS endpoints after API reports listening to avoid transient proxy ECONNREFUSED

## 2026-04-27 - Local Run Health-Wait Guard

- task type: implement
- objective: avoid startup race by waiting API ready before local scripts continue with smoke/check steps
- what was implemented:
  - added `Wait-ApiReady` helper (`/healthz` + `/readyz`, retry loop) to:
    - `docs/scripts/run-c1-cycle.ps1`
    - `docs/scripts/cms-core-prefix-smoke.ps1`
  - wired wait calls immediately after compose up in cycle script and before token/API calls in CMS smoke script
- affected repos/files/modules:
  - `cobo_iam_services/docs/scripts/run-c1-cycle.ps1`
  - `cobo_iam_services/docs/scripts/cms-core-prefix-smoke.ps1`
- important contracts/behaviors/constraints/decisions:
  - readiness gate is non-invasive and does not change API contract
  - prevents transient proxy/API race where web checks fire before API fully listens
- build/verification result:
  - `powershell -ExecutionPolicy Bypass -File docs/scripts/cms-core-prefix-smoke.ps1` passed (with readiness logs)
  - fresh docker rebuild succeeded for `api` + `web`; compose healthy
- remaining gaps/risks/next steps:
  - optional: apply same readiness helper to any other local smoke scripts that hit API immediately after compose start

## 2026-04-27 - CMS Admin Account + Company Flow (Prefix)

- task type: implement
- objective: enable CMS admin flow to create enterprise account with company membership and list company memberships via strict CMS prefix
- what was implemented:
  - extended platform CMS handler with admin-user endpoints:
    - `GET /api/v1/platform/cms/admin/users?company_id=`
    - `POST /api/v1/platform/cms/admin/users`
  - endpoints delegate to existing `companyaccess` admin service (`ListCompanyMemberships`, `CreateUser`) and enforce permissions:
    - `rbac.manage` OR `system.settings`
  - kept standardized CMS envelope (`data` + optional `meta`) for these endpoints
  - updated server wiring to inject `adminSvc` into platform CMS handler
  - added integration coverage:
    - create admin user with company membership via prefix
    - list users by company via prefix and assert created user appears
- affected repos/files/modules:
  - `cobo_iam_services/internal/platformcms/transport/http/handler.go`
  - `cobo_iam_services/internal/httpserver/server.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
- important contracts/behaviors/constraints/decisions:
  - flow is company-scoped: `company_id` required for create; list defaults to token company when query is empty
  - response contract aligned with CMS strict prefix strategy; no fallback to legacy FE bridge
- build/verification result:
  - `go test ./internal/httpserver -run TestIntegration_platformCMSPrefix -count=1` passed
  - `go test ./internal/httpserver -count=1` passed
  - fresh docker rebuild succeeded for `api` + `web`; compose healthy
- remaining gaps/risks/next steps:
  - optional next step: expose role assignment shortcut in same CMS admin page after create-user for one-click bootstrap

## 2026-04-27 - Week 2 Plan Recheck Sync

- task type: understand
- objective: sync cross-repo Week 2 recheck status (`B2/B3/C1`) against implementation plan
- discovered:
  - Week 2 target remains `B2 + B3 + C1`
  - `C1` is closed by existing disclosure contract/migration/test cycles
  - `B2` and `B3` are still pending/partial from plan acceptance perspective
  - recent CMS admin/platform expansions are additive and do not automatically close `B2/B3`
- affected repos/files/modules:
  - `cobo_web_design/docs/cross_repo_step_by_step_implementation_plan.md`
  - cross-repo `docs/ai-cache/reusable-task-updates.md`
- important contracts/behaviors/constraints/decisions:
  - keep Week 2 closure criteria tied to original plan deliverables, not only CMS side progress
- build/verification result:
  - analysis-only task; no runtime code changes
- remaining gaps/risks/next steps:
  - complete and verify `B2` then `B3` to mark Week 2 done end-to-end

## 2026-04-27 - Week 2 B2 Entitlement Hardcode Removal Sync (FE-focused)

- task type: implement
- objective: sync cross-repo status after closing Week 2 B2 entitlement hardcode removal in frontend
- what was implemented/discovered:
  - no backend code changes were required for this cycle
  - FE now uses backend `subscription_tier` as entitlement source and fails safe to `Free` when missing
  - role-label-based premium check (`Premium`/`Enterprise` role names) was removed from portal gating logic
- affected repos/files/modules:
  - `cobo_web_design/src/pages/portal/DisclosureTypeDetail.tsx`
  - `cobo_web_design/src/services/normalizers.ts`
  - `cobo_web_design/src/services/normalizers.me.test.ts`
- important contracts/behaviors/constraints/decisions:
  - backend field `subscription_tier` remains contract source; FE should not infer entitlement from role strings
  - fail-closed behavior preserved for missing entitlement data
- build/verification result:
  - frontend lint/tests passed for B2 changes
  - fresh docker rebuild succeeded for affected service `web`
- remaining gaps/risks/next steps:
  - proceed to Week 2 `B3` payload hardening work

## 2026-04-27 - Week 2 B3 Company Payload Hardening Sync (FE-focused)

- task type: implement
- objective: sync cross-repo status after completing Week 2 B3 company list payload hardening in frontend
- what was implemented/discovered:
  - no backend code changes required for this cycle
  - FE company normalizers now handle sparse/minimal payloads safely with stable fallback ids/names/codes
  - FE pre-company membership mapping is hardened to avoid empty company selectors in `/select-company` flow
  - regression tests added for minimal companies payload and login membership fallback mapping
- affected repos/files/modules:
  - `cobo_web_design/src/services/normalizers.ts`
  - `cobo_web_design/src/App.tsx`
  - `cobo_web_design/src/services/normalizers.companies.test.ts`
- important contracts/behaviors/constraints/decisions:
  - optional company fields (`region`, `last_accessed`) remain optional and UI-safe
  - FE preserves fail-safe rendering when required identity fields are partially missing
- build/verification result:
  - frontend lint/tests passed for B3 scope
  - fresh docker rebuild succeeded for affected service `web`
- remaining gaps/risks/next steps:
  - Week 2 FE stabilization track (`B2/B3`) is now complete; proceed with next planned priority

## 2026-04-27 - Login Redirect Recheck Sync (Enterprise User -> CMS)

- task type: implement
- objective: sync cross-repo status after fixing enterprise user post-login redirect unexpectedly landing on CMS
- what was implemented/discovered:
  - no backend code changes required
  - FE redirect logic now uses strict `platform.cms.view` check for landing to `/cms`
  - regression tests confirm non-cms enterprise/admin user with `system.settings` no longer auto-redirects to `/cms`
- affected repos/files/modules:
  - `cobo_web_design/src/App.tsx`
  - `cobo_web_design/src/App.cms-landing.test.tsx`
- important contracts/behaviors/constraints/decisions:
  - this cycle changes landing behavior only; `/cms` guard policy remains unchanged
- build/verification result:
  - frontend lint/tests passed for redirect fix
  - fresh docker rebuild succeeded for affected service `web`
- remaining gaps/risks/next steps:
  - optional: align `/cms` route guard strictness with new landing rule if product policy requires explicit CMS permission everywhere

## 2026-04-27 - CMS Guard Strict Mode Sync (platform.cms.view only)

- task type: implement
- objective: sync cross-repo status after tightening `/cms` guard to strict explicit CMS permission
- what was implemented/discovered:
  - no backend code changes required in this cycle
  - FE `/cms` route guard now requires `platform.cms.view` only (no `system.settings`/`rbac.manage` alias)
  - regression tests confirm strict allow/deny behavior for CMS route access
- affected repos/files/modules:
  - `cobo_web_design/src/App.tsx`
  - `cobo_web_design/src/App.cms-landing.test.tsx`
- important contracts/behaviors/constraints/decisions:
  - CMS access policy is now consistent across landing + route guard and tied to explicit platform permission
- build/verification result:
  - frontend lint/tests passed for strict guard update
  - fresh docker rebuild succeeded for affected service `web`
- remaining gaps/risks/next steps:
  - optional next step: audit other CMS gate call sites to ensure they do not rely on legacy alias semantics

## 2026-04-27 - CMS Strict Policy Hardening Sync (Matrix + Alias Guards)

- task type: implement
- objective: sync cross-repo status after FE hardened CMS strict policy beyond route landing guard
- what was implemented/discovered:
  - no backend code changes in this cycle
  - FE removed legacy alias acceptance for CMS entry capability in both route matrix and CMS permission guard aliases
  - new permission-guard unit tests lock strict behavior and prevent regressions
  - runtime check confirms `admin.dn@example.com` effective-access currently lacks `platform.cms.view`; capability endpoint still marks platform_cms true due backend alias logic
- affected repos/files/modules:
  - `cobo_web_design/src/config/menuPermissionMatrix.ts`
  - `cobo_web_design/src/features/cms-core/permissionGuards.ts`
  - `cobo_web_design/src/features/cms-core/permissionGuards.test.ts`
- important contracts/behaviors/constraints/decisions:
  - FE CMS entry now consistently requires explicit `platform.cms.view`
  - backend capability semantics are not yet strict and may need follow-up alignment
- build/verification result:
  - frontend lint/tests passed for hardening changes
  - fresh docker rebuild succeeded for affected service `web`
- remaining gaps/risks/next steps:
  - optional next: align IAM `/me/capabilities.modules.platform_cms` and platform CMS backend permission checks to strict `platform.cms.view` if policy should be end-to-end strict

## 2026-04-27 - Backend CMS Strict Policy (End-to-End)

- task type: implement
- objective: enforce strict backend policy so CMS capability and `/api/v1/platform/cms/*` entry-level access require explicit `platform.cms.view`
- what was implemented:
  - updated `GET /api/v1/me/capabilities`:
    - `modules.platform_cms` now checks only `platform.cms.view` (removed `rbac.manage/system.settings` alias)
  - hardened platform CMS handler with strict entry-level gate:
    - added `requireCMSAccess(... platform.cms.view ...)`
    - applied to all `/api/v1/platform/cms/*` handlers before action-level permission checks
    - dashboard summary now returns `platform_cms: true` only after strict gate passes
  - updated in-memory fixtures for integration tests:
    - added dedicated CMS operator account `cms.operator@example.com` -> `u_cms` / `m_cms_001`
    - granted explicit `platform.cms.view` (+ existing CMS action permissions) in in-memory auth repository
  - updated integration tests to use CMS operator for CMS success paths and verify strict deny for `admin.dn@example.com` on CMS dashboard
- affected repos/files/modules:
  - `cobo_iam_services/internal/iam/transport/http/me_handler.go`
  - `cobo_iam_services/internal/platformcms/transport/http/handler.go`
  - `cobo_iam_services/internal/httpserver/server.go`
  - `cobo_iam_services/internal/companyaccess/infra/inmemory/membership_query.go`
  - `cobo_iam_services/internal/authorization/infra/inmemory/repository.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
- important contracts/behaviors/constraints/decisions:
  - CMS access policy is now strict server-side: admin aliases (`rbac.manage`, `system.settings`) are insufficient without `platform.cms.view`
  - action-level permission checks remain in place after strict entry gate (defense in depth)
  - in-memory test fixture adds explicit CMS actor to preserve positive integration coverage under strict policy
- build/verification result:
  - `go test ./internal/httpserver -run "TestIntegration_meCapabilities_platformCmsMatrix|TestIntegration_platformCMSPrefix_dashboardCollectionsEntries|TestIntegration_platformCMSPrefix_entriesReviewsSchedulesContract|TestIntegration_platformCMSPrefix_adminUsersCreateAndList" -count=1` passed
  - fresh docker rebuild succeeded for affected services `api` + `web`; both containers running
- remaining gaps/risks/next steps:
  - DB-mode seed currently may not include an explicit CMS account with `platform.cms.view`; add canonical seed/role mapping if required for compose DB smoke using real account credentials

## 2026-04-27 - DB Canonical CMS Account Seed (cms.operator)

- task type: implement
- objective: add canonical DB-mode CMS account so strict CMS smoke can run without in-memory-only fixtures
- what was implemented:
  - added canonical CMS operator identity in both seed paths:
    - user: `cms.operator@example.com` (`u_cms_operator`)
    - credential: password `secret` hash
    - membership: `m_106` in `c_001`
    - role: `cms_operator` (`r...016`)
  - added explicit CMS permission seed:
    - permission `platform.cms.view` (`10000000-0001-4000-8000-00000000000d`)
    - role-permission mapping grants CMS operator both `platform.cms.view` and existing CMS action permissions
  - wired related org/dept/responsibility seed records for the new membership (`department_memberships`, `membership_effective_responsibilities`, `org_unit_memberships`)
  - updated DB smoke script `cms-core-prefix-smoke.ps1`:
    - switched CMS actor defaults from `admin.dn@example.com` to `cms.operator@example.com`
    - changed CMS prefix API calls to use CMS token consistently under strict entry-level policy
- affected repos/files/modules:
  - `cobo_iam_services/migrations/seed_dev_identity_authorization.sql`
  - `cobo_iam_services/migrations/0009_seed_authz_test_accounts.up.sql`
  - `cobo_iam_services/docs/scripts/cms-core-prefix-smoke.ps1`
- important contracts/behaviors/constraints/decisions:
  - strict CMS policy is preserved: enterprise admin account without `platform.cms.view` is not used as CMS smoke actor
  - DB-mode smoke now validates canonical explicit CMS entitlement, not alias permissions
- build/verification result:
  - `go test ./internal/httpserver -run "TestIntegration_meCapabilities_platformCmsMatrix|TestIntegration_platformCMSPrefix_dashboardCollectionsEntries|TestIntegration_platformCMSPrefix_entriesReviewsSchedulesContract|TestIntegration_platformCMSPrefix_adminUsersCreateAndList" -count=1` passed
  - `powershell -ExecutionPolicy Bypass -File docs/scripts/cms-core-prefix-smoke.ps1 ...` passed after applying updated dev seed
  - fresh docker rebuild succeeded for affected services `api` + `web`; both containers running
- remaining gaps/risks/next steps:
  - optional: thread new `-CmsLogin/-CmsPassword` params through wrapper scripts if team wants one-command DB smoke from a single entrypoint

## 2026-04-27 - Wrapper C1 Cycle Threaded CMS Params

- task type: implement
- objective: extend one-command wrapper so it can run strict CMS DB smoke end-to-end using canonical CMS account credentials
- what was implemented:
  - updated `docs/scripts/run-c1-cycle.ps1` params:
    - added `-CmsLogin` (default `cms.operator@example.com`)
    - added `-CmsPassword` (default `secret`)
  - inserted CMS smoke into wrapper execution flow:
    - new step calls `docs/scripts/cms-core-prefix-smoke.ps1` with `-CmsLogin/-CmsPassword`
    - kept disclosure DB smoke and fresh docker rebuild steps intact
  - adjusted step numbering to reflect added CMS smoke stage
- affected repos/files/modules:
  - `cobo_iam_services/docs/scripts/run-c1-cycle.ps1`
- important contracts/behaviors/constraints/decisions:
  - wrapper now validates both disclosure C1 and strict CMS prefix contract in one run
  - CMS smoke explicitly uses CMS operator account to stay aligned with strict `platform.cms.view` policy
- build/verification result:
  - executed wrapper command end-to-end successfully:
    - backend tests passed
    - frontend contract tests passed
    - disclosure DB smoke passed
    - CMS DB smoke passed
    - fresh no-cache docker rebuild passed for `api` + `web`
    - services recovered healthy after restart (`api`, `web`, `mysql`, `redis`)
- remaining gaps/risks/next steps:
  - none critical; wrapper is now ready for QA/dev one-command strict cycle in DB mode

## 2026-04-27 - CMS Releases Slice (P1)

- task type: implement
- objective: implement strict contract-first releases slice for CMS prefix and wire FE to live endpoint
- what was implemented:
  - added `GET /api/v1/platform/cms/releases` in platform CMS handler:
    - strict entry gate via `requireCMSAccess(... platform.cms.view ...)`
    - action gate via publish/admin permissions (`disclosure.publish|rbac.manage|system.settings`)
    - response envelope normalized to `data.items` + `meta.total`
    - supports `entry_id` filter and sorts by `published_date` desc
  - extended integration matrix test to cover releases success path and strict forbidden path for `admin.dn@example.com`
  - FE wired `CmsReleasesPage` to real API via `getReleases()` and mapped UI states
  - FE added `cms.publish.read` alias mapping and regression tests for releases happy/forbidden/error
- affected repos/files/modules:
  - `cobo_iam_services/internal/platformcms/transport/http/handler.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
  - `cobo_web_design/src/features/cms-core/services/cmsApi.ts`
  - `cobo_web_design/src/features/cms-core/pages.tsx`
  - `cobo_web_design/src/features/cms-core/permissionGuards.ts`
  - `cobo_web_design/src/features/cms-core/pages.regression.test.tsx`
  - `cobo_web_design/src/features/cms-core/permissionGuards.test.ts`
- important contracts/behaviors/constraints/decisions:
  - strict CMS access policy remains E2E; releases endpoint inherits explicit `platform.cms.view` entry gate
  - release history stays read-only and filtered by publication states (`published/completed/confirmed`)
- build/verification result:
  - `go test ./internal/httpserver -run TestIntegration_platformCMSPrefix_dashboardCollectionsEntries` passed
  - `npm run test -- src/features/cms-core/pages.regression.test.tsx src/features/cms-core/permissionGuards.test.ts` passed
  - fresh docker rebuild/restart passed on affected services (`api`, `web`)
  - post-rebuild smoke passed: `GET /healthz` = 200, `GET /api/v1/platform/cms/releases` = 200 (cms.operator token)
- remaining gaps/risks/next steps:
  - P2 remains: implement live APIs/screens for remaining scaffold routes (`roles`, `rules`, `audit`, `sessions`, `health`)

## 2026-04-27 - CMS P2 Live APIs (roles/rules/audit/sessions/health)

- task type: implement
- objective: complete P2 by adding strict CMS prefix APIs for roles/rules/audit/sessions/health and wiring FE to these contracts
- what was implemented:
  - added new strict CMS prefix endpoints in platform handler:
    - `GET /api/v1/platform/cms/admin/roles`
    - `GET /api/v1/platform/cms/admin/rules`
    - `POST /api/v1/platform/cms/admin/rules/validate`
    - `GET /api/v1/platform/cms/ops/audit`
    - `GET /api/v1/platform/cms/ops/sessions`
    - `POST /api/v1/platform/cms/ops/sessions/{session_id}/revoke`
    - `GET /api/v1/platform/cms/ops/health`
  - all new handlers enforce strict entry gate via `requireCMSAccess(...platform.cms.view...)`
  - all responses follow CMS envelope `data` + `meta`
  - expanded integration test `TestIntegration_platformCMSPrefix_dashboardCollectionsEntries` to cover all new endpoints + strict forbid for `admin.dn@example.com`
  - FE P2 pages now call these endpoints directly (live wiring completed)
- affected repos/files/modules:
  - `cobo_iam_services/internal/platformcms/transport/http/handler.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
  - `cobo_web_design/src/features/cms-core/services/cmsApi.ts`
  - `cobo_web_design/src/features/cms-core/pages.tsx`
  - `cobo_web_design/src/features/cms-core/pages.regression.test.tsx`
  - `cobo_web_design/src/features/cms-core/permissionGuards.ts`
  - `cobo_web_design/src/features/cms-core/permissionGuards.test.ts`
- important contracts/behaviors/constraints/decisions:
  - strict CMS policy remains E2E: no `/api/v1/platform/cms/*` access without explicit `platform.cms.view`
  - action endpoints for P2 modules are now available for FE (`validate rule`, `revoke session`)
  - payloads are intentionally minimal bootstrap contracts for safe rollout
- build/verification result:
  - backend test passed: `go test ./internal/httpserver -run TestIntegration_platformCMSPrefix_dashboardCollectionsEntries`
  - frontend tests passed: `npm run test -- src/features/cms-core/pages.regression.test.tsx src/features/cms-core/permissionGuards.test.ts`
  - fresh docker rebuild passed for affected services (`api`, `web`)
  - post-rebuild smoke passed (`200`) for:
    - `/api/v1/platform/cms/admin/roles`
    - `/api/v1/platform/cms/admin/rules`
    - `/api/v1/platform/cms/admin/rules/validate`
    - `/api/v1/platform/cms/ops/audit`
    - `/api/v1/platform/cms/ops/sessions`
    - `/api/v1/platform/cms/ops/health`
- remaining gaps/risks/next steps:
  - next cycle can replace bootstrap P2 payload generators with persistent/domain-backed read models while preserving route/error envelopes

## 2026-04-27 - CMS P2 Hardening (Domain-backed audit/sessions/health)

- task type: implement
- objective: harden P2 backend payload generation to use domain data sources while preserving existing FE contracts
- what was implemented:
  - audit module hardened to domain-backed reads:
    - extended audit repository contract with `ListByCompany(ctx, companyID, limit)`
    - implemented MySQL + in-memory `ListByCompany` adapters
    - `/api/v1/platform/cms/ops/audit` now returns rows derived from `audit_logs` instead of synthetic static events
  - sessions module hardened to IAM domain service:
    - `/api/v1/platform/cms/ops/sessions` now calls `iamSvc.ListSessions(...)`
    - `/api/v1/platform/cms/ops/sessions/{session_id}/revoke` now calls `iamSvc.RevokeSession(...)`
  - health module hardened to runtime dependency checks:
    - `/api/v1/platform/cms/ops/health` now derives component status from real checks (`authorizer.GetEffectiveAccess`, `disclosures.List`)
  - platform CMS handler wiring updated to inject `iamSvc` and `auditRepo`
  - integration test updated to revoke an actual returned `session_id` from sessions list
- affected repos/files/modules:
  - `cobo_iam_services/internal/audit/app/contracts.go`
  - `cobo_iam_services/internal/audit/infra/inmemory/repository.go`
  - `cobo_iam_services/internal/audit/infra/mysql/repository.go`
  - `cobo_iam_services/internal/platformcms/transport/http/handler.go`
  - `cobo_iam_services/internal/httpserver/server.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
- important contracts/behaviors/constraints/decisions:
  - FE contract for P2 endpoints is preserved (`data.items` + `meta.total`, same route paths)
  - strict CMS entry policy remains unchanged (`platform.cms.view` required at entry-level)
  - hardening is backward-compatible for FE while replacing synthetic payload generation with domain-backed sources
- build/verification result:
  - backend test passed: `go test ./internal/httpserver -run TestIntegration_platformCMSPrefix_dashboardCollectionsEntries`
  - frontend regression remained green: `npm run test -- src/features/cms-core/pages.regression.test.tsx src/features/cms-core/permissionGuards.test.ts`
  - fresh docker rebuild + restart succeeded for `api` + `web`
  - post-rebuild smoke passed:
    - `audit_total=0`
    - `sessions_total=9`
    - `health_total=3`
    - `revoke_status=200`
- remaining gaps/risks/next steps:
  - audit data in non-DB/in-memory mode stays sparse unless new actions append logs; can add explicit CMS audit append points in next cycle if richer feed is needed

## 2026-04-27 - CMS Ops Audit Append Points (validate/revoke)

- task type: implement
- objective: append audit records for CMS ops actions so audit screen is populated even on fresh envs
- what was implemented:
  - injected `auditSvc` into platform CMS handler wiring
  - added explicit audit append calls in:
    - `POST /api/v1/platform/cms/admin/rules/validate` with action `cms.rule.validate`
    - `POST /api/v1/platform/cms/ops/sessions/{session_id}/revoke` with action `cms.session.revoke`
  - kept API contract unchanged for all P2 endpoints (`data/meta` envelope and existing fields)
  - updated integration test to verify audit list reflects post-operation entries
- affected repos/files/modules:
  - `cobo_iam_services/internal/platformcms/transport/http/handler.go`
  - `cobo_iam_services/internal/httpserver/server.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
- important contracts/behaviors/constraints/decisions:
  - strict CMS entry policy remains unchanged (`platform.cms.view`)
  - audit append errors are non-blocking for business response in current implementation (best-effort logging)
  - FE does not require changes; contract remains stable
- build/verification result:
  - `go test ./internal/httpserver -run TestIntegration_platformCMSPrefix_dashboardCollectionsEntries` passed
  - `npm run test -- src/features/cms-core/pages.regression.test.tsx src/features/cms-core/permissionGuards.test.ts` passed
  - fresh docker rebuild + restart passed for `api` + `web`
  - smoke check confirmed audit density increase after ops:
    - `audit_before=0`
    - `audit_after=2`
- remaining gaps/risks/next steps:
  - optional: append audit for additional CMS ops actions (e.g. schedule create/delete, review decision) to further enrich timeline

## 2026-04-27 - CMS Write Actions Audit Append Expansion

- task type: implement
- objective: append audit entries for remaining CMS write actions so audit timeline reflects full content workflow
- what was implemented:
  - added audit append calls for:
    - entries: `cms.entry.create`, `cms.entry.update`
    - reviews: `cms.review.approve`, `cms.review.reject`
    - schedules: `cms.schedule.create`, `cms.schedule.delete`
  - each append includes actor/company context and resource identifiers; schedule/create also includes publish date metadata
  - expanded integration test to assert audit feed contains expected write-action events after full entriesÃ¢â€ â€™reviewsÃ¢â€ â€™schedules flow
- affected repos/files/modules:
  - `cobo_iam_services/internal/platformcms/transport/http/handler.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
- important contracts/behaviors/constraints/decisions:
  - no FE/API contract changes; endpoints still return same envelope and payload shape
  - strict `platform.cms.view` policy remains unchanged
  - audit append remains best-effort (non-blocking for primary write response)
- build/verification result:
  - backend tests passed:
    - `go test ./internal/httpserver -run "TestIntegration_platformCMSPrefix_entriesReviewsSchedulesContract|TestIntegration_platformCMSPrefix_dashboardCollectionsEntries"`
  - frontend regression remained green:
    - `npm run test -- src/features/cms-core/pages.regression.test.tsx src/features/cms-core/permissionGuards.test.ts`
  - fresh docker rebuild + restart passed for `api` + `web`
  - smoke verification for full write flow:
    - `audit_before=2`
    - `audit_after=7`
- remaining gaps/risks/next steps:
  - optional: add FE filter chips by audit action type if timeline volume increases

## 2026-04-27 - CMS Audit Action Filter (`?action=`) API

- task type: implement
- objective: add backend filter by audit action for CMS timeline endpoint so FE can query grouped event streams
- what was implemented:
  - extended audit repository contract and adapters to support action filter:
    - `ListByCompany(ctx, companyID, action, limit)`
  - updated `/api/v1/platform/cms/ops/audit` handler to parse optional `?action=` and apply filter at repository level
  - expanded integration test to verify filtered feed only contains requested action (`cms.entry.create`) and empty set for unmatched action
- affected repos/files/modules:
  - `cobo_iam_services/internal/audit/app/contracts.go`
  - `cobo_iam_services/internal/audit/infra/inmemory/repository.go`
  - `cobo_iam_services/internal/audit/infra/mysql/repository.go`
  - `cobo_iam_services/internal/platformcms/transport/http/handler.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
- important contracts/behaviors/constraints/decisions:
  - backward-compatible: omitting `action` keeps previous behavior
  - API response envelope unchanged (`data.items`, `meta.total`)
  - filter matches exact action code (case-sensitive in MySQL query path)
- build/verification result:
  - backend tests passed:
    - `go test ./internal/httpserver -run "TestIntegration_platformCMSPrefix_entriesReviewsSchedulesContract|TestIntegration_platformCMSPrefix_dashboardCollectionsEntries"`
  - frontend regression passed unchanged
  - fresh docker rebuild + restart passed for `api` + `web`
  - smoke verification:
    - `audit_all=7`
    - `audit_filtered=1` (`action=cms.entry.create`)
    - `audit_invalid=0` (`action=non.existent.action`)
    - `filtered_only_match=True`
- remaining gaps/risks/next steps:
  - FE can now add audit action filter UI/route-state sync directly against `?action=` without API changes

## 2026-04-27 - CMS Audit FE Filter Sync Confirmation

- task type: implement
- objective: sync cache after FE wired audit action filter UI/query-state to existing `?action=` API
- what was implemented/discovered:
  - FE now drives audit filter via URL query `action` and calls backend with optional `?action=...`
  - backend contract unchanged; no additional BE code changes required in this step
- affected repos/files/modules:
  - `cobo_web_design/src/features/cms-core/pages.tsx`
  - `cobo_web_design/src/features/cms-core/services/cmsApi.ts`
  - `cobo_web_design/src/features/cms-core/pages.regression.test.tsx`
- important contracts/behaviors/constraints/decisions:
  - audit API remains source-compatible and FE now consumes action filter as intended
- build/verification result:
  - FE regression passed
  - fresh docker rebuild/restart passed for `api` + `web`
  - smoke endpoint check for filtered audit passed (`audit_filtered_total=1`)
- remaining gaps/risks/next steps:
  - optional future step: expose canonical action dictionary endpoint if FE/BE action set grows

## 2026-04-27 - CMS Week 2 Recheck Board (Flow Clusters)

- task type: understand
- objective: provide team-ready Done/Partial/Pending board for CMS by flow cluster after Week 2 recheck
- what was discovered:
  - CMS Week 2 baseline is complete and now expanded with P1/P2 + hardening layers
  - strict authz, contract-first APIs, UI-state handling, smoke checks, and audit observability are all present
- affected repos/files/modules:
  - no code changes
  - primary references:
    - `cobo_iam_services/internal/platformcms/transport/http/handler.go`
    - `cobo_iam_services/internal/httpserver/server_test.go`
    - `cobo_iam_services/migrations/seed_dev_identity_authorization.sql`
    - `cobo_iam_services/migrations/0009_seed_authz_test_accounts.up.sql`
    - `cobo_web_design/src/features/cms-core/*`
- important contracts/behaviors/constraints/decisions:
  - strict `platform.cms.view` entry policy is still the controlling gate for CMS module access
  - audit timeline now supports append-rich write events and action-level filtering
- build/verification result:
  - no additional run in this understand-only recheck step; board is based on latest green verify/build cycles
- remaining gaps/risks/next steps:
  - optional: add centralized action code catalog for FE labels and BE filters

## 2026-04-27 - CMS Action Catalog + Metrics/Trace Layer

- task type: implement
- objective: standardize CMS action catalog across FE/BE and add dedicated CMS metrics/traces observability
- what was implemented:
  - introduced canonical CMS action constants in platform CMS transport layer and reused them for audit append actions
  - added strict action filter validation for `GET /api/v1/platform/cms/ops/audit?action=...` (reject unknown codes)
  - added CMS route-level metrics collector (requests/errors/avg latency/last status/last request id)
  - wrapped all CMS prefix handlers with instrumentation
  - added new observability endpoint:
    - `GET /api/v1/platform/cms/ops/metrics`
  - enriched health payload with `cms_observability` component metadata
  - extended integration tests for metrics endpoint and invalid-action rejection
- affected repos/files/modules:
  - `cobo_iam_services/internal/platformcms/transport/http/audit_actions.go`
  - `cobo_iam_services/internal/platformcms/transport/http/metrics.go`
  - `cobo_iam_services/internal/platformcms/transport/http/handler.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
  - synced FE side action catalog usage:
    - `cobo_web_design/src/features/cms-core/auditActionCatalog.ts`
    - `cobo_web_design/src/features/cms-core/pages.tsx`
- important contracts/behaviors/constraints/decisions:
  - existing endpoint envelopes remain unchanged (`data`, `meta`)
  - observability endpoint is additive and protected by CMS access gate
  - trace correlation is based on existing `X-Request-Id` header
- build/verification result:
  - targeted BE tests passed:
    - `go test ./internal/platformcms/... ./internal/httpserver -run TestIntegration_platformCMSPrefix -count=1`
  - targeted FE tests passed:
    - `npm run test -- src/features/cms-core/pages.regression.test.tsx src/features/cms-core/permissionGuards.test.ts`
  - full cycle + docker rebuild passed:
    - `powershell -ExecutionPolicy Bypass -File docs/scripts/run-c1-cycle.ps1`
    - includes `docker compose -f docker-compose.dev.yml build --no-cache api web`
- remaining gaps/risks/next steps:
  - optional future step: push metrics into external sink (Prometheus/OpenTelemetry) if central dashboards are required

## 2026-04-27 - Week1/Week2 Plan Recheck (Cross-repo)

- task type: understand
- objective: re-audit Week 1 and Week 2 plan completeness after latest CMS expansion cycles
- what was discovered:
  - Week 1 baseline status is mostly complete (auth/session/company switch/forbidden flows + openapi snapshot)
  - remaining Week 1 contract gap: `/api/v1/me` still omits `user.subscription_tier`
  - Week 2 CMS-related execution is effectively complete and expanded beyond original baseline (strict entry policy, prefix contracts, domain-backed P2 flows, audit enrichment, action filter, metrics endpoint)
  - minor documentation update debt remains to align Ã¢â‚¬Å“stub/partialÃ¢â‚¬Â notes with current live CMS state
- affected repos/files/modules:
  - `cobo_iam_services/internal/iam/transport/http/me_handler.go`
  - `cobo_iam_services/docs/PLATFORM_CMS_PREFIX_AND_TENANT.md`
  - `cobo_web_design/src/services/normalizers.ts`
  - `cobo_web_design/docs/week-1-foundation-status-and-plan.md`
  - `cobo_web_design/docs/cross_repo_step_by_step_implementation_plan.md`
- important contracts/behaviors/constraints/decisions:
  - strict CMS gate remains `platform.cms.view` at FE and BE layers
  - additive observability path exists via `/api/v1/platform/cms/ops/metrics` with `X-Request-Id` correlation
  - no identified regression requiring rollback from latest Week 2 CMS slices
- build/verification result:
  - understand-only step; no new build run
  - latest verified cycle remains green via `docs/scripts/run-c1-cycle.ps1` (from previous implementation turn)
- remaining gaps/risks/next steps:
  - implement Week 1 B3 contract closure (`subscription_tier`) + related tests
  - refresh week status docs/checklists to avoid stale planning signals

## 2026-04-27 - Close Week1 B3 + Week Docs Sync

- task type: implement
- objective: close Week 1 B3 by returning `subscription_tier` in `/api/v1/me` and sync Week1/Week2 docs status
- what was implemented:
  - extended IAM identity contract with `SubscriptionTier`
  - populated identity query paths (in-memory + mysql) with safe default `Free`
  - `/api/v1/me` now returns:
    - `user.subscription_tier`
  - added integration test:
    - `TestIntegration_meProfileIncludesSubscriptionTier`
  - updated planning docs in web repo to mark B3 done and clarify Week2 status note
- affected repos/files/modules:
  - `cobo_iam_services/internal/iam/app/contracts.go`
  - `cobo_iam_services/internal/iam/infra/mysql/credentials.go`
  - `cobo_iam_services/internal/iam/infra/inmemory/credentials.go`
  - `cobo_iam_services/internal/iam/transport/http/me_handler.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
  - `cobo_web_design/docs/week-1-foundation-status-and-plan.md`
  - `cobo_web_design/docs/cross_repo_step_by_step_implementation_plan.md`
- important contracts/behaviors/constraints/decisions:
  - contract change is additive (non-breaking for FE clients)
  - backend now owns baseline subscription tier field instead of FE-only fallback inference
  - current default source is `Free`; can be replaced by entitlement-backed source in later phase
- build/verification result:
  - targeted tests passed:
    - `go test ./internal/httpserver -run "TestIntegration_meProfileIncludesSubscriptionTier|TestIntegration_platformCMSPrefix_dashboardCollectionsEntries" -count=1`
    - `npm run test -- src/services/normalizers.me.test.ts src/App.cms-landing.test.tsx`
  - full wrapper + docker rebuild passed:
    - `powershell -ExecutionPolicy Bypass -File docs/scripts/run-c1-cycle.ps1`
    - includes `docker compose -f docker-compose.dev.yml build --no-cache api web`
- remaining gaps/risks/next steps:
  - optional: connect `subscription_tier` to a dedicated entitlement source for non-default tiers

## 2026-04-27 - Subscription Tier from Entitlement Source (DB-backed)

- task type: implement
- objective: replace `/api/v1/me` hardcoded/default-only `subscription_tier` with real entitlement source from dedicated table/config
- what was implemented:
  - introduced entitlement table:
    - `migrations/0011_user_subscription_tiers.up.sql`
    - `migrations/0011_user_subscription_tiers.down.sql`
  - migration includes safe backfill strategy:
    - default `Free` for existing users
    - deterministic overrides for seeded privileged accounts (`Premium`/`Enterprise`)
  - wired MySQL identity query to load active tier from `user_subscription_tiers` with effective window support
  - in-memory identity path now supports `SubscriptionTier` on static users (for local non-DB parity)
  - updated dev migration runner order to ensure table exists before seed scripts that write tier data:
    - `migrations/run_dev_migrations.sh`
  - updated seed scripts to write explicit tier rows:
    - `migrations/seed_dev_identity_authorization.sql`
    - `migrations/0009_seed_authz_test_accounts.up.sql`
  - strengthened integration assertion:
    - `/api/v1/me` returns `Free` for `user@example.com`
    - `/api/v1/me` returns `Enterprise` for `cms.operator@example.com`
- affected repos/files/modules:
  - `cobo_iam_services/migrations/0011_user_subscription_tiers.up.sql`
  - `cobo_iam_services/migrations/0011_user_subscription_tiers.down.sql`
  - `cobo_iam_services/migrations/run_dev_migrations.sh`
  - `cobo_iam_services/migrations/seed_dev_identity_authorization.sql`
  - `cobo_iam_services/migrations/0009_seed_authz_test_accounts.up.sql`
  - `cobo_iam_services/internal/iam/infra/mysql/credentials.go`
  - `cobo_iam_services/internal/iam/infra/inmemory/credentials.go`
  - `cobo_iam_services/internal/httpserver/server.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
- important contracts/behaviors/constraints/decisions:
  - contract remains `user.subscription_tier` in `/api/v1/me` (backward-compatible field already introduced)
  - behavior is now source-of-truth DB-backed in MySQL mode, with fallback to `Free` only when entitlement row is absent
  - migration strategy is expand + backfill (safe for mixed existing DB states)
- build/verification result:
  - targeted integration test passed:
    - `go test ./internal/httpserver -run "TestIntegration_meProfileIncludesSubscriptionTier|TestIntegration_platformCMSPrefix_dashboardCollectionsEntries" -count=1`
  - full cycle + fresh docker rebuild passed:
    - `powershell -ExecutionPolicy Bypass -File docs/scripts/run-c1-cycle.ps1`
    - includes `docker compose -f docker-compose.dev.yml build --no-cache api web`
  - live DB smoke check:
    - `admin.web@example.com` => `Premium`
    - `cms.operator@example.com` => `Enterprise`
- remaining gaps/risks/next steps:
  - optional next step: expose admin API to manage tier lifecycle (`effective_from/effective_to`) instead of seed-only control

## 2026-04-28 - Week1/Week2 Review Sync

- task type: understand
- objective: verify whether Week 1 and Week 2 still have missing items after entitlement-source rollout
- what was discovered:
  - Week 1 code-level items are materially complete, including B3 contract and DB-backed tier source
  - remaining Week 1 items are mostly governance/process:
    - Q1 sign-off evidence execution
    - fuller B4 narrative doc for tenant-boundary explanation
  - Week 2 CMS scope is complete and expanded beyond baseline; no blocking code gap identified
  - remaining enhancements are optional operational maturity items (external metrics sink, tier admin lifecycle API)
- affected repos/files/modules:
  - `cobo_web_design/docs/week-1-foundation-status-and-plan.md`
  - `cobo_web_design/docs/cross_repo_step_by_step_implementation_plan.md`
  - `cobo_web_design/docs/ai-cache/reusable-task-updates.md`
  - `cobo_iam_services/docs/ai-cache/reusable-task-updates.md`
- important contracts/behaviors/constraints/decisions:
  - `subscription_tier` now resolves from `user_subscription_tiers` in MySQL path
  - strict CMS entry policy (`platform.cms.view`) remains unchanged
- build/verification result:
  - understand-only recheck; no new build/test command in this turn
  - latest green baseline remains full wrapper cycle + docker rebuild from previous turn
- remaining gaps/risks/next steps:
  - attach formal Q1 sign-off artifacts if week closure requires auditable acceptance
  - decide whether to include operational enhancements in Week 2 closure criteria

## 2026-04-28 - Week 3 Task Board Planning Sync

- task type: understand
- objective: align Week 3 execution board (C2/C3/start D1) with file/module-level tasks and dependency order
- what was discovered:
  - Week 3 delivery should follow dependency-first chain:
    - C2 BE disclosure-type/template contracts -> C2 FE wiring/tests
    - C3 BE history model -> C3 FE timeline wiring/tests
    - D1 BE deadline baseline -> D1 FE replacement for deadline list/detail
  - risk-managed order favors backward-compatible API additions before FE cutover
- affected repos/files/modules:
  - `cobo_web_design/docs/cross_repo_step_by_step_implementation_plan.md`
  - `cobo_web_design/docs/ai-cache/reusable-task-updates.md`
  - `cobo_iam_services/docs/ai-cache/reusable-task-updates.md`
- important contracts/behaviors/constraints/decisions:
  - keep current response envelope and strict permission model consistent across new Week 3 endpoints
  - require smoke + docker rebuild at end of each implementation slice
- build/verification result:
  - planning-only step; no new build/test run
- remaining gaps/risks/next steps:
  - optional next step: split the Week 3 board into assignable PR-sized batches (1-2 days each)

## 2026-04-28 - W3-C2-BE-01 + W3-C2-BE-04 Disclosure Catalog Read APIs

- task type: implement
- objective: implement contract-first backend read APIs for disclosure type groups/type details and add integration matrix coverage
- what was implemented/discovered:
  - added new disclosure catalog read endpoints:
    - `GET /api/v1/disclosure-groups`
    - `GET /api/v1/disclosure-types` (`group_id`, `q` filters)
    - `GET /api/v1/disclosure-types/{type_id}`
  - introduced typed catalog contracts (`DisclosureGroupDTO`, `DisclosureTypeSummaryDTO`, `DisclosureTypeDTO`) and read methods in disclosure service
  - seeded in-service baseline catalog dataset for periodic/event/custom type flows to unblock FE API cutover
  - added integration test:
    - `TestIntegration_disclosureTypeCatalog_contractAndAuth`
    - asserts success contract, filtered list, detail not-found (`404`), and forbidden access (`403`)
- affected repos/files/modules:
  - `cobo_iam_services/internal/disclosure/app/contracts.go`
  - `cobo_iam_services/internal/disclosure/app/catalog.go`
  - `cobo_iam_services/internal/disclosure/app/service.go`
  - `cobo_iam_services/internal/disclosure/transport/http/handler.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
- important contracts/behaviors/constraints/decisions:
  - contract-first shape is additive and backward-compatible with existing disclosure CRUD APIs
  - catalog endpoints currently authorize via `disclosure.create` policy path (same subject gate used by record creation) to avoid exposing catalog to users without disclosure workflow access
  - detail endpoint returns `404 invalid_request` with message `disclosure type not found` for unknown type id
- build/verification result:
  - targeted test passed:
    - `go test ./internal/disclosure/... ./internal/httpserver -run TestIntegration_disclosureTypeCatalog_contractAndAuth -count=1`
  - full cycle + fresh docker rebuild passed:
    - `powershell -ExecutionPolicy Bypass -File docs/scripts/run-c1-cycle.ps1`
    - includes `go test ./...`, FE lint/tests, DB smoke scripts, and `docker compose -f docker-compose.dev.yml build --no-cache api web`
- remaining gaps/risks/next steps:
  - current catalog source is static in-service seed data; next C2 steps should move to persistent store + admin upsert/versioning flow for template lifecycle

## 2026-04-28 - W3-C2 BE Persistent Catalog + Admin Version Lifecycle

- task type: implement
- objective: move disclosure type catalog from static in-service data to persistent MySQL store and add admin lifecycle APIs for upsert/versioning
- what was implemented/discovered:
  - added persistent schema + seed migration:
    - `disclosure_type_groups`
    - `disclosure_types` (active version pointer)
    - `disclosure_type_versions` (versioned payload + metadata)
  - wired disclosure catalog read APIs to repository-backed storage (in-memory + MySQL implementations)
  - added admin lifecycle endpoints:
    - `PUT /api/v1/admin/disclosure-types/{type_id}` (create new version and activate)
    - `GET /api/v1/admin/disclosure-types/{type_id}/versions` (version history, active flag)
  - added integration regression:
    - `TestIntegration_disclosureTypeCatalog_adminUpsertAndVersioning`
    - covers admin upsert success, active version bump, detail reflects active version, version listing, non-admin forbidden
- affected repos/files/modules:
  - `cobo_iam_services/migrations/0012_disclosure_catalog_versions.up.sql`
  - `cobo_iam_services/migrations/0012_disclosure_catalog_versions.down.sql`
  - `cobo_iam_services/migrations/run_dev_migrations.sh`
  - `cobo_iam_services/internal/disclosure/app/contracts.go`
  - `cobo_iam_services/internal/disclosure/app/catalog.go`
  - `cobo_iam_services/internal/disclosure/app/service.go`
  - `cobo_iam_services/internal/disclosure/transport/http/handler.go`
  - `cobo_iam_services/internal/disclosure/infra/inmemory/repository.go`
  - `cobo_iam_services/internal/disclosure/infra/mysql/repository.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
- important contracts/behaviors/constraints/decisions:
  - read contract remains backward-compatible; detail now includes `version_no`
  - admin version lifecycle is gated by `rbac.manage` permission check
  - upsert always creates a new immutable version row and moves `active_version_no` to latest
  - tenant scope enforced on company-owned types (`company_id`) while allowing global (`NULL`) catalog types
- build/verification result:
  - targeted tests passed:
    - `go test ./internal/disclosure/... ./internal/httpserver -run "TestIntegration_disclosureTypeCatalog_contractAndAuth|TestIntegration_disclosureTypeCatalog_adminUpsertAndVersioning" -count=1`
  - full cycle + fresh docker rebuild passed:
    - `powershell -ExecutionPolicy Bypass -File docs/scripts/run-c1-cycle.ps1`
    - includes `go test ./...`, FE lint/tests, DB smokes, and `docker compose -f docker-compose.dev.yml build --no-cache api web`
- remaining gaps/risks/next steps:
  - add explicit admin API to roll back/activate a previous version (current flow only appends new active version)
  - consider audit enrichments for version diff payload to improve compliance traceability

## 2026-04-28 - W3-C2 Admin Rollback/Activate Version Endpoint

- task type: implement
- objective: add explicit admin endpoint to activate an older disclosure type version (rollback) and extend integration test matrix
- what was implemented/discovered:
  - added admin lifecycle endpoint:
    - `POST /api/v1/admin/disclosure-types/{type_id}/activate`
  - activation flow now supports selecting existing `version_no` and updating active pointer
  - added full regression assertions in existing admin lifecycle test:
    - activate version 1 success
    - detail endpoint reflects rolled-back active version
    - invalid payload (`version_no <= 0`) returns `400`
    - unknown version returns `404`
    - non-admin activation returns `403`
  - both in-memory and MySQL disclosure repositories now implement activation behavior
- affected repos/files/modules:
  - `cobo_iam_services/internal/disclosure/app/contracts.go`
  - `cobo_iam_services/internal/disclosure/app/service.go`
  - `cobo_iam_services/internal/disclosure/transport/http/handler.go`
  - `cobo_iam_services/internal/disclosure/infra/inmemory/repository.go`
  - `cobo_iam_services/internal/disclosure/infra/mysql/repository.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
- important contracts/behaviors/constraints/decisions:
  - endpoint requires `rbac.manage` (same admin boundary as upsert)
  - activation switches `active_version_no` without mutating historical version payload
  - response returns active version metadata (`type_id`, `version_no`, `is_active`, `updated_by`, `activated_at`)
- build/verification result:
  - targeted tests passed:
    - `go test ./internal/disclosure/... ./internal/httpserver -run "TestIntegration_disclosureTypeCatalog_adminUpsertAndVersioning|TestIntegration_disclosureTypeCatalog_contractAndAuth" -count=1`
  - full cycle + fresh docker rebuild passed:
    - `powershell -ExecutionPolicy Bypass -File docs/scripts/run-c1-cycle.ps1`
    - includes `go test ./...`, FE lint/tests, DB smokes, and `docker compose -f docker-compose.dev.yml build --no-cache api web`
- remaining gaps/risks/next steps:
  - optional next step: add dedicated audit event payload for version activation reason and old/new version pair for compliance reporting

## 2026-04-28 - Disclosure Version Lifecycle Audit Enrichment

- task type: implement
- objective: add compliance-grade audit trail for disclosure type version upsert/activate operations with old/new version context
- what was implemented/discovered:
  - disclosure handler now appends audit events for admin lifecycle operations:
    - `disclosure.type.version.upsert`
    - `disclosure.type.version.activate`
  - audit metadata includes version transition context:
    - `old_version_no`
    - `new_version_no`
    - `change_note` (upsert) / `reason` (activate)
  - integration test now verifies both lifecycle actions are present in audit feed via `/api/v1/platform/cms/ops/audit`
- affected repos/files/modules:
  - `cobo_iam_services/internal/disclosure/transport/http/handler.go`
  - `cobo_iam_services/internal/httpserver/server.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
  - `cobo_iam_services/docs/ai-cache/reusable-task-updates.md`
- important contracts/behaviors/constraints/decisions:
  - no breaking change to disclosure API response contracts
  - audit emission is best-effort (`AppendAuditLog` non-blocking for response path)
  - lifecycle actions remain under `rbac.manage` boundary; audit acts as observability/compliance supplement
- build/verification result:
  - targeted test passed:
    - `go test ./internal/httpserver -run TestIntegration_disclosureTypeCatalog_adminUpsertAndVersioning -count=1`
  - full cycle + fresh docker rebuild passed:
    - `powershell -ExecutionPolicy Bypass -File docs/scripts/run-c1-cycle.ps1`
    - includes `go test ./...`, FE lint/tests, DB smokes, and `docker compose -f docker-compose.dev.yml build --no-cache api web`
- remaining gaps/risks/next steps:
  - optional next step: expose audit metadata fields in `/api/v1/platform/cms/ops/audit` response for richer forensic/debug UX

## 2026-04-28 - CMS Ops Audit Feed Metadata Exposure

- task type: implement
- objective: expose richer audit fields (including metadata) in `/api/v1/platform/cms/ops/audit` to support forensic/debug UX for disclosure version lifecycle
- what was implemented/discovered:
  - expanded audit feed item shape to include:
    - `resource_type`
    - `resource_id`
    - `decision`
    - `request_id`
    - `metadata`
  - kept existing fields unchanged (`event_id`, `action`, `actor`, `company_id`, `created_at`) for compatibility
  - extended integration test for disclosure version lifecycle to assert metadata payload presence and expected values:
    - upsert includes `old_version_no`/`new_version_no`
    - activate includes `reason = rollback to baseline`
- affected repos/files/modules:
  - `cobo_iam_services/internal/platformcms/transport/http/handler.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
  - `cobo_iam_services/docs/ai-cache/reusable-task-updates.md`
- important contracts/behaviors/constraints/decisions:
  - audit feed change is additive; existing consumers of prior keys remain valid
  - metadata exposure intentionally follows existing audit repository payload (no new schema write required)
- build/verification result:
  - targeted tests passed:
    - `go test ./internal/httpserver -run "TestIntegration_disclosureTypeCatalog_adminUpsertAndVersioning|TestIntegration_platformCMSPrefix_metricsAndAuditActionCatalog" -count=1`
  - full cycle + fresh docker rebuild passed:
    - `powershell -ExecutionPolicy Bypass -File docs/scripts/run-c1-cycle.ps1`
    - includes `go test ./...`, FE lint/tests, DB smokes, and `docker compose -f docker-compose.dev.yml build --no-cache api web`
- remaining gaps/risks/next steps:
  - optional next step: add API-level filter by `resource_type/resource_id` for faster audit investigation in large datasets

## 2026-04-28 - CMS Ops Audit Resource Filters

- task type: implement
- objective: support audit query filtering by `resource_type` and `resource_id` to improve investigation speed on large audit datasets
- what was implemented/discovered:
  - extended audit repository query contract to accept:
    - `resource_type`
    - `resource_id`
  - implemented filter behavior in both backends:
    - in-memory audit repository
    - MySQL audit repository SQL predicates
  - wired new query params in `/api/v1/platform/cms/ops/audit`
  - extended integration test to validate combined filter:
    - `action=disclosure.type.version.activate`
    - `resource_type=disclosure_type`
    - `resource_id=dt-custom-obligation`
  - updated known action allow-list so disclosure lifecycle actions remain valid for `action` filter
- affected repos/files/modules:
  - `cobo_iam_services/internal/audit/app/contracts.go`
  - `cobo_iam_services/internal/audit/infra/inmemory/repository.go`
  - `cobo_iam_services/internal/audit/infra/mysql/repository.go`
  - `cobo_iam_services/internal/platformcms/transport/http/handler.go`
  - `cobo_iam_services/internal/platformcms/transport/http/audit_actions.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
  - `cobo_iam_services/docs/ai-cache/reusable-task-updates.md`
- important contracts/behaviors/constraints/decisions:
  - change is additive; existing `action`/`limit` behavior remains unchanged
  - `action` still validated against known action catalog; resource filters are free-form exact match filters
  - MySQL query predicates keep empty-string bypass semantics for backward compatibility
- build/verification result:
  - targeted tests passed:
    - `go test ./internal/httpserver -run "TestIntegration_disclosureTypeCatalog_adminUpsertAndVersioning|TestIntegration_platformCMSPrefix_metricsAndAuditActionCatalog" -count=1`
  - full cycle + fresh docker rebuild passed:
    - `powershell -ExecutionPolicy Bypass -File docs/scripts/run-c1-cycle.ps1`
    - includes `go test ./...`, FE lint/tests, DB smokes, and `docker compose -f docker-compose.dev.yml build --no-cache api web`
- remaining gaps/risks/next steps:
  - optional next step: add time-range pagination (`from/to`, cursor) for long-horizon audit export/reporting

## 2026-04-28 - CMS Ops Audit Time Range + Cursor Pagination

- task type: implement
- objective: add long-horizon audit querying support via `from/to` time filters and cursor-style pagination for `/api/v1/platform/cms/ops/audit`
- what was implemented/discovered:
  - extended audit query contract to accept:
    - `from` (inclusive RFC3339)
    - `to` (inclusive RFC3339)
    - `cursor` (exclusive upper bound on `created_at` / `occurred_at`)
  - wired handler query parsing and forwarding to audit repositories
  - updated MySQL repository predicates for optional time window + cursor filters while preserving old empty-filter behavior
  - updated in-memory repository parity with RFC3339 parsing and equivalent filtering
  - response `meta` now includes pagination shape:
    - `total`
    - `limit`
    - `next_cursor` (derived from last returned item's `created_at` when page is full)
  - expanded integration test coverage under disclosure lifecycle flow to assert time-filter query success and `next_cursor` presence
- affected repos/files/modules:
  - `cobo_iam_services/internal/audit/app/contracts.go`
  - `cobo_iam_services/internal/audit/infra/inmemory/repository.go`
  - `cobo_iam_services/internal/audit/infra/mysql/repository.go`
  - `cobo_iam_services/internal/platformcms/transport/http/handler.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
- important contracts/behaviors/constraints/decisions:
  - change is additive and backward-compatible for clients not sending `from/to/cursor`
  - cursor semantics are descending-feed friendly: request next page with `cursor=<previous meta.next_cursor>`
  - invalid/missing timestamp values are treated as empty filters in repository layer (non-breaking)
- build/verification result:
  - targeted tests passed:
    - `go test ./internal/httpserver -run "TestIntegration_disclosureTypeCatalog_adminUpsertAndVersioning|TestIntegration_platformCMSPrefix_metricsAndAuditActionCatalog" -count=1`
  - full cycle + fresh docker rebuild passed:
    - `powershell -ExecutionPolicy Bypass -File docs/scripts/run-c1-cycle.ps1`
    - includes `go test ./...`, FE lint/tests, DB smokes, and `docker compose -f docker-compose.dev.yml build --no-cache api web`
- remaining gaps/risks/next steps:
  - optional next step: validate/normalize RFC3339 query params at handler layer and return explicit `400 invalid_request` for malformed values

## 2026-05-01 - P0-1 PR-A CMS Media Signed Upload Contract + Endpoints

- task type: implement
- objective: implement PR-A backend slice for Week 3 P0-1 with contract-first CMS media signed-upload flow and integration test matrix
- what was implemented/discovered:
  - added CMS media endpoints under platform prefix:
    - `POST /api/v1/platform/cms/media/upload` (issue upload intent)
    - `POST /api/v1/platform/cms/media/{asset_id}/complete` (finalize upload)
    - `GET /api/v1/platform/cms/media` (list/filter by `type`, `q`, `cursor`, `limit`)
    - `DELETE /api/v1/platform/cms/media/{asset_id}` (mark deleted)
  - implemented in-memory media asset store for PR-A contract validation:
    - state flow: `pending_upload` -> `ready` -> `deleted`
    - tenant/company scoping, MIME allow-list, size limit, cursor-based listing
  - added new CMS audit actions:
    - `cms.media.upload.intent`
    - `cms.media.upload.complete`
    - `cms.media.delete`
  - added integration test matrix:
    - happy path intent -> complete -> list -> delete
    - invalid content type (`400`)
    - oversize payload (`413`)
    - forbidden non-CMS user (`403`)
    - audit feed contains all media actions
- affected repos/files/modules:
  - `cobo_iam_services/internal/platformcms/transport/http/handler.go`
  - `cobo_iam_services/internal/platformcms/transport/http/media_handler.go`
  - `cobo_iam_services/internal/platformcms/transport/http/media_store.go`
  - `cobo_iam_services/internal/platformcms/transport/http/audit_actions.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
  - `cobo_iam_services/docs/ai-cache/reusable-task-updates.md`
- important contracts/behaviors/constraints/decisions:
  - PR-A uses signed-upload intent contract with stub signed URL (`upload.local`) to lock FE/BE contract before real object storage integration
  - permission boundary remains strict:
    - require `platform.cms.view`
    - plus operational manage permission (`rbac.manage` or `system.settings`)
  - contract is additive and does not break existing CMS prefix APIs
- build/verification result:
  - targeted test passed:
    - `go test ./internal/httpserver -run TestIntegration_platformCMSPrefix_mediaUploadContract -count=1`
  - full backend suite passed:
    - `go test ./...`
  - fresh no-cache docker rebuild passed:
    - `docker compose -f docker-compose.dev.yml build --no-cache api web`
  - note:
    - one `run-c1-cycle.ps1` attempt was interrupted/hung at docker fetch stage; required steps were rerun directly and passed
- remaining gaps/risks/next steps:
  - PR-B should wire real FE `CmsMediaPage` to these endpoints (intent/upload/complete/list/delete)
  - PR-C should replace stub signed URL generation with real object storage signer + persistent MySQL media repository

## 2026-05-01 - P0-1 PR-C Real Signed Upload + MySQL Media Repository

- task type: implement
- objective: complete production-ready signed upload middleware by replacing `upload.local` stub with real HMAC signed upload URLs, real binary storage, and persistent MySQL media repository
- what was implemented/discovered:
  - added real signed upload flow:
    - `POST /api/v1/platform/cms/media/upload` now returns signed URL pointing to API upload endpoint
    - new endpoint `PUT /api/v1/platform/cms/media/upload/{asset_id}` accepts binary payload with signature verification
    - `POST /api/v1/platform/cms/media/{asset_id}/complete` now requires binary presence before marking `ready`
  - implemented HMAC signer + expiry validation for upload URLs
  - implemented disk-backed media storage (write/exists/delete) under configurable storage directory
  - replaced in-memory-only media persistence with repository abstraction:
    - in-memory repo kept for no-DB mode/tests
    - MySQL repo added for durable metadata in DB mode
  - added migration for persistent media table:
    - `0013_cms_media_assets.up.sql`
    - `0013_cms_media_assets.down.sql`
    - wired into `migrations/run_dev_migrations.sh`
  - wired platform CMS handler with media options from process config (DB/signing secret/url ttl/storage dir)
  - updated integration media contract test to execute real signed binary `PUT` before `complete`
- affected repos/files/modules:
  - `cobo_iam_services/internal/platform/config/config.go`
  - `cobo_iam_services/internal/httpserver/server.go`
  - `cobo_iam_services/internal/platformcms/transport/http/handler.go`
  - `cobo_iam_services/internal/platformcms/transport/http/media_handler.go`
  - `cobo_iam_services/internal/platformcms/transport/http/media_store.go`
  - `cobo_iam_services/internal/platformcms/transport/http/media_security.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
  - `cobo_iam_services/migrations/0013_cms_media_assets.up.sql`
  - `cobo_iam_services/migrations/0013_cms_media_assets.down.sql`
  - `cobo_iam_services/migrations/run_dev_migrations.sh`
  - `cobo_iam_services/docs/ai-cache/reusable-task-updates.md`
- important contracts/behaviors/constraints/decisions:
  - upload signature binds `asset_id`, `company_id`, `method`, `content_type`, `size_bytes`, and `exp` to prevent replay/tampering
  - completion is blocked until binary file exists in storage path (`media binary not uploaded yet`)
  - DB mode uses persistent `cms_media_assets`; no-DB mode keeps in-memory parity for local tests
  - media config introduced:
    - `CMS_MEDIA_UPLOAD_SIGNING_SECRET`
    - `CMS_MEDIA_UPLOAD_URL_TTL`
    - `CMS_MEDIA_STORAGE_DIR`
- build/verification result:
  - targeted integration test passed:
    - `go test ./internal/httpserver -run TestIntegration_platformCMSPrefix_mediaUploadContract -count=1`
  - full backend tests passed:
    - `go test ./...`
  - fresh no-cache docker rebuild passed:
    - `docker compose -f docker-compose.dev.yml build --no-cache api web`
  - containerized migration run verified by compose startup:
    - `docker compose -f docker-compose.dev.yml up -d api web` (`migrate` exited `0`)
- remaining gaps/risks/next steps:
  - optional next step: add dedicated DB-mode media smoke script covering full intent/upload/complete/list/delete against running compose stack
  - optional hardening: switch disk storage to external object store adapter (S3/GCS) using same signer/repository contract

## 2026-05-01 - P0-1 PR-C Follow-up: Media DB Smoke Script + Plan Sync Support

- task type: implement
- objective: add explicit DB smoke evidence for CMS media flow and support QA closure workflow for signed upload middleware
- what was implemented/discovered:
  - added new smoke script:
    - `docs/scripts/cms-media-db-smoke.ps1`
    - validates end-to-end flow: intent -> signed binary PUT -> complete -> list/filter -> delete -> audit check
  - integrated smoke script into full cycle runner:
    - `docs/scripts/run-c1-cycle.ps1`
    - new step `6/8 DB-mode CMS media smoke`
  - executed smoke directly against running API and observed pass:
    - `CMS MEDIA DB-MODE SMOKE PASSED`
- affected repos/files/modules:
  - `cobo_iam_services/docs/scripts/cms-media-db-smoke.ps1`
  - `cobo_iam_services/docs/scripts/run-c1-cycle.ps1`
  - `cobo_iam_services/docs/ai-cache/reusable-task-updates.md`
- important contracts/behaviors/constraints/decisions:
  - smoke verifies signed-upload contract exactly as FE/BE flow uses it, including binary upload with signed query params
  - audit evidence check is included for `cms.media.upload.complete` on uploaded asset resource
- build/verification result:
  - media smoke passed:
    - `powershell -ExecutionPolicy Bypass -File docs/scripts/cms-media-db-smoke.ps1 ...`
  - backend tests passed:
    - `go test ./...`
  - fresh no-cache docker rebuild passed:
    - `docker compose -f docker-compose.dev.yml build --no-cache api web`
- remaining gaps/risks/next steps:
  - optional next step: wire this new smoke script into CI pipeline stage for automated PR evidence

## 2026-05-01 - Media Signed URL Public Host Override

- task type: implement
- objective: prevent CMS media signed upload intent from returning internal docker host (`api:8080`) by using configurable public API base URL
- what was implemented/discovered:
  - added `PUBLIC_API_BASE_URL` config and wired it into platform CMS media handler options
  - updated signed upload URL generation to prefer configured public base URL, with fallback to request-derived host only when config is empty
  - updated dev compose API env to set `PUBLIC_API_BASE_URL=http://localhost:8080`
- affected repos/files/modules:
  - `cobo_iam_services/internal/platform/config/config.go`
  - `cobo_iam_services/internal/httpserver/server.go`
  - `cobo_iam_services/internal/platformcms/transport/http/handler.go`
  - `cobo_iam_services/internal/platformcms/transport/http/media_handler.go`
  - `cobo_iam_services/docker-compose.dev.yml`
  - `cobo_iam_services/docs/ai-cache/reusable-task-updates.md`
- important contracts/behaviors/constraints/decisions:
  - upload intent URL host is now environment-controlled for dev/staging/public ingress compatibility
  - explicit config wins over `X-Forwarded-*` and request `Host` to avoid proxy/network topology leakage into client URLs
  - backward-safe fallback remains when `PUBLIC_API_BASE_URL` is intentionally unset
- build/verification result:
  - integration test passed:
    - `go test ./internal/httpserver -run TestIntegration_platformCMSPrefix_mediaUploadContract -count=1`
  - DB smoke passed:
    - `powershell -ExecutionPolicy Bypass -File docs/scripts/cms-media-db-smoke.ps1 ...`
  - full backend tests passed:
    - `go test ./...`
  - fresh no-cache docker rebuild passed for affected service:
    - `docker compose -f docker-compose.dev.yml build --no-cache api`
- remaining gaps/risks/next steps:
  - staging/prod must set `PUBLIC_API_BASE_URL` to ingress domain (e.g. `https://iam.<env-domain>`) to avoid mixed host issues

## 2026-05-01 - Disclosure Template Type Analysis Baseline (Periodic/Irregular/Custom)

- task type: understand
- objective: analyze backend-supported disclosure template creation/lifecycle and extract type-specific standardization requirements for CMS operations
- what was discovered:
  - disclosure type model already supports versioned, persistent template lifecycle with activation rollback
  - schema fields are broad enough for compliance-grade templates (`legal_basis`, `applicability`, `implementation_*`, `report_content`, `required_docs`, `deadline_rule`, `periodicity`, channels/authorities/beneficiaries, risks, tags)
  - service-level validation for upsert is currently minimal (`type_id`, `group_id`, `name`) and does not yet enforce type-specific rules for periodic/irregular/custom
  - activation flow is robust for rollback (`active_version_no` pointer swap), but not tied to semantic validation of template consistency by type
- affected repos/files/modules:
  - `cobo_iam_services/internal/disclosure/app/contracts.go`
  - `cobo_iam_services/internal/disclosure/app/service.go`
  - `cobo_iam_services/internal/disclosure/infra/mysql/repository.go`
  - `cobo_iam_services/internal/disclosure/transport/http/handler.go`
  - `cobo_iam_services/migrations/0012_disclosure_catalog_versions.up.sql`
- important contracts/behaviors/constraints/decisions:
  - current contract allows flexible free-text content; this is good for adoption speed but increases template drift risk across operators
  - recommended backend rule baseline for Ã¢â‚¬Å“template chuÃ¡ÂºÂ©nÃ¢â‚¬Â:
    - periodic: require non-empty `periodicity` and normative `deadline_rule`
    - irregular: require event-based deadline semantics and trigger-condition content
    - custom: require governance/change-note context and explicit applicability scope
  - preserve backward compatibility by adding validation as additive constraints (with clear `400 invalid_request` details), not by contract-breaking field removals
- build/verification result:
  - analysis-only task; no code changes and no build/test run required
- remaining gaps/risks/next steps:
  - implement type-aware validation matrix in service layer and cover with HTTP integration tests
  - optionally introduce structured enums for `template_category`/`periodicity` to reduce free-text ambiguity
  - align CMS FE form constraints with backend validation to avoid user-facing mismatch/retry loops

## 2026-05-01 - Add 3 Disclosure Template Types (Obligation/AGM/Transaction)

- task type: implement
- objective: bÃ¡Â»â€¢ sung thÃƒÂªm 3 loÃ¡ÂºÂ¡i template bÃƒÂ¡o cÃƒÂ¡o trong disclosure catalog Ã„â€˜Ã¡Â»Æ’ CMS cÃƒÂ³ thÃ¡Â»Æ’ tÃ¡ÂºÂ¡o/chÃ¡Â»Ân ngay: nghÃ„Â©a vÃ¡Â»Â¥, tÃ¡Â»â€¢ chÃ¡Â»Â©c Ã„â€˜Ã¡ÂºÂ¡i hÃ¡Â»â„¢i cÃ¡Â»â€¢ Ã„â€˜ÃƒÂ´ng, cÃƒÂ´ng bÃ¡Â»â€˜ thÃƒÂ´ng tin vÃ¡Â»Â giao dÃ¡Â»â€¹ch
- what was implemented/discovered:
  - added new migration `0014_disclosure_catalog_extra_types.up.sql` to insert/upsert:
    - `dt-obligation-report` (group `group-001`)
    - `dt-shareholder-meeting` (group `group-001`)
    - `dt-disclosure-transaction` (group `group-002`)
  - added rollback migration `0014_disclosure_catalog_extra_types.down.sql`
  - updated migration runner order in `run_dev_migrations.sh` to include `0014_disclosure_catalog_extra_types.up.sql`
  - synced in-memory catalog seed (`internal/disclosure/app/catalog.go`) with the same 3 new types for DB/non-DB parity in tests/dev
- affected repos/files/modules:
  - `cobo_iam_services/migrations/0014_disclosure_catalog_extra_types.up.sql`
  - `cobo_iam_services/migrations/0014_disclosure_catalog_extra_types.down.sql`
  - `cobo_iam_services/migrations/run_dev_migrations.sh`
  - `cobo_iam_services/internal/disclosure/app/catalog.go`
- important contracts/behaviors/constraints/decisions:
  - rollout-safe approach: add new migration instead of modifying historical migration to avoid drift on already-migrated environments
  - category/group mapping:
    - nghÃ„Â©a vÃ¡Â»Â¥: `Ã„ÂÃ¡Â»â€¹nh kÃ¡Â»Â³` / `group-001`
    - tÃ¡Â»â€¢ chÃ¡Â»Â©c Ã„â€˜Ã¡ÂºÂ¡i hÃ¡Â»â„¢i cÃ¡Â»â€¢ Ã„â€˜ÃƒÂ´ng: `Ã„ÂÃ¡Â»â€¹nh kÃ¡Â»Â³` / `group-001`
    - cÃƒÂ´ng bÃ¡Â»â€˜ thÃƒÂ´ng tin vÃ¡Â»Â giao dÃ¡Â»â€¹ch: `BÃ¡ÂºÂ¥t thÃ†Â°Ã¡Â»Âng` / `group-002`
  - seed is idempotent via `ON DUPLICATE KEY UPDATE`
- build/verification result:
  - targeted integration tests passed:
    - `go test ./internal/disclosure/... ./internal/httpserver -run "TestIntegration_disclosureTypeCatalog_contractAndAuth|TestIntegration_disclosureTypeCatalog_adminUpsertAndVersioning" -count=1`
  - lints/diagnostics: no issues on changed files
- remaining gaps/risks/next steps:
  - run compose migration on active environment (`run_dev_migrations.sh` via stack startup) to materialize new types in DB
  - optional next step: prefill FE `CmsTemplatesPage` quick-select presets for these 3 new types to speed operator setup

## 2026-05-01 - Template Standardization: Type-aware Validation + Enum Contract

- task type: implement
- objective: chuÃ¡ÂºÂ©n hÃƒÂ³a lifecycle template disclosure bÃ¡ÂºÂ±ng matrix validation theo loÃ¡ÂºÂ¡i (periodic/irregular/custom), enum hÃƒÂ³a field reference vÃƒÂ  bÃ¡Â»â€¢ sung test matrix integration
- what was implemented/discovered:
  - applied running-stack migrations and verified catalog data:
    - `0014_disclosure_catalog_extra_types.up.sql` (3 loÃ¡ÂºÂ¡i mÃ¡Â»â€ºi)
    - `0015_disclosure_template_enums.up.sql` (chuÃ¡ÂºÂ©n hÃƒÂ³a enum + thÃƒÂªm `deadline_strategy`)
  - added backend template enum/validation module:
    - new `internal/disclosure/app/template_validation.go`
    - validates matrix by `template_category` and returns `400 INVALID_REQUEST` with `details.field_errors`
  - extended disclosure contracts with `deadline_strategy`
  - wired service validation in `UpsertTypeVersion` before repository persistence
  - updated MySQL + in-memory repositories to store/read `deadline_strategy`
  - synced seed catalog constants to canonical enums (`periodic/irregular/custom`, `monthly/...`, strategy values)
  - expanded integration test matrix in `TestIntegration_disclosureTypeCatalog_adminUpsertAndVersioning`:
    - happy path upsert + version list + rollback activate
    - invalid-by-type matrix for periodic/irregular/custom expecting `400`
    - forbidden/non-admin checks remain covered
- affected repos/files/modules:
  - `cobo_iam_services/internal/disclosure/app/contracts.go`
  - `cobo_iam_services/internal/disclosure/app/service.go`
  - `cobo_iam_services/internal/disclosure/app/template_validation.go`
  - `cobo_iam_services/internal/disclosure/app/catalog.go`
  - `cobo_iam_services/internal/disclosure/infra/mysql/repository.go`
  - `cobo_iam_services/internal/disclosure/infra/inmemory/repository.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
  - `cobo_iam_services/migrations/0014_disclosure_catalog_extra_types.*.sql`
  - `cobo_iam_services/migrations/0015_disclosure_template_enums.*.sql`
  - `cobo_iam_services/migrations/run_dev_migrations.sh`
- important contracts/behaviors/constraints/decisions:
  - canonical enum contract:
    - `template_category`: `periodic | irregular | custom`
    - `periodicity`: `monthly | quarterly | yearly | event_based | ad_hoc`
    - `deadline_strategy`: `fixed_cycle_days | event_relative_hours | configurable`
  - matrix rules:
    - periodic -> periodicity in `[monthly, quarterly, yearly]` + strategy `fixed_cycle_days`
    - irregular -> periodicity `event_based` + strategy `event_relative_hours`
    - custom -> periodicity in `[monthly, quarterly, yearly, ad_hoc]` + strategy `configurable`
  - legacy VN values are normalized to canonical enums via alias mapping + migration rewrite
- build/verification result:
  - migration apply on running stack passed:
    - `docker compose -f docker-compose.dev.yml run --rm migrate`
  - targeted integration matrix passed:
    - `go test ./internal/disclosure/... ./internal/httpserver -run "TestIntegration_disclosureTypeCatalog_contractAndAuth|TestIntegration_disclosureTypeCatalog_adminUpsertAndVersioning" -count=1`
  - fresh docker rebuild passed:
    - `docker compose -f docker-compose.dev.yml build --no-cache api`
    - `docker compose -f docker-compose.dev.yml up -d api`
  - API smoke confirms invalid-by-type returns field-specific details:
    - `INVALID_REQUEST` + `details.field_errors.periodicity/deadline_strategy`
- remaining gaps/risks/next steps:
  - optional: expose enum/reference-data endpoint from BE instead of FE hardcode to avoid future drift
  - optional: add stricter DB constraints/checks when enum set is fully stabilized across environments

## 2026-05-01 - Disclosure Template Reference-Data Endpoint

- task type: implement
- objective: add backend reference-data endpoint for disclosure template enums/rules to prevent FE hardcoded drift over time
- what was implemented/discovered:
  - added admin endpoint:
    - `GET /api/v1/admin/disclosure-types/reference-data`
  - endpoint returns canonical template enum catalog + validation rule hints:
    - `template_categories`
    - `periodicities`
    - `deadline_strategies`
    - `matrix_rules`
  - service method added to disclosure app layer with `rbac.manage` permission boundary
  - integration test matrix expanded to cover:
    - admin success response shape
    - non-admin forbidden (`403`)
- affected repos/files/modules:
  - `cobo_iam_services/internal/disclosure/app/contracts.go`
  - `cobo_iam_services/internal/disclosure/app/service.go`
  - `cobo_iam_services/internal/disclosure/transport/http/handler.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
- important contracts/behaviors/constraints/decisions:
  - response is additive and backward-compatible with existing admin lifecycle APIs
  - enum values are sourced from backend canonical constants used by validation matrix to keep FE/BE consistent
  - endpoint keeps admin-only boundary (`rbac.manage`) similar to other template lifecycle admin APIs
- build/verification result:
  - targeted backend integration test passed:
    - `go test ./internal/httpserver -run TestIntegration_disclosureTypeCatalog_adminUpsertAndVersioning -count=1`
  - fresh docker rebuild/restart passed for affected services:
    - `docker compose -f docker-compose.dev.yml build --no-cache api web`
    - `docker compose -f docker-compose.dev.yml up -d api web`
- remaining gaps/risks/next steps:
  - optional next step: expose localized labels per enum from backend when FE needs backend-driven display text (not only canonical codes)

## 2026-05-04 - Week 3 Closure Evidence Run (`-Phase all`)

- task type: implement
- objective: execute full week-3 closure evidence in one pass using phase gate runner and capture pass/fail output across baseline, regression, and seed/account verification gates
- what was implemented/discovered:
  - executed:
    - `powershell -ExecutionPolicy Bypass -File docs/scripts/run-week3-phase-gate.ps1 -Phase all`
  - first run failed at FE lint due missing VI i18n keys in `cobo_web_design/src/features/cms-core/language.tsx`:
    - `templates.blocks.error.mandatoryBlocksMissingPrefix`
    - `templates.blocks.error.mandatoryBlocksExtraAllowed`
  - added the missing VI translations in FE repo and reran the full phase gate successfully
  - successful rerun covered:
    - phase 1 (`run-c1-cycle.ps1`): BE tests, FE lint/tests, DB smokes (disclosure/cms/media), no-cache docker rebuild + restart
    - phase 2: full BE regression + platform CMS integration + FE lint/test/build
    - phase 3: seed/account verification (`verify-seed-accounts.ps1`)
- affected repos/files/modules:
  - `cobo_iam_services/docs/scripts/run-week3-phase-gate.ps1`
  - `cobo_iam_services/docs/scripts/verify-seed-accounts.ps1`
  - `cobo_web_design/src/features/cms-core/language.tsx`
  - `cobo_iam_services/docs/ai-cache/reusable-task-updates.md`
- important contracts/behaviors/constraints/decisions:
  - phase gate remains contract-preserving; changes were evidence/verification oriented except FE translation completeness fix
  - docker compose emits status lines to stderr on Windows PowerShell but run completed with exit code `0`
- build/verification result:
  - full phase gate passed:
    - terminal footer: `WEEK 3 PHASE GATE PASSED`
    - command exit: `0` (`elapsed_ms: 121875`)
  - key checkpoints:
    - FE targeted regression: `45 passed`
    - FE production build: success (with non-blocking chunk-size warning)
    - DB smokes: disclosure/CMS prefix/CMS media all passed
    - seed/account verification: passed
- remaining gaps/risks/next steps:
  - optional: suppress or track Vite chunk-size warning via code-splitting/manual chunks if performance budget requires it
  - optional: publish this `-Phase all` run output as release artifact/CI evidence for formal week-3 signoff

## 2026-05-04 - Week 3 Phase Gate Scripts (Regression + Seed Verification)

- task type: implement
- objective: implement phased execution gates to close remaining Week 3 operational gaps with runnable scripts (`full regression` + `devops seed/account verification`)
- what was implemented/discovered:
  - added new phase orchestrator script:
    - `docs/scripts/run-week3-phase-gate.ps1`
    - supports `-Phase 1|2|3|all`
    - phase 1: delegates to `run-c1-cycle.ps1` (existing C2/CMS baseline + docker parity cycle)
    - phase 2: runs full BE+FE regression gate (`go test ./...`, platform CMS integration tests, FE lint + focused regression tests + FE build)
    - phase 3: brings compose services up and executes seed/account verification gate
  - added new seed/account verification script:
    - `docs/scripts/verify-seed-accounts.ps1`
    - validates default seeded user/admin/cms logins, membership/select-company flow, `/me` identity consistency, CMS collections access for cms operator, and baseline disclosure list for normal user
  - fixed initial phase-3 script assumptions to match live API shape:
    - replaced non-existing endpoint probes (`/api/v1/platform/cms/dashboard`, `/api/v1/companies`) with existing endpoints used by current stack contracts
- affected repos/files/modules:
  - `cobo_iam_services/docs/scripts/run-week3-phase-gate.ps1`
  - `cobo_iam_services/docs/scripts/verify-seed-accounts.ps1`
  - `cobo_iam_services/docs/ai-cache/reusable-task-updates.md`
- important contracts/behaviors/constraints/decisions:
  - phase gate design is additive and does not alter runtime API behavior; it standardizes release-readiness evidence collection
  - phase 3 verifies seeded account operability using real auth/session/company selection contracts instead of static fixture assumptions
  - scripts keep compatibility with current local compose defaults and existing smoke credentials
- build/verification result:
  - phase-3 execution passed end-to-end:
    - `powershell -ExecutionPolicy Bypass -File docs/scripts/run-week3-phase-gate.ps1 -Phase 3`
    - result includes successful `SEED ACCOUNT VERIFICATION PASSED` and `WEEK 3 PHASE GATE PASSED`
  - noted runtime noise:
    - `docker compose up` prints status lines to stderr in PowerShell, but command exited `0` and flow succeeded
- remaining gaps/risks/next steps:
  - run `-Phase all` on release branch to generate complete Week 3 closure evidence (phase 1 + 2 + 3)
  - optional hardening: migrate script password params to `SecureString`/`PSCredential` if strict PSScriptAnalyzer policy is required

## 2026-05-04 - Disclosure Template Contract: add `reminder_milestones` (backward-compatible)

- task type: implement
- objective: add backend support for `reminder_milestones` in disclosure template upsert/detail APIs so FE can render milestone chips from API data instead of static fallback
- what was implemented/discovered:
  - contract update in `internal/disclosure/app/contracts.go`:
    - added `ReminderMilestones []string 'json:"reminder_milestones"'` to:
      - `UpsertTypeVersionRequest`
      - `DisclosureTypeDTO`
  - service-layer normalization in `internal/disclosure/app/service.go`:
    - sanitize and dedupe `reminder_milestones` entries (`trim`, remove empty, stable order)
  - MySQL repository update in `internal/disclosure/infra/mysql/repository.go`:
    - read/write `reminder_milestones_json` in detail/version queries and upsert insert
    - decode JSON array safely with fallback `[]` for null/empty payloads
  - in-memory repository parity in `internal/disclosure/infra/inmemory/repository.go`:
    - include `ReminderMilestones` in upsert snapshot and clone on reads
  - migration added:
    - `migrations/0018_disclosure_reminder_milestones.up.sql`
    - `migrations/0018_disclosure_reminder_milestones.down.sql`
    - schema delta: add nullable JSON column `reminder_milestones_json` to `disclosure_type_versions`
  - integration regression expanded in `internal/httpserver/server_test.go`:
    - admin upsert payload now includes `reminder_milestones`
    - asserts detail/version endpoints return the same milestones
- affected repos/files/modules:
  - `cobo_iam_services/internal/disclosure/app/contracts.go`
  - `cobo_iam_services/internal/disclosure/app/service.go`
  - `cobo_iam_services/internal/disclosure/infra/mysql/repository.go`
  - `cobo_iam_services/internal/disclosure/infra/inmemory/repository.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
  - `cobo_iam_services/migrations/0018_disclosure_reminder_milestones.up.sql`
  - `cobo_iam_services/migrations/0018_disclosure_reminder_milestones.down.sql`
- important contracts/behaviors/constraints/decisions:
  - backward-compatible expand change:
    - new DB column is nullable
    - old clients (without `reminder_milestones`) continue to work unchanged
    - API response now carries milestones as `[]` when null/absent
  - mixed-version safe:
    - new code tolerates null column values via decode fallback
    - old code ignores extra JSON field from clients
- build/verification result:
  - `go test ./internal/disclosure/...`: pass
  - `go test ./internal/httpserver -run TestIntegration_disclosureTypeCatalog_adminUpsertAndVersioning -count=1`: pass
  - `docker compose -f docker-compose.dev.yml build api`: pass
- remaining gaps/risks/next steps:
  - FE normalizer in `cobo_web_design` still needs to map `reminder_milestones` -> `type.reminderMilestones` for end-to-end visual parity

## 2026-05-06 - Permission split for template workflow override (`template.workflow.override.*`)

- task type: implement
- objective: tach quyen rieng cho company-level workflow override thay vi dung chung disclosure action gate
- what was implemented/discovered:
  - disclosure service authorization for override endpoints now uses dedicated actions:
    - `template.workflow.override.read`
    - `template.workflow.override.write`
    - `template.workflow.override.approve`
    - `template.workflow.override.reset`
  - updated in-memory authorization fixture/policy mapping:
    - granted new permission codes to admin fixtures (`m_admin_001`, `m_cms_001`)
    - added action policies for four new actions -> matching required permissions
  - updated MySQL authorization legacy fallback policy:
    - map the four new actions to corresponding `template.workflow.override.*` permissions when `action_policy_matrix` is unavailable/unseeded
  - added migration for explicit permission records + default seed role grants:
    - `migrations/0021_template_workflow_override_permissions.up.sql`
    - `migrations/0021_template_workflow_override_permissions.down.sql`
    - up migration inserts 4 new permissions and grants them to roles `admin_web`, `cms_operator`, `admin_doanh_nghiep` in `c_001`
  - added migration to dev runner list:
    - `migrations/run_dev_migrations.sh` includes `0021_template_workflow_override_permissions.up.sql`
  - updated API contract note:
    - `docs/api-contracts-json.md` now documents required permission split by endpoint group
- affected repos/files/modules:
  - `cobo_iam_services/internal/disclosure/app/service.go`
  - `cobo_iam_services/internal/authorization/infra/inmemory/repository.go`
  - `cobo_iam_services/internal/authorization/infra/mysql/repository.go`
  - `cobo_iam_services/migrations/0021_template_workflow_override_permissions.up.sql`
  - `cobo_iam_services/migrations/0021_template_workflow_override_permissions.down.sql`
  - `cobo_iam_services/migrations/run_dev_migrations.sh`
  - `cobo_iam_services/docs/api-contracts-json.md`
- important contracts/behaviors/constraints/decisions:
  - override CRUD/approve/reset permissions are now independently governable from disclosure record permissions
  - read operations for workflow override/effective workflow require explicit `template.workflow.override.read`
  - migration keeps behavior backward-safe for running environments by adding new permissions without changing existing permission ids/codes
- build/verification result:
  - `go test ./internal/disclosure/... ./internal/authorization/...`: pass
  - `go test ./internal/httpserver -count=1`: pass
  - `docker compose -f docker-compose.dev.yml build api`: pass
- remaining gaps/risks/next steps:
  - if deployment environment uses customized role sets beyond seed roles, ops must assign new `template.workflow.override.*` permissions to intended roles before rollout

## 2026-05-20 - Feature: KiÃ¡Â»Æ’m soÃƒÂ¡t quy trÃƒÂ¬nh (Process Controller) for Ad-Hoc Alert flow

- task type: feature-extension (cross-repo)
- objective: replace permission-based admin approval gate (`ad_hoc_alert.admin_review`) in ad-hoc alert flow with identity-based process controller assigned by the proposal creator
- affected repos/files/modules:
  - **cobo_iam_services**:
    - `migrations/0042_adhoc_process_controller.up.sql` / `.down.sql` Ã¢â‚¬â€ nullable `process_controller_id` column + index
    - `migrations/0033_smoke_workflow_dev_seed.up.sql` Ã¢â‚¬â€ seed new permission `ad_hoc_alert.process_control`
    - `migrations/0034_seed_org_structure_demo.up.sql` Ã¢â‚¬â€ assign permission to `admin_doanh_nghiep` + `truong_phong` roles
    - `internal/adhoc/app/contracts.go` Ã¢â‚¬â€ `EligibleController`, `MembershipValidator` interface, `ListEligibleControllers`, `ProcessControllerID` on DTO
    - `internal/adhoc/infra/mysql/membership_validator.go` Ã¢â‚¬â€ NEW: SQL implementation of MembershipValidator
    - `internal/adhoc/infra/mysql/repository.go` Ã¢â‚¬â€ scan/insert `process_controller_id` in all 4 repo methods
    - `internal/adhoc/app/service.go` Ã¢â‚¬â€ validate controller at CreateProposal; identity gate at AdminApprove + Reject(admin stage); `ListEligibleControllers` method
    - `internal/adhoc/transport/http/handler.go` Ã¢â‚¬â€ NEW route `GET /api/v1/company/ad-hoc-proposals/eligible-controllers` (registered BEFORE wildcard `{proposal_id}`)
    - `internal/httpserver/server.go` Ã¢â‚¬â€ wire `MembershipValidator` into `adhocapp.NewService`
    - `internal/adhoc/app/service_test.go` Ã¢â‚¬â€ updated 4 existing tests + 6 new process controller tests
    - `internal/adhoc/transport/http/handler_test.go` Ã¢â‚¬â€ added `ListEligibleControllers` stub to fakeService
  - **cobo_web_design**:
    - `src/types.ts` Ã¢â‚¬â€ `EligibleProcessController` interface; `process_controller_id` on `AdHocProposalDto`; `ad_hoc_alert.process_control` permission
    - `src/services/adHocAlertsApi.ts` Ã¢â‚¬â€ `listEligibleControllers()` method; `process_controller_membership_id` required field
    - `src/services/workflowOverrideMappers.ts` Ã¢â‚¬â€ map `process_controller_id` in normalizer
    - `src/pages/portal/AdHocProposalCreatePage.tsx` Ã¢â‚¬â€ required process controller combobox (eligible-controllers fetch + dropdown)
    - `src/pages/portal/AdHocProposalDetailPage.tsx` Ã¢â‚¬â€ `canAdminApprove` now identity-based; button text "PhÃƒÂª duyÃ¡Â»â€¡t"; status "ChÃ¡Â»Â KiÃ¡Â»Æ’m soÃƒÂ¡t duyÃ¡Â»â€¡t"
    - `src/pages/portal/AdHocProposalListPage.tsx` Ã¢â‚¬â€ status label updated
    - `src/services/workflowServices.contract.test.ts` Ã¢â‚¬â€ 2 new contract tests
    - `src/pages/portal/portal.workflow.regression.test.tsx` Ã¢â‚¬â€ fixed 4 tests + 1 new identity gate test
  - `docs/canh-bao-bat-thuong-feature-doc.md` Ã¢â‚¬â€ updated to reflect Process Controller role
  - `docs/permission_catalog.md` Ã¢â‚¬â€ added `ad_hoc_alert.process_control`; marked `ad_hoc_alert.admin_review` deprecated
- contracts/decisions (ADRs):
  - **ADR-1**: `process_controller_membership_id` required at create time; validated: not empty, not self, active membership, has `ad_hoc_alert.process_control`
  - **ADR-2**: `AdminApprove` + `Reject(admin stage)` gate changed from permission check to identity check: `cur.ProcessControllerID != req.Subject.MembershipID Ã¢â€ â€™ 403`
  - **ADR-3**: `GET /api/v1/company/ad-hoc-proposals/eligible-controllers` added to adhoc module (NOT admin module) to avoid `admin.membership.list` permission requirement
  - **ADR-4**: `MembershipValidator` interface defined in `adhoc/app/contracts.go`, implemented in `adhoc/infra/mysql` to prevent circular imports with `authorization` module
  - **ADR-5**: Migration adds `process_controller_id` as NULL for zero-downtime rollout; application layer enforces non-null
- build/verification result:
  - `go test ./...`: pass (all packages)
  - `npm run lint` (tsc --noEmit): pass
  - `npx vitest run src/pages/portal/portal.workflow.regression.test.tsx`: 19 tests pass
  - `npx vitest run src/services/workflowServices.contract.test.ts`: all pass
- remaining gaps/risks/next steps:
  - **Reassign controller**: no endpoint to change `process_controller_id` after creation Ã¢â‚¬â€ only cancellation + re-creation workaround
  - **`ad_hoc_alert.admin_review` permission**: deprecated but not removed from DB; old admin users with this permission can no longer approve unless also designated as controller
  - ops must run migration 0042 before deploying; nullable column ensures safe rollout

## 2026-05-20 - Bug fix: ad-hoc alert create page denied for valid proposer

- task type: bug-fix
- objective/question:
  - investigate why `admin.dn@example.com` received access denied after clicking `TÃ¡ÂºÂ¡o cÃ¡ÂºÂ£nh bÃƒÂ¡o bÃ¡ÂºÂ¥t thÃ†Â°Ã¡Â»Âng mÃ¡Â»â€ºi` despite having `ad_hoc_alert.propose`
- implemented/discovered:
  - confirmed DB effective permissions for membership `m_102` include `ad_hoc_alert.propose`
  - confirmed `/api/v1/me/effective-access` returns `ad_hoc_alert.propose` for a fresh `admin.dn@example.com` session
  - traced the failure to `GET /api/v1/disclosure-types/{type_id}/effective-workflow`, which returned `403 access denied`
  - root cause: `internal/disclosure/app/service.go` gated `GetEffectiveWorkflow` with `template.workflow.override.read`, which is stricter than the surrounding disclosure catalog / ad-hoc create flow
  - fixed `GetEffectiveWorkflow` to reuse `requireDisclosureCatalogRead`, matching `listTypes` and `getTypeDetail`
  - added regression assertions in `internal/httpserver/server_test.go`:
    - authorized catalog user can fetch effective workflow
    - forbidden user still receives `403`
- affected repos/files/modules:
  - `cobo_iam_services/internal/disclosure/app/service.go`
  - `cobo_iam_services/internal/httpserver/server_test.go`
- contracts/behaviors/constraints/decisions:
  - read access to effective workflow now follows disclosure catalog read boundary, not workflow override admin-read boundary
  - this preserves denial for users who cannot read the disclosure catalog at all
  - `template.workflow.override.read` remains in place for company workflow override management endpoints
- build/verification result:
  - `go test ./internal/httpserver -run TestIntegration_disclosureTypeCatalog_contractAndAuth`: pass
  - `docker compose -f docker-compose.dev.yml build api`: pass
  - `docker compose -f docker-compose.dev.yml up -d --force-recreate api`: pass
  - live probe after restart:
    - login `admin.dn@example.com` succeeded
    - `GET /api/v1/disclosure-types/dt-disclosure-transaction/effective-workflow` => `200`
- remaining gaps/risks/next steps:
  - if the browser still holds an older session/app state, user should refresh and retry after the recreated API is running

## 2026-05-20 - Review/doc: current-state audit for ad-hoc alert CRUD/proposal flow

- task type: review-and-documentation (cross-repo)
- objective/question:
  - audit the latest real logic for ad-hoc alert CRUD / related workflow
  - document what is actually implemented, what is missing, current issues/conflicts, and business-readable contract notes for BA/marketing
- implemented/discovered:
  - created business + engineering summary doc:
    - `docs/ai-cache/adhoc-alert-crud-current-state-business-audit-summary.md`
  - clarified that current scope is not Ã¢â‚¬Å“full alert CRUDÃ¢â‚¬Â; it is primarily a proposal workflow:
    - create/list/detail proposal
    - submit / focal approve / final approve / reject / cancel
    - auto-create disclosure record + workflow instance after final approval
  - identified major contract debt / risks:
    - no real draft update API/UI after creation
    - FE title/description are inferred from backend `change_note`
    - FE `proposed_deadline_days` conflicts semantically with BE/DB `proposed_deadline_date`
    - `admin-approve` has partial-failure/data-consistency risk around downstream record/workflow creation
    - disclosure record creation adapter appears to reuse membership id as user id
    - disclosure type detail CTA for ad-hoc create is not explicitly gated by `ad_hoc_alert.propose`
    - docs drift: `docs/api-contracts-json.md` still describes older effective-workflow permission boundary
- affected repos/files/modules:
  - `cobo_iam_services/internal/adhoc/*`
  - `cobo_iam_services/internal/disclosure/*`
  - `cobo_iam_services/internal/httpserver/server.go`
  - `cobo_iam_services/docs/api-contracts-json.md`
  - `cobo_web_design/src/pages/portal/AdHocProposal*.tsx`
  - `cobo_web_design/src/pages/portal/DisclosureTypeDetail.tsx`
  - `cobo_web_design/src/services/adHocAlertsApi.ts`
  - `cobo_web_design/src/services/workflowOverrideMappers.ts`
  - `cobo_web_design/docs/canh-bao-bat-thuong-feature-doc.md`
  - `cobo_web_design/docs/permission_catalog.md`
- contracts/behaviors/constraints/decisions:
  - product wording should prefer Ã¢â‚¬Å“Ã„â€˜Ã¡Â»Â xuÃ¡ÂºÂ¥t cÃ¡ÂºÂ£nh bÃƒÂ¡o bÃ¡ÂºÂ¥t thÃ†Â°Ã¡Â»ÂngÃ¢â‚¬Â / Ã¢â‚¬Å“proposal workflowÃ¢â‚¬Â over Ã¢â‚¬Å“full CRUD cÃ¡ÂºÂ£nh bÃƒÂ¡oÃ¢â‚¬Â
  - final approval is identity-based via `process_controller_id`, not generic admin role-based
  - existing feature can be demoed, but documentation should not promise editable drafts or fully clean domain contracts
- build/verification result:
  - `BLOCKED:` review/documentation-only task; no runtime code changed in this cycle
- remaining gaps/risks/next steps:
  - decide whether to split follow-up into:
    - contract cleanup
    - correctness fixes
    - UX/permission alignment
    - business doc refresh in sibling frontend repo

## 2026-05-20 - Documentation split: dev action plan + BA/marketing one-pager for ad-hoc alert flow

- task type: documentation-derivative
- objective/question:
  - turn the current-state ad-hoc alert audit into:
    - a dev-facing priority action plan
    - a business-facing simplified one-pager
- implemented/discovered:
  - created:
    - `docs/ai-cache/adhoc-alert-crud-priority-action-plan.md`
    - `docs/ai-cache/adhoc-alert-business-one-pager.md`
  - dev plan is grouped by `P0/P1/P2` and focuses on execution order, ownership split, and acceptance
  - business one-pager strips most engineering detail and frames the feature as a Ã¢â‚¬Å“proposal + approval workflowÃ¢â‚¬Â, not full final-alert CRUD
- affected repos/files/modules:
  - `cobo_iam_services/docs/ai-cache/adhoc-alert-crud-priority-action-plan.md`
  - `cobo_iam_services/docs/ai-cache/adhoc-alert-business-one-pager.md`
- contracts/behaviors/constraints/decisions:
  - business narrative should use Ã¢â‚¬Å“Ã„â€˜Ã¡Â»Â xuÃ¡ÂºÂ¥t cÃ¡ÂºÂ£nh bÃƒÂ¡o bÃ¡ÂºÂ¥t thÃ†Â°Ã¡Â»ÂngÃ¢â‚¬Â and avoid overclaiming editable drafts / full CRUD
  - engineering follow-up should prioritize contract correctness and admin-approve consistency before broader UX polish
- build/verification result:
  - `BLOCKED:` documentation-only task; no runtime code changed in this cycle
- remaining gaps/risks/next steps:
  - optionally sync the business one-pager back into `cobo_web_design/docs/` if that repo is the canonical BA-facing doc location

## 2026-05-20 - Backlog refinement: Jira-ready checklist + P0 bug-vs-techdebt classification

- task type: backlog-refinement
- objective/question:
  - convert the ad-hoc alert `P0/P1/P2` action plan into Jira-ready ticket checklists
  - classify each `P0` item as confirmed bug, tech debt, or investigation/risk
- implemented/discovered:
  - created:
    - `docs/ai-cache/adhoc-alert-jira-ticket-ready-checklist.md`
    - `docs/ai-cache/adhoc-alert-p0-bug-vs-techdebt-review.md`
  - P0 classification result:
    - `P0.1` deadline contract mismatch => confirmed bug + contract debt
    - `P0.2` title/description via `change_note` => tech debt
    - `P0.3` admin-approve consistency => high-risk investigation / hardening, not yet proven as a live bug
    - `P0.4` downstream actor attribution => likely bug, needs persistence verification
    - `P0.5` backend validation parity => confirmed robustness bug
- affected repos/files/modules:
  - `cobo_iam_services/docs/ai-cache/adhoc-alert-jira-ticket-ready-checklist.md`
  - `cobo_iam_services/docs/ai-cache/adhoc-alert-p0-bug-vs-techdebt-review.md`
- contracts/behaviors/constraints/decisions:
  - Jira wording should distinguish:
    - bug
    - tech debt
    - investigation/hardening
  - not every P0 item should be filed as a confirmed runtime bug
- build/verification result:
  - `BLOCKED:` documentation-only task; no runtime code changed in this cycle
- remaining gaps/risks/next steps:
  - if needed, next step is to turn each P0 item into per-team tickets with owners / estimates / dependencies

## 2026-05-20 - Backlog planning: ticket estimates/dependency graph + P0.1 cross-repo implementation plan

- task type: backlog-planning
- objective/question:
  - assign `S/M/L` estimates and a dependency graph for ad-hoc alert tickets
  - break out `P0.1` into a cross-repo implementation plan ready to start once semantics are approved
- implemented/discovered:
  - created:
    - `docs/ai-cache/adhoc-alert-ticket-estimates-and-dependency-graph.md`
    - `docs/ai-cache/p0-1-deadline-contract-cross-repo-implementation-plan.md`
  - recommended ticket size summary:
    - `P0-1` M
    - `P0-2` L
    - `P0-3` L
    - `P0-4` M
    - `P0-5` S
    - `P1-1` L
    - `P1-2` S
    - `P1-3` M
    - `P1-4` M
    - `P1-5` S
    - `P2-*` S
  - identified dependency highlights:
    - `P0-5` depends on `P0-1`
    - `P1-1` depends on `P0-1` and partially `P0-2`
    - `P0-3` benefits from `P0-4`
    - docs refresh should happen after `P0/P1` decisions settle
  - `P0.1` implementation recommendation:
    - normalize whole system toward `proposed_deadline_days`
    - use additive migration path instead of overloading existing `DATE` semantics
- affected repos/files/modules:
  - `cobo_iam_services/docs/ai-cache/adhoc-alert-ticket-estimates-and-dependency-graph.md`
  - `cobo_iam_services/docs/ai-cache/p0-1-deadline-contract-cross-repo-implementation-plan.md`
- contracts/behaviors/constraints/decisions:
  - for `P0.1`, preferred direction is canonical `proposed_deadline_days` across FE/API/DB if product confirms day-count semantics
  - recommended migration strategy is additive:
    - add canonical field
    - dual-read/controlled compatibility
    - remove deprecated field later
- build/verification result:
  - `BLOCKED:` documentation-only task; no runtime code changed in this cycle
- remaining gaps/risks/next steps:
  - the only blocking decision before implementing `P0.1` is product confirmation on `days` vs `absolute date`

## 2026-05-20 - Review pack: mandatory pre-implementation questions for ad-hoc alert flow

- task type: review-facilitation
- objective/question:
  - compress the ad-hoc alert analysis into a short set of mandatory decision questions for a pre-implementation review meeting
- implemented/discovered:
  - created `docs/ai-cache/adhoc-alert-pre-implementation-review-pack.md`
  - packed the flow into 7 decision gates:
    - deadline semantics
    - draft semantics
    - title/description contract
    - downstream actor attribution
    - final approval hardening target
    - final approval identity rule
    - business/product wording
- affected repos/files/modules:
  - `cobo_iam_services/docs/ai-cache/adhoc-alert-pre-implementation-review-pack.md`
- contracts/behaviors/constraints/decisions:
  - the pack is intended to be used before implementation approval, not after coding starts
  - missing any of these decisions materially increases rework risk
- build/verification result:
  - `BLOCKED:` documentation-only task; no runtime code changed in this cycle
- remaining gaps/risks/next steps:
  - next practical step is to run the review meeting and fill the decision table at the end of the review-pack doc

## 2026-05-22 - FE build debugging: guard against sudo/root Node mismatch

- task type: bugfix
- objective/question:
  - explain and fix the frontend build failure triggered from `make fe-build`
- implemented/discovered:
  - confirmed the reported `vite` syntax error came from running FE build via `sudo`, which switched execution from user Node `v23.9.0` to a different root runtime too old for Vite 6 ESM entrypoints
  - added FE environment guards in `Makefile` so `fe-install`, `fe-dev`, `fe-build`, `fe-test`, and `fe-clean` now fail fast when invoked as `root` or with an unsupported Node version
  - reran `make fe-build` as the normal user and isolated a second issue: previous privileged builds left `../cobo_web_design/dist/assets` with foreign ownership, causing `EACCES` during Vite output cleanup
- affected repos/files/modules:
  - `cobo_iam_services/Makefile`
  - runtime artifact path: `cobo_web_design/dist`
- contracts/behaviors/constraints/decisions:
  - FE Make targets must run as the normal user, not via `sudo`
  - FE Make targets require a Node version matching Vite 6 engine support: `^18 || ^20 || >=22`
  - root-owned or foreign-owned FE build artifacts must be cleaned from a privileged shell before unprivileged FE builds can succeed again
- build/verification result:
  - `make fe-build` (normal user, outside sandbox): reached Vite build and then failed with `EACCES` on `../cobo_web_design/dist/assets`
  - `docker compose -f docker-compose.dev.yml build api`: passed
  - `BLOCKED:` agent environment could not repair ownership of `../cobo_web_design/dist` because privileged filesystem access was unavailable
- remaining gaps/risks/next steps:
  - run `sudo rm -rf ../cobo_web_design/dist`
  - rerun `make fe-build` without `sudo`

## 2026-05-22 - BE deploy debugging: prevent remote binary path from turning into a directory

- task type: bugfix
- objective/question:
  - fix `make deploy-be` failure where the `api` container could not start because `./bin/api` was treated as a directory
- implemented/discovered:
  - confirmed local artifacts `deploy-artifacts/backend/bin/api` and `worker` are valid ELF executables, so the failure was not from cross-compilation output
  - identified the most probable failure path in deploy logic: `scp ...:/root/cobo_project/bin/api` will copy *into* that path if it already exists as a directory, leaving Docker with `/app/bin/api` as a directory mount target
  - changed `deploy-be` to remove any stale remote `bin/api` and `bin/worker` paths first, upload to `.api.tmp` / `.worker.tmp`, then atomically `mv` to final executable names and `chmod 755` before restarting `api` and `worker`
- affected repos/files/modules:
  - `cobo_iam_services/Makefile`
- contracts/behaviors/constraints/decisions:
  - backend deploy must treat remote binary targets as files, not assume previous state is sane
  - deploy step is now idempotent against stale remote directories at `/root/cobo_project/bin/api` and `/root/cobo_project/bin/worker`
- build/verification result:
  - `make -n deploy-be`: confirmed new remote command order is `rm -rf -> scp to temp -> mv/chmod -> docker compose up`
  - `docker compose -f docker-compose.dev.yml build api`: passed
  - `BLOCKED:` direct SSH verification against the dev server was not possible from the agent environment because `root@88.216.208.0:21239` authentication was unavailable here
- remaining gaps/risks/next steps:
  - rerun `make deploy-be` from your shell with working SSH credentials to clean remote stale paths and redeploy binaries

## 2026-05-22 - Deploy script: allow interactive SSH password during preflight

- task type: bugfix
- objective/question:
  - make `deploy-dev.sh` allow password-based SSH instead of failing early in preflight when no SSH key is loaded
- implemented/discovered:
  - confirmed the failing behavior came from `ssh ... -o BatchMode=yes` in `preflight_check`, which forces non-interactive auth and suppresses password prompts
  - removed `BatchMode=yes` from preflight and added an explicit informational log that SSH may prompt for a password
  - updated the script header and failure guidance to reflect that both SSH key and password flows are valid
- affected repos/files/modules:
  - `cobo_iam_services/deploy-dev.sh`
- contracts/behaviors/constraints/decisions:
  - deploy preflight now supports either SSH key auth or interactive password auth
  - the rest of the deploy flow still depends on the invoking terminal being able to satisfy SSH/SCP prompts interactively
- build/verification result:
  - code review/readback confirmed `preflight_check` no longer passes `BatchMode=yes`
  - `docker compose -f docker-compose.dev.yml build api`: passed
  - `BLOCKED:` agent environment could not complete an end-to-end interactive SSH/password verification against the dev server
- remaining gaps/risks/next steps:
  - rerun `make deploy-dev MODE=migrate` from your terminal and confirm password prompt appears

## 2026-05-22 - Deploy script: fail hard and classify schema_migrations lookup errors

- task type: bugfix
- objective/question:
  - make `deploy-dev.sh` explain why reading `schema_migrations` failed and stop the deploy immediately instead of skipping migrations silently
- implemented/discovered:
  - replaced the old `schema_migrations` lookup path that swallowed stderr and returned early with a warning
  - `deploy_migrations()` now captures the real `ssh`/`docker exec`/`mysql` error output and classifies common cases:
    - SSH authentication failure
    - missing `cobo-iam-mysql` container
    - non-running MySQL container
    - MySQL credential rejection
    - missing `cobo_iam` database
    - missing `schema_migrations` table
    - MySQL connection failure inside the dev stack
  - the script now exits with status `1` when migration state cannot be determined, so it no longer prints a misleading success path after migrations were skipped
- affected repos/files/modules:
  - `cobo_iam_services/deploy-dev.sh`
- contracts/behaviors/constraints/decisions:
  - migration deploy requires a successful remote read of `schema_migrations`; otherwise the deploy is considered failed
  - stderr from the remote migration-state probe is now preserved for diagnostics instead of being discarded
- build/verification result:
  - `sh -n deploy-dev.sh`: passed
  - `docker compose -f docker-compose.dev.yml build api`: passed
  - `BLOCKED:` interactive end-to-end migration verification against the dev server was not possible from the agent environment
- remaining gaps/risks/next steps:
  - rerun `make deploy-dev MODE=migrate` and confirm the new error classification matches the actual remote fault if migration lookup still fails

## 2026-05-22 - Deploy script: remove shell command substitution from migration error messages

- task type: bugfix
- objective/question:
  - fix the new `deploy-dev.sh` regression where migration diagnostics printed `schema_migrations: not found` and `cobo_iam: not found`
- implemented/discovered:
  - traced the regression to backticks inside double-quoted `log_error` messages in a `/bin/sh` script
  - `/bin/sh` treated `` `schema_migrations` `` and `` `cobo_iam` `` as command substitution, which corrupted the diagnostic output
  - replaced those backticks with plain single-quoted identifiers in the migration error branch
- affected repos/files/modules:
  - `cobo_iam_services/deploy-dev.sh`
- contracts/behaviors/constraints/decisions:
  - shell-visible diagnostics in POSIX `sh` scripts must avoid backticks inside double-quoted strings unless command substitution is intended
- build/verification result:
  - `sh -n deploy-dev.sh`: passed
  - `docker compose -f docker-compose.dev.yml build api`: passed
  - `BLOCKED:` end-to-end interactive verification against the dev server was not possible from the agent environment
- remaining gaps/risks/next steps:
  - rerun `make deploy-dev MODE=migrate`; if the migration lookup still fails, the next log should now show the real classified cause cleanly

## 2026-05-22 - Deploy migration bootstrap: sync remote migrations dir and auto-bootstrap missing schema_migrations

- task type: bugfix
- objective/question:
  - fix dev migration bootstrap when the remote server does not yet have `/root/cobo_project/migrations/run_dev_migrations.sh` and `schema_migrations` is missing
- implemented/discovered:
  - confirmed the remote bootstrap failure was caused by a missing `migrations/` directory payload on the dev server, even though `docker-compose.artifacts.yml` expects it to exist for the `migrate` service
  - updated `deploy-dev.sh` so `deploy_migrations()` now:
    - synchronizes the full local `migrations/` directory to the dev server before any comparison
    - attempts the normal `schema_migrations` query
    - automatically runs `docker compose -f docker-compose.artifacts.yml run --rm migrate` when the table is missing
    - retries the migration-state query after bootstrap before continuing
  - updated `Makefile` so `deploy-init` and `deploy-be` also copy the `migrations/` directory to the dev server, preventing future artifacts environments from missing the migration runner inputs
- affected repos/files/modules:
  - `cobo_iam_services/deploy-dev.sh`
  - `cobo_iam_services/Makefile`
- contracts/behaviors/constraints/decisions:
  - the dev artifacts environment now treats `migrations/` as a required deploy asset, not an implicit manual prerequisite
  - missing `schema_migrations` is now a bootstrap case, not just a hard error, provided the migration runner can execute successfully
- build/verification result:
  - `sh -n deploy-dev.sh`: passed
  - `make -n deploy-be`: confirmed migrations directory is now copied during backend deploy
  - `docker compose -f docker-compose.dev.yml build api`: passed
  - `BLOCKED:` end-to-end interactive verification against the dev server was not possible from the agent environment
- remaining gaps/risks/next steps:
  - rerun `make deploy-dev MODE=migrate` from your terminal; the script should now sync `migrations/`, bootstrap `schema_migrations` if needed, and then continue with the normal diff/apply flow

## 2026-05-22 - Migration fix: align disclosure_template_blocks FK column type with disclosure_type_versions

- task type: bugfix
- objective/question:
  - fix bootstrap migration failure at `0016_disclosure_template_blocks.up.sql` caused by an incompatible foreign key definition
- implemented/discovered:
  - confirmed from bootstrap logs that MySQL failed on foreign key `fk_disclosure_template_blocks_version`
  - traced the incompatibility to `disclosure_template_blocks.type_id` being declared as `VARCHAR(100)` while the parent key `disclosure_type_versions.type_id` is `VARCHAR(64)`
  - updated `0016_disclosure_template_blocks.up.sql` to use `VARCHAR(64)` and explicitly match the table engine/charset/collation used by the parent tables
- affected repos/files/modules:
  - `cobo_iam_services/migrations/0016_disclosure_template_blocks.up.sql`
- contracts/behaviors/constraints/decisions:
  - string foreign key columns must match both logical width and storage charset/collation of the referenced parent key
- build/verification result:
  - readback confirmed `0016_disclosure_template_blocks.up.sql` now defines `type_id VARCHAR(64)` with `ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`
  - `docker compose -f docker-compose.dev.yml build api`: passed
  - `BLOCKED:` end-to-end rerun of the migration bootstrap against the dev server was not possible from the agent environment
- remaining gaps/risks/next steps:
  - rerun `make deploy-dev MODE=migrate`; bootstrap should now pass `0016` and either continue or reveal the next failing migration, if any

## 2026-05-22 - Migration fix: align company_template_workflow_overrides table charset/collation with disclosure_types

- task type: bugfix
- objective/question:
  - fix migration failure at `0020_company_template_workflow_overrides.up.sql` on foreign key `fk_ctwo_type`
- implemented/discovered:
  - confirmed `type_id` width already matched `disclosure_types.type_id` (`VARCHAR(64)`), so the remaining incompatibility was table-level storage definition
  - `0020` created both override tables without explicit `ENGINE/CHARSET/COLLATE`, unlike the referenced disclosure catalog tables created earlier
  - updated both tables in `0020_company_template_workflow_overrides.up.sql` to use `ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`
- affected repos/files/modules:
  - `cobo_iam_services/migrations/0020_company_template_workflow_overrides.up.sql`
- contracts/behaviors/constraints/decisions:
  - string foreign keys into disclosure catalog tables must match table charset/collation, not just column width
- build/verification result:
  - readback confirmed both tables in `0020` now explicitly use `InnoDB/utf8mb4/utf8mb4_unicode_ci`
  - `sh -n deploy-dev.sh`: passed
  - `docker compose -f docker-compose.dev.yml build api`: passed
  - `BLOCKED:` end-to-end migration rerun against the dev server was not possible from the agent environment
- remaining gaps/risks/next steps:
  - rerun `make deploy-dev MODE=migrate`; if there is another legacy migration with implicit table collation, the next failing FK should now be exposed directly

## 2026-05-22 - Deploy fix: build static Linux binaries for Alpine runtime

- task type: bugfix
- objective/question:
  - fix dev runtime failures `exec ./bin/api: no such file or directory` and `exec ./bin/worker: no such file or directory`
- implemented/discovered:
  - confirmed the deployed backend artifacts were Linux ELF binaries dynamically linked against glibc (`interpreter /lib64/ld-linux-x86-64.so.2`)
  - confirmed the dev runtime image is Alpine-based, so those binaries fail at process start even when the file exists
  - updated `be-build-linux` in `Makefile` to compile deploy artifacts with `CGO_ENABLED=0`, producing statically linked binaries compatible with the current Alpine runtime image
- affected repos/files/modules:
  - `cobo_iam_services/Makefile`
  - generated artifacts: `deploy-artifacts/backend/bin/api`, `deploy-artifacts/backend/bin/worker`
- contracts/behaviors/constraints/decisions:
  - deploy artifacts for the SSH/dev-server flow must be static Linux binaries unless the runtime image is changed to ship glibc
- build/verification result:
  - `make be-build-linux`: passed outside sandbox
  - `file deploy-artifacts/backend/bin/api deploy-artifacts/backend/bin/worker`: both now report `statically linked`
  - `docker compose -f docker-compose.dev.yml build api`: passed
- remaining gaps/risks/next steps:
  - rerun `make deploy-be` so the new static binaries are copied to `/root/cobo_project/bin/`
  - after backend is healthy again, `web` should stop failing on upstream resolution of `api`

## 2026-05-22 - Cross-repo documentation inventory: FE and BE feature/flow map

- task type: documentation
- objective/question:
  - review both FE and BE codebases and list the currently designed functions and flows into one file
- implemented/discovered:
  - created a single cross-repo inventory document based on:
    - FE route wiring in `src/App.tsx`
    - FE service clients in `src/services/**`, `src/features/admin-core/services/**`, `src/features/cms-core/services/**`
    - BE route registration in `internal/httpserver/server.go` and transport handlers
    - active product/spec docs in both repos
  - the resulting document groups the project into:
    - public/auth/account lifecycle
    - company context and portal shell
    - portal business flows (disclosure, deadlines, workflow, notifications, ad-hoc)
    - enterprise admin flows
    - CMS/platform admin flows
    - authorization/audit/outbox/worker internals
- affected repos/files/modules:
  - `cobo_iam_services/docs/current-feature-flow-inventory.md`
- contracts/behaviors/constraints/decisions:
  - because the session did not provide the expected `integration-cross-repo` skill, the task was completed using `documentation-and-adrs` plus direct code/spec review across both repos
  - the inventory intentionally documents features/flows that are visible in current code/spec wiring, not hypothetical backlog items
- build/verification result:
  - `BLOCKED:` documentation-only task; no runtime code changed in this cycle
- remaining gaps/risks/next steps:
  - if needed, next step is to split this inventory into:
  - endpoint matrix by module
  - screen-to-endpoint traceability table
  - implemented vs partial vs planned status map

## 2026-05-22 - Documentation split: route matrix, implementation status, CMS and Enterprise Admin inventories

- task type: documentation
- objective/question:
  - split the master cross-repo inventory into smaller documents for traceability and status review
- implemented/discovered:
  - created:
    - `docs/fe-route-to-be-endpoint-matrix.md`
    - `docs/feature-implementation-status.md`
    - `docs/cms-feature-inventory.md`
    - `docs/enterprise-admin-feature-inventory.md`
  - the split now supports four different review lenses:
    - FE route to BE endpoint traceability
    - implemented vs partial vs planned status
    - CMS-only scope
    - Enterprise Admin-only scope
- affected repos/files/modules:
  - `cobo_iam_services/docs/fe-route-to-be-endpoint-matrix.md`
  - `cobo_iam_services/docs/feature-implementation-status.md`
  - `cobo_iam_services/docs/cms-feature-inventory.md`
  - `cobo_iam_services/docs/enterprise-admin-feature-inventory.md`
- contracts/behaviors/constraints/decisions:
  - status labels were defined conservatively:
    - `Implemented` only when FE and BE wiring are both visible
    - `Partial` when the route/design is clear but the backend surface is incomplete or indirect
    - `Planned` for spec-visible areas not fully evidenced in current code wiring
  - although the user said "CMS hoÃ¡ÂºÂ·c Enterprise Admin", both focused inventories were generated because they are separate major surfaces and the split is more reusable this way
- build/verification result:
  - `BLOCKED:` documentation-only task; no runtime code changed in this cycle
- remaining gaps/risks/next steps:
  - next useful split, if needed:
    - by backend module (`iam`, `companyaccess`, `disclosure`, `workflow`, `platformcms`)
    - by lifecycle (`auth`, `disclosure`, `reminder`, `ops`)
- 2026-05-22: Added two more documentation slices on top of the workspace feature inventory:
  - `docs/backend-module-inventory.md` groups current design by backend ownership (`iam`, `companyaccess`, `disclosure`, `workflow`, `platformcms`), with FE surfaces, major endpoints, and flow ownership per module.
  - `docs/lifecycle-flow-inventory.md` groups the same system by operational lifecycle (`auth`, `disclosure`, `reminder`, `ops`) to make end-to-end tracing easier than route-by-route reading.
- 2026-05-22: Added `docs/inventory-index.md` as the short navigation entrypoint for the full documentation set, with a "read this when you want to know X" table linking the master inventory, route matrix, status map, module view, lifecycle view, CMS inventory, and Enterprise Admin inventory.
- 2026-05-22: Converted the email notification SA into an implementation-ready rollout plan in `docs/email-notification-system-implementation-plan.md`.
  - grounded the plan in the current repo reality:
    - auth emails are still built in `internal/iam/app/service.go`
    - reminder email still sends directly from `internal/reminder/infra/email/smtp_sender.go`
    - worker still handles auth outbox events in `cmd/worker/main.go`
    - `internal/notification` already exists for generic notification jobs, so the safest plan is to extend it with email-focused subpackages/files instead of replacing it
  - the plan includes:
    - phase-by-phase rollout with feature flags and rollback
    - current code impact map
    - target package structure
    - DB migration plan
    - testing strategy
    - risk register
    - PR plan
    - staging/production deployment checklist

## 2026-05-22 - Artifacts migrate service DB credentials bootstrap fix

- task type: bugfix
- objective/question:
  - fix dev artifacts migration bootstrap when `docker-compose.artifacts.yml` runs `migrate` with default `root/root` credentials that do not have DDL access from the migrate container
- implemented/discovered:
  - confirmed on the dev server that `migrate` failed at `CREATE TABLE schema_migrations` with `ERROR 1142 (42000): CREATE command denied to user 'root'@'172.18.0.x'`
  - traced the failure to `migrations/run_dev_migrations.sh` defaulting to `DB_USER=root`, `DB_PASSWORD=root`, `DB_NAME=cobo_iam` when compose does not inject overrides
  - updated `docker-compose.artifacts.yml` so service `migrate` now passes:
    - `DB_HOST=mysql`
    - `DB_USER=cobo`
    - `DB_PASSWORD=cobo`
    - `DB_NAME=cobo_iam`
- affected repos/files/modules:
  - `cobo_iam_services/docker-compose.artifacts.yml`
- contracts/behaviors/constraints/decisions:
  - the dev artifacts migration runner should use the same database principal as `api` and `worker` unless a dedicated migration principal is explicitly provisioned
  - relying on cross-container MySQL `root` defaults is not safe across dev servers
- build/verification result:
  - `BLOCKED:` local `docker compose -f docker-compose.artifacts.yml config` crashed in the agent environment's Docker CLI plugin before returning config output
  - `BLOCKED:` remote end-to-end rerun of `migrate` was not available from the agent environment
- remaining gaps/risks/next steps:
  - rerun `docker compose -f docker-compose.artifacts.yml run --rm migrate` on the dev server
  - if migrations complete, restart `api`, `worker`, and `web`, then verify login shifts from `500` to `401 INVALID_CREDENTIALS` or `200`

## 2026-05-22 - Migration runner: tolerate missing ALTER USER privilege

- task type: bugfix
- objective/question:
  - allow dev migration bootstrap to continue when the application DB user can create tables in `cobo_iam` but cannot execute `ALTER USER`
- implemented/discovered:
  - confirmed the remote runner progressed past `schema_migrations` creation with `cobo/cobo`, then failed on:
    - `ERROR 1227 (42000): Access denied; you need (at least one of) the CREATE USER privilege(s) for this operation`
  - updated both migration runner copies so the auth-plugin step is now best-effort:
    - `migrations/run_dev_migrations.sh`
    - `deploy-artifacts/backend/migrations/run_dev_migrations.sh`
  - failed `ALTER USER IF EXISTS ...` now emits a warning and does not abort the whole migration run
- affected repos/files/modules:
  - `cobo_iam_services/migrations/run_dev_migrations.sh`
  - `cobo_iam_services/deploy-artifacts/backend/migrations/run_dev_migrations.sh`
- contracts/behaviors/constraints/decisions:
  - schema/bootstrap migrations must not depend on account-management privileges when the runtime stack already authenticates successfully with the configured app user
  - auth plugin normalization remains opportunistic rather than mandatory in dev bootstrap
- build/verification result:
  - `BLOCKED:` remote rerun against the dev server was not available from the agent environment
- remaining gaps/risks/next steps:
  - rerun the remote `migrate` service with `DB_USER=cobo`
  - if another migration fails, capture the first failing SQL file name and MySQL error

## 2026-05-22 - Dev FE deploy path fix for nested dist directory

- task type: bugfix
- objective/question:
  - fix dev frontend deploy so `/root/cobo_project/web/dist` contains `index.html` directly instead of accidentally nesting as `web/dist/dist/index.html`
- implemented/discovered:
  - confirmed local FE build artifacts include `deploy-artifacts/web/dist/index.html`
  - traced the dev-server `403 /` to `deploy-fe` copying the whole `dist` directory into an existing remote `web/dist` path via `scp -r`, which can produce `web/dist/dist/...`
  - updated `Makefile` `deploy-fe` target to:
    - recreate remote `$(DEV_PATH)/web/dist`
    - copy the contents of local `dist` via `scp -r deploy-artifacts/web/dist/. .../web/dist/`
- affected repos/files/modules:
  - `cobo_iam_services/Makefile`
- contracts/behaviors/constraints/decisions:
  - dev FE artifacts on the server must be mounted such that nginx root `/usr/share/nginx/html` contains `index.html` at the top level
- build/verification result:
  - local artifact inspection confirmed `deploy-artifacts/web/dist/index.html` exists
  - `BLOCKED:` remote redeploy/verify was not available from the agent environment
- remaining gaps/risks/next steps:
  - rerun `make deploy-fe`
  - if the remote tree still looks wrong, inspect `/root/cobo_project/web/dist` and remove any stale nested `dist/` directory before restarting `web`

## 2026-05-22 - Email notification system phase 1 embed template extraction

- task type: feature implementation
- objective/question:
  - implement phase 1 from `docs/email-notification-system-implementation-plan.md` without changing auth/reminder business behavior:
    - extract current auth + reminder email content into `embed.FS` templates
    - add template registry + renderer
    - wire auth/reminder to use `EMAIL_TEMPLATE_SOURCE=embed`
    - fallback to legacy hardcoded rendering on registry/render failure
- implemented/discovered:
  - added `internal/notification/templates` with 6 template sets that mirror current outputs:
    - `auth.email_verification`
    - `auth.password_reset.user`
    - `auth.password_reset.admin`
    - `auth.user_invitation.new_user`
    - `auth.user_invitation.existing_user`
    - `reminder.disclosure_deadline`
  - added `internal/notification/app/email_contracts.go`, `email_renderer.go` and `internal/notification/infra/registry/embed_registry.go`
  - metadata supports required variable validation, locale fallback, and legacy trailing-newline preservation so rendered output matches current auth/reminder strings
  - auth flows now render through the template layer only when `EMAIL_TEMPLATE_SOURCE=embed`; otherwise they keep legacy strings
  - reminder SMTP sender now renders through the same embed template layer under the same flag, while preserving direct SMTP + retry path and falling back to legacy renderer on failure
  - added config wiring and runtime injection in:
    - `internal/httpserver/server.go`
    - `cmd/worker/main.go`
- affected repos/files/modules:
  - `cobo_iam_services/internal/notification/app/*`
  - `cobo_iam_services/internal/notification/infra/registry/*`
  - `cobo_iam_services/internal/notification/templates/*`
  - `cobo_iam_services/internal/iam/app/{hooks.go,service.go,service_test.go}`
  - `cobo_iam_services/internal/reminder/infra/email/{smtp_sender.go,smtp_sender_test.go}`
  - `cobo_iam_services/internal/platform/config/config.go`
  - `cobo_iam_services/internal/httpserver/server.go`
  - `cobo_iam_services/cmd/worker/main.go`
- contracts/behaviors/constraints/decisions:
  - `EMAIL_TEMPLATE_SOURCE` accepts `legacy|embed`; default remains `legacy`
  - phase 1 does not change:
    - outbox event types/payload shape (`to`, `subject`, `body`)
    - OTP/reset/invitation token generation
    - TTL source of truth
    - reminder SMTP dispatch and retry semantics
  - when embed registry/render fails under `embed`, auth/reminder fall back to legacy rendering instead of failing the business path
  - reminder embed output is normalized to CRLF body lines to preserve the legacy SMTP body formatting
- build/verification result:
  - `go test ./internal/notification/app ./internal/notification/infra/registry ./internal/reminder/infra/email ./internal/iam/app` Ã¢Å“â€¦
  - `go test ./internal/httpserver ./cmd/worker` Ã¢Å“â€¦
  - `docker compose -f docker-compose.dev.yml build api` Ã¢Å“â€¦
- remaining gaps/risks/next steps:
  - auth worker delivery still consumes rendered `subject/body` payload exactly as before; phase 2 is still needed for `NotificationService` and `email.dispatch`
  - only `vi` template files exist in phase 1; locale fallback is wired but not yet populated with additional locales
  - no HTML templates were added in phase 1 because the current legacy paths are plain-text only

## 2026-05-23 - DOC-001: Maker-checker capability split + sort_by API + CMS template migrations fix

- task type: implement (multi-increment)
- objective: chÃ¡Â»â€˜t 4 quyÃ¡ÂºÂ¿t Ã„â€˜Ã¡Â»â€¹nh DOC-001 vÃƒÂ  fix lÃ¡Â»â€”i danh sÃƒÂ¡ch template rÃ¡Â»â€”ng + API 500 trÃƒÂªn server dev
- what was implemented:

  **Increment 1 Ã¢â‚¬â€ BE: lifecycle capability split (maker-checker)**
  - tÃƒÂ¡ch permission `disclosure_type.manage` (maker: create/edit/submit-review) khÃ¡Â»Âi `disclosure_type.publish` (checker: publish/reject/archive)
  - thÃƒÂªm helper `companyTemplateLifecycleCapability()` trong `internal/disclosure/app/service.go`
  - test: `internal/disclosure/app/lifecycle_capability_test.go` Ã¢â‚¬â€ 7 tests, fakeAuthService, fakeLifecycleRepo

  **Increment 2 Ã¢â‚¬â€ BE: sort_by cho list API**
  - thÃƒÂªm `SortBy`/`SortDir` vÃƒÂ o `ListTypesParams` vÃƒÂ  `ListTypesRequest` (`internal/disclosure/app/contracts.go`)
  - validation + defaulting trong service (`created_at DESC` nÃ¡ÂºÂ¿u khÃƒÂ´ng truyÃ¡Â»Ân)
  - handler parse `sort_by`/`sort_dir` tÃ¡Â»Â« query params
  - repo SQL `ORDER BY` Ã„â€˜ÃƒÂ£ Ã„â€˜Ã†Â°Ã¡Â»Â£c wire thÃ¡Â»Â±c sÃ¡Â»Â± (trÃ†Â°Ã¡Â»â€ºc Ã„â€˜ÃƒÂ³ hardcode `t.type_id ASC`)
  - allowed values: `sort_by=name|created_at`, `sort_dir=asc|desc`

  **Increment 3 Ã¢â‚¬â€ BE: reset-override trÃ¡ÂºÂ£ 400 khi khÃƒÂ´ng cÃƒÂ³ override active**
  - `ResetCompanyWorkflowOverrideActive` trong service.go kiÃ¡Â»Æ’m tra `effective_source == "global_template"` trÃ†Â°Ã¡Â»â€ºc khi cho phÃƒÂ©p reset
  - trÃ¡ÂºÂ£ `400 INVALID_REQUEST` thay vÃƒÂ¬ 200 no-op

  **Increment 4 Ã¢â‚¬â€ FE: capability split cho action visibility**
  - `canManageCompanyTemplate` (disclosure_type.manage) Ã¢â‚¬â€ gate submit-review
  - `canPublishCompanyTemplate` (disclosure_type.publish) Ã¢â‚¬â€ gate publish/reject/archive
  - cÃ¡ÂºÂ­p nhÃ¡ÂºÂ­t `types.ts` Ã„â€˜Ã¡Â»Æ’ thÃƒÂªm 3 permission mÃ¡Â»â€ºi vÃƒÂ o union
  - cÃ¡ÂºÂ­p nhÃ¡ÂºÂ­t regression tests Ã„â€˜Ã¡Â»Æ’ reflect permission split

  **Fix: CMS migrations Ã„â€˜ÃƒÂºng sÃ¡Â»â€˜ (0053Ã¢â‚¬â€œ0057)**
  - root cause: `deploy-artifacts/backend/migrations/0045Ã¢â‚¬â€œ0047` cÃƒÂ³ nÃ¡Â»â„¢i dung CMS nhÃ†Â°ng Ã„â€˜ÃƒÂ£ bÃ¡Â»â€¹ conflict sÃ¡Â»â€˜ vÃ¡Â»â€ºi migration chÃƒÂ­nh
  - fix tÃ¡Â»Â« session trÃ†Â°Ã¡Â»â€ºc: tÃ¡ÂºÂ¡o lÃ¡ÂºÂ¡i Ã„â€˜ÃƒÂºng Ã¡Â»Å¸ `migrations/0053Ã¢â‚¬â€œ0055` (Ã„â€˜ÃƒÂ£ cÃƒÂ³), `0056_company_template_lifecycle`, `0057_workflow_override_versioning`
  - session nÃƒÂ y xÃƒÂ³a file duplicate 0058/0059/0060 (Ã„â€˜Ã†Â°Ã¡Â»Â£c tÃ¡ÂºÂ¡o nhÃ¡ÂºÂ§m khi context window cÃ…Â©)

  **Fix: repo SQL sort thÃ¡Â»Â±c sÃ¡Â»Â± Ã„â€˜Ã†Â°Ã¡Â»Â£c dÃƒÂ¹ng**
  - `internal/disclosure/infra/mysql/repository.go` Ã¢â‚¬â€ `ORDER BY` dÃƒÂ¹ng `sortCol`/`sortDir` thay vÃƒÂ¬ hardcode

  **ChÃ¡ÂºÂ©n Ã„â€˜oÃƒÂ¡n API 500 trÃƒÂªn server dev**
  - nguyÃƒÂªn nhÃƒÂ¢n: migration 0053Ã¢â‚¬â€œ0057 chÃ†Â°a Ã„â€˜Ã†Â°Ã¡Â»Â£c apply lÃƒÂªn DB server Ã¢â€ â€™ `review_status`, `is_mandatory` columns thiÃ¡ÂºÂ¿u Ã¢â€ â€™ MySQL error Ã¢â€ â€™ 500
  - fix: apply migrations thÃ¡Â»Â§ cÃƒÂ´ng trÃƒÂªn server (xem lÃ¡Â»â€¡nh bÃƒÂªn dÃ†Â°Ã¡Â»â€ºi)

- affected repos/files/modules:
  - `cobo_iam_services/internal/disclosure/app/service.go`
  - `cobo_iam_services/internal/disclosure/app/contracts.go`
  - `cobo_iam_services/internal/disclosure/app/lifecycle_capability_test.go` (NEW)
  - `cobo_iam_services/internal/disclosure/transport/http/handler.go`
  - `cobo_iam_services/internal/disclosure/infra/mysql/repository.go`
  - `cobo_web_design/src/types.ts`
  - `cobo_web_design/src/pages/portal/DisclosureTypeDetail.tsx`
  - `cobo_web_design/src/pages/portal/DisclosureTypeDetail.lifecycle-regression.test.tsx`
  - `cobo_web_design/src/pages/portal/DisclosureTypeDetail.fe004cd-regression.test.tsx`
  - migrations 0053Ã¢â‚¬â€œ0057 (Ã„â€˜ÃƒÂ£ tÃ¡Â»â€œn tÃ¡ÂºÂ¡i, khÃƒÂ´ng thay Ã„â€˜Ã¡Â»â€¢i)
  - xÃƒÂ³a migrations 0058/0059/0060 (duplicate, Ã„â€˜ÃƒÂ£ remove)

- contracts/behaviors/constraints/decisions:
  - `disclosure_type.manage`: create/edit/submit-review (maker)
  - `disclosure_type.publish`: publish/reject/archive (checker) Ã¢â‚¬â€ tÃƒÂ¡ch biÃ¡Â»â€¡t hoÃƒÂ n toÃƒÂ n, khÃƒÂ´ng kiÃƒÂªm nhiÃ¡Â»â€¡m
  - `archive` vÃƒÂ  `reject` cÃ…Â©ng thuÃ¡Â»â„¢c `publish` permission (khÃƒÂ´ng phÃ¡ÂºÂ£i `manage`) Ã„â€˜Ã¡Â»Æ’ Ã„â€˜Ã¡ÂºÂ£m bÃ¡ÂºÂ£o segregation of duties
  - sort default: `created_at DESC`
  - reset-override 400 khi `effective_source == "global_template"` (Option A trong DOC-001 Q3)

- build/verification result:
  - `go build ./...`: pass
  - FE: `npm run lint` (tsc --noEmit): pass
  - FE tests: lifecycle-regression + fe004cd-regression: pass sau khi fix permission assertions

- server dev apply migrations Ã¢â‚¬â€ lÃ¡Â»â€¡nh SCP + run:

  ```
  Server: 88.216.208.0:21239  user: root  path: /root/cobo_project
  DB container: cobo-iam-mysql  DB: cobo_iam  user/pass: root/root
  ```

  **BÃ†Â°Ã¡Â»â€ºc 1 Ã¢â‚¬â€ KiÃ¡Â»Æ’m tra migration hiÃ¡Â»â€¡n tÃ¡ÂºÂ¡i:**
  ```
  ssh -p 21239 root@88.216.208.0 "docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e \"SELECT file_name, executed_at FROM schema_migrations ORDER BY executed_at DESC LIMIT 20;\""
  ```

  **BÃ†Â°Ã¡Â»â€ºc 2 Ã¢â‚¬â€ XÃƒÂ³a file CMS cÃ…Â© sai sÃ¡Â»â€˜ (nÃ¡ÂºÂ¿u tÃ¡Â»â€œn tÃ¡ÂºÂ¡i):**
  ```
  ssh -p 21239 root@88.216.208.0 "rm -f /root/cobo_project/migrations/0045_cms_portal_template_tables.{up,down}.sql /root/cobo_project/migrations/0046_cms_display_groups_po_seed.{up,down}.sql /root/cobo_project/migrations/0047_cms_system_template_seed.{up,down}.sql && echo done"
  ```

  **BÃ†Â°Ã¡Â»â€ºc 3 Ã¢â‚¬â€ SCP 5 file migration mÃ¡Â»â€ºi:**
  ```powershell
  $SRC = "C:\Users\tvttt\OneDrive\Desktop\cobo\cobo_web\cobo_iam_services\migrations"
  $DST = "root@88.216.208.0:/root/cobo_project/migrations/"
  scp -P 21239 `
    "$SRC\0053_cms_portal_template_tables.up.sql" `
    "$SRC\0054_cms_display_groups_po_seed.up.sql" `
    "$SRC\0055_cms_system_template_seed.up.sql" `
    "$SRC\0056_company_template_lifecycle.up.sql" `
    "$SRC\0057_workflow_override_versioning.up.sql" `
    $DST
  ```

  **BÃ†Â°Ã¡Â»â€ºc 4 Ã¢â‚¬â€ Apply tÃ¡Â»Â«ng migration (idempotent, skip nÃ¡ÂºÂ¿u Ã„â€˜ÃƒÂ£ apply):**
  ```powershell
  $PORT = "21239"; $HOST = "root@88.216.208.0"
  $MP   = "/root/cobo_project/migrations"
  @("0053_cms_portal_template_tables.up.sql",
    "0054_cms_display_groups_po_seed.up.sql",
    "0055_cms_system_template_seed.up.sql",
    "0056_company_template_lifecycle.up.sql",
    "0057_workflow_override_versioning.up.sql") | ForEach-Object {
    $f = $_
    ssh -p $PORT $HOST @"
      already=`$(docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -Nse "SELECT COUNT(1) FROM schema_migrations WHERE file_name='$f'")
      if [ "`$already" -gt 0 ]; then echo "SKIP: $f"; else
        docker exec -i cobo-iam-mysql mysql -uroot -proot cobo_iam < $MP/$f && \
        docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e "INSERT IGNORE INTO schema_migrations(file_name) VALUES ('$f');" && \
        echo "OK: $f"
      fi
"@
  }
  ```

  **BÃ†Â°Ã¡Â»â€ºc 5 Ã¢â‚¬â€ Verify templates trong DB:**
  ```
  ssh -p 21239 root@88.216.208.0 "docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e \"SELECT type_id, status, is_mandatory FROM disclosure_types WHERE company_id IS NULL;\""
  ```
  KÃ¡ÂºÂ¿t quÃ¡ÂºÂ£ mong Ã„â€˜Ã¡Â»Â£i: 3 rows Ã¢â‚¬â€ `dt-sys-q1-financial`, `dt-sys-hr-executive`, `dt-sys-board-resolution`

- remaining gaps/risks/next steps:
  - sau khi apply migrations: rebuild Docker image API vÃ¡Â»â€ºi code mÃ¡Â»â€ºi (sort wiring + lifecycle capability) rÃ¡Â»â€œi redeploy
  - `docker compose -f docker-compose.artifacts.yml build api && docker compose -f docker-compose.artifacts.yml up -d api`
  - verify API khÃƒÂ´ng cÃƒÂ²n 500: `curl http://88.216.208.0:3000/api/v1/disclosure-types?page=1&page_size=20 -H "Authorization: Bearer <token>"`

## 2026-06-08 - Batch 2A: wire durable email pipeline end-to-end (Transactional Publish + worker registration)

- task type: feature implementation (scoped remediation batch Ã¢â‚¬â€ adhoc-email-spec-v3.md / Batch 2A rescoping plan, "PASS Ã¢â‚¬â€ READY TO IMPLEMENT")
- objective/question:
  - finish wiring the durable `email.dispatch` outbox pipeline end-to-end per ADR-3, strictly within the 7-item Batch 2A scope: Transactional Dispatch, `InsertNotificationTx`, `WithTransactionalDispatch`, `toEmailOutboxEvent`, worker `email.dispatch` registration, handler DI wiring, synthetic E2E test
  - mandatory transaction decision: Option A Ã¢â‚¬â€ Transactional Publish (`BeginTx -> InsertNotificationTx -> PublishEventTx -> Commit`, no best-effort publish)
  - mandatory worker registration shape: wrapper closure `event.PayloadJSON -> emailDispatchHandler.Handle(ctx, event.PayloadJSON)`, never a direct method-value `processor.Register(type, handler.Handle)`
- implemented/discovered:
  - pre-implementation verification found **zero drift**: every constructor/interface signature assumed by the rescoping plan (`NewEmailDispatchHandler`, `NewEmailNotificationRepository`, `NewEmailDeliveryAttemptRepository`, `WithTransactionalEnqueue`/`toOutboxEvent`/`deliverAuthEmailEvent` patterns, `outboxmysql.Repository.PublishEventTx`, `platformoutbox.Processor.Register`/`HandlerFunc`) matched exactly Ã¢â‚¬â€ proceeded without redesign
  - added `OutboxPublisher` (`PublishEventTx(ctx, tx, event)`) and `TxEmailNotificationRepository` (`EmailNotificationRepository` + `InsertNotificationTx`) contracts to `email_dispatch_contracts.go`
  - `EmailNotificationRepository.InsertNotificationTx` shares an `insertNotification(ctx, ex emailExecer, n)` helper with `InsertNotification` via an `emailExecer` seam (`*sql.DB`/`*sql.Tx`), mirroring the `execer`/`createJob` pattern in the sibling job repository
  - `EmailNotificationService` gained `sqlDB`/`outbox` fields, `EmailServiceOption` (renamed from the originally-planned `ServiceOption` Ã¢â‚¬â€ that name collides with `notification.service.ServiceOption` in the same `app` package), and `WithTransactionalDispatch(db, outbox)`
  - `DispatchEmail` now branches: when `sqlDB`+`outbox` are set AND `repo` satisfies `TxEmailNotificationRepository`, it calls `dispatchTransactional` (Insert + Publish + Commit atomically, with `ErrAlreadyDispatched` replay short-circuit); otherwise the existing non-transactional path is unchanged Ã¢â‚¬â€ every existing construction site keeps compiling/passing
  - `toEmailOutboxEvent` builds the `email.dispatch` envelope with the **plain** (unsanitized) `req.Variables` in the payload Ã¢â‚¬â€ never `VariablesJSONSanitized` Ã¢â‚¬â€ because the worker handler needs real values (e.g. OTP) to render while the persisted row stores only the redacted copy
  - `cmd/worker/main.go`: added `notificationmysql`/`notificationsmtp` imports; inside the existing `if sqlDB != nil` gate, constructed real MySQL-backed `EmailNotificationRepository`/`EmailDeliveryAttemptRepository`, the SMTP `DeliveryAdapter` (mirroring `httpserver/server.go`'s `notificationsmtp.NewAdapter(Config{Host/Port/User/Pass/From: cfg.SMTP*}, nil)`), and `EmailDispatchHandler`, then registered `email.dispatch` via the mandated wrapper-closure shape Ã¢â‚¬â€ gated on `sqlDB != nil` because the handler requires real DB-backed repositories that cannot be constructed without it
  - deliberately did **not** construct `EmailNotificationService` in `httpserver/server.go`'s production DI graph: there is no in-scope caller (`adhoc`/Batch 2 untouchable; no existing dispatch/preview admin route), so doing so would be dead code or scope creep Ã¢â‚¬â€ AK.4's acceptance criterion ("constructed and reachable, proven only by the synthetic test") is satisfied by the new E2E test alone
  - added `internal/notification/app/email_dispatch_e2e_test.go` (package `app_test`) Ã¢â‚¬â€ a **real-MySQL** synthetic E2E exercising the genuine production chain: `EmailNotificationService(WithTransactionalDispatch)` -> real `outboxmysql.Repository` -> real `platformoutbox.Processor.Tick` -> the exact wrapper-closure registration -> `EmailDispatchHandler` -> fake `DeliveryAdapter` (only the SMTP transport is faked):
    - `TestEmailDispatchE2E_TransactionalPublishWorkerDeliversHappyPath`: dispatch inserts+publishes atomically (status=pending, outbox row=pending) -> one `Tick` delivers -> status=sent/`sent_at` set/adapter called once with the real OTP in body -> outbox row=processed; then proves replay-safety (same idempotency key returns the same row, no duplicate outbox publish, no resend on a follow-up tick)
    - `TestEmailDispatchE2E_TransientErrorRetriesThenSucceeds`: first attempt returns a transient SMTP error -> notification=retry/`last_error_code=transient_smtp`, outbox row stays `pending` with a future `available_at` (redelivery scheduled, not dropped) -> an early tick does not redeliver -> forcing `available_at` into the past (simulates backoff elapsing) lets the processor redeliver -> second attempt succeeds -> status=sent, outbox row=processed
    - follows the `iam/infra/mysql/credentials_subscription_test.go` `openTestDB`/`t.Skipf` convention (`root:secret@tcp(127.0.0.1:3306)/cobo_iam?...`), with a run-unique `e2eIDGen` prefix and `t.Cleanup` deletes (`email_notifications` cascades to `email_delivery_attempts` via FK) so reruns against a persistent DB never collide or accumulate rows
- affected repos/files/modules:
  - `cobo_iam_services/internal/notification/app/email_dispatch_contracts.go`
  - `cobo_iam_services/internal/notification/app/email_service.go`
  - `cobo_iam_services/internal/notification/infra/mysql/email_notification_repository.go`
  - `cobo_iam_services/cmd/worker/main.go`
  - `cobo_iam_services/internal/notification/app/email_dispatch_e2e_test.go` (new)
- contracts/behaviors/constraints/decisions:
  - `WithTransactionalDispatch` is purely additive Ã¢â‚¬â€ services constructed without it keep the legacy non-transactional `InsertNotification` path; no existing call site changed behavior
  - `EmailServiceOption` (not `ServiceOption`) is the option-type name for `EmailNotificationService` Ã¢â‚¬â€ required because `ServiceOption` already exists for `notification.service` in the same `app` package
  - did not touch `internal/adhoc/...`, `AdhocProposalNotifier`, `internal/reminder/...`, migrations 0051/0052, retry/backoff constants, `EMAIL_SHADOW_MODE`, or any Batch 1 / Batch 5(a) surface
- build/verification result:
  - `go build ./...` Ã¢Å“â€¦
  - `go vet ./...` Ã¢Å“â€¦
  - `go test -race ./internal/notification/... ./internal/platform/outbox/...`: Ã¢Å“â€¦ all packages pass except one **pre-existing, unrelated** failure Ã¢â‚¬â€ `TestDispatchEmail_SanitisesAllSensitiveVars` fails identically on the unmodified baseline (`90b3db5`, verified via `git stash -u` + rerun): a template/test-fixture mismatch where `auth.user_invitation.new_user_company` requires `support_email` but the test's `Variables` map omits it. Not a Batch 2A regression; out of this batch's scope
  - the two new synthetic E2E tests compile and run the `t.Skipf` path cleanly (`--- SKIP`) in this sandbox (no MySQL/Docker available Ã¢â‚¬â€ `docker ps` reported no docker cmd); they are written to run for real against a migrated MySQL instance (e.g. the staging box at `88.216.208.0:21239` per [[reference_dev_server]]) and assert the full pending -> sent / pending -> retry -> sent transitions there
- remaining gaps/risks/next steps:
  - BLOCKED (environment): could not execute the synthetic E2E tests against a live MySQL in this sandbox Ã¢â‚¬â€ they are runnable in staging/CI where `MYSQL_DSN`/Docker are available; recommend running `go test -race ./internal/notification/app/... -run EmailDispatchE2E -v` there before considering Batch 2A merge-ready
  - the pre-existing `TestDispatchEmail_SanitisesAllSensitiveVars` failure should be triaged separately (likely needs `support_email` added to the test's `Variables`, or the template's required-var list reviewed) Ã¢â‚¬â€ not addressed here per "khÃƒÂ´ng mÃ¡Â»Å¸ rÃ¡Â»â„¢ng scope"
  - `EmailNotificationService` remains uncalled in production (`httpserver/server.go`); wiring an actual caller (e.g. an admin preview/dispatch route) is Batch 2 / new-surface territory and intentionally deferred

## 2026-06-08 - Batch 2: AdhocProposalNotifier cutover to durable email pipeline (Publisher-side Shadow Recipient Rewrite)

- task type: implement
- objective: cut `AdhocProposalNotifier` over to the durable `EmailNotificationService` pipeline behind two flags (`ADHOC_EMAIL_OUTBOX_ENABLED`, `EMAIL_SHADOW_MODE` + `ADHOC_EMAIL_SHADOW_RECIPIENT`) per the canonical D1 decision Ã¢â‚¬â€ Publisher-side Shadow Recipient Rewrite (NOT a worker-side adapter) Ã¢â‚¬â€ while keeping legacy byte-identical when both flags are off, and closing CF-12 (detached-context goroutine dispatch)
- what was implemented:
  - Config (Phase 1): `AdhocEmailOutboxEnabled`, `AdhocEmailShadowRecipient`, `AdhocEmailMetricsEnabled` added to `internal/platform/config/config.go` + `configs/config.example.env` (env: `ADHOC_EMAIL_OUTBOX_ENABLED`, `ADHOC_EMAIL_SHADOW_RECIPIENT`, `ADHOC_EMAIL_METRICS_ENABLED`)
  - Shadow Recipient Rewrite (Phase 2): `notifier.go`'s `sendEmail` is now a 3-way router Ã¢â‚¬â€ `outboxEnabled` (durable-only, real recipient, "wins") -> `shadowMode` (legacy to real recipient via new `sendLegacyEmail` extraction + durable to `shadowRecipient` via new `dispatchDurable`) -> default (legacy-only, byte-identical). The rewrite happens ONLY at the `sendEmail` call site for the durable branch Ã¢â‚¬â€ `sendLegacyEmail`/`dispatchDurable` never see or alter `recipientMembershipID`/idempotency key/audit fields
  - Notifier migration (Phase 3): all 4 `Notify*` methods now thread `eventType, recipientMembershipID, companyID` through to `sendEmail` -> `dispatchDurable`, which computes the LOCKED idempotency key `adhoc.<event_type>.<proposalID>.<recipientMembershipID>` and calls `DispatchEmail` with `SourceAggregateID=proposalID` (real, never rewritten)
  - CF-12 closure (Phase 4): deleted `dispatchNotificationAsync` (was `go func(){ ...context.Background()... }`) entirely from `internal/adhoc/app/service.go`; all 4 call sites now call `s.notifier.Notify*(ctx, ...)` synchronously with the live request-scoped `ctx`; removed the now-unused `log/slog` import
  - Metrics (Phase 5): added `cobo_adhoc_email_shadow_total{company_id, outcome="match"|"mismatch"}` (`internal/adhoc/observability/metrics.go`, gated by `AdhocEmailMetricsEnabled` -> `adhocapp.NewNoopMetrics()` when off) and `RecordEmailShadowOutcome(companyID, outcome)` on the `adhocapp.Metrics` interface (+ `noopMetrics`, `spyMetrics` test stub). Grounding note: the spec's literal `COUNT(*) GROUP BY idempotency_key HAVING COUNT(*) > 1` is structurally tautological Ã¢â‚¬â€ `email_notifications.idempotency_key` carries a DB UNIQUE constraint (`uk_email_notifications_idempotency`, migration `0051_email_notifications.up.sql:33`), so that count can never exceed 1. Reinterpreted as the functionally-equivalent in-app-observable signal: detect when `DispatchEmail` short-circuits to a pre-existing record (`FindByIdempotencyKey`) whose `SourceAggregateID`/`TemplateKey` differ from what this call sent Ã¢â‚¬â€ i.e. the same idempotency key resolved to a different notification intent (a real collision). `RecipientEmail` was deliberately rejected as a comparison field: in the shadow branch it is always rewritten to `shadowRecipient`, so comparing it can never produce "mismatch" Ã¢â‚¬â€ caught via self-review before the metric shipped
  - Tests (Phase 6): new `internal/adhoc/infra/notification/notifier_test.go` (`package notification_test`) Ã¢â‚¬â€ fakes (`noopInApp`, `recordingDeliveryAdapter`, `staticRegistry`/`staticRenderer` since `adhoc.*` template keys aren't in `notificationregistry.NewEmbedRegistry()`, `seqIDGen`, `spyMetrics`) + a real in-memory `*notifapp.EmailNotificationService` (`inmemory.NewEmailNotificationRepository`). 10 tests: TC-Shadow-01..05 (dual-send + shadow-sink-only delivery + idempotency-collision -> "mismatch" + empty-shadow-recipient skip + nil-notificationService no-op), TC-Rollback-01/02 (default flags = byte-identical legacy-only; `outboxEnabled` wins over `shadowMode` with no dual-send), a Case-C regression test, and two CF-12 closure tests Ã¢â‚¬â€ one functional (asserts a `context.WithValue` marker placed on the live `ctx` is observed synchronously by both the legacy `DeliveryAdapter.Send` and the durable pipeline's `TemplateRegistry.Resolve`, which a detached `context.Background()` goroutine could never see) and one static (`os.ReadFile` + `strings.Index` asserting zero occurrences of `context.Background()`/`dispatchNotificationAsync` in `internal/adhoc/app/service.go`)
- affected repos/files/modules:
  - `cobo_iam_services/internal/platform/config/config.go`, `configs/config.example.env`
  - `cobo_iam_services/internal/adhoc/infra/notification/notifier.go`
  - `cobo_iam_services/internal/adhoc/infra/notification/notifier_test.go` (new)
  - `cobo_iam_services/internal/adhoc/app/service.go`, `internal/adhoc/app/contracts.go`, `internal/adhoc/app/service_test.go`
  - `cobo_iam_services/internal/adhoc/observability/metrics.go`
  - `cobo_iam_services/internal/httpserver/server.go` (DI wiring: conditional `EmailNotificationService` construction on `pool != nil && outboxSQL != nil`, `adhocMetrics` moved before `proposalNotifier` construction, `adhocnotif.New(...)` updated to the new 11-arg signature)
- contracts/behaviors/constraints/decisions:
  - Behavior matrix verified end-to-end by tests: Case A (`shadow=false,outbox=false`) legacy-only byte-identical; Case B (`shadow=true,outbox=false`) dual pipeline Ã¢â‚¬â€ legacy to real recipient (system of record), durable to `shadowRecipient` only, never duplicating to the real user; Case C (`shadow=false,outbox=true`) durable-only, real recipient; Case D (both on) Ã¢â‚¬â€ OutboxEnabled wins, durable-only, no dual-send, no shadow metric
  - did NOT touch `internal/notification/...`, `cmd/worker/...`, `internal/platform/outbox/...`, `EmailDispatchOutboxEventType`, `DeliveryMessage`, `email_notifications` schema; no new migration, no new event type, no new worker registration
  - `sendLegacyEmail` is a byte-identical extraction of the pre-Batch-2 `sendEmail` body Ã¢â‚¬â€ guarantees Case A is unchanged
  - rewrite touches ONLY the `to` argument passed into `dispatchDurable` for the shadow branch Ã¢â‚¬â€ `recipientMembershipID`, the idempotency key, and `SourceAggregateID`/audit trail fields are never rewritten
- build/verification result:
  - `go build ./...` passed
  - `go vet ./...` passed
  - `gofmt -l` clean across all 6 touched/added files (1 file needed `gofmt -w` after edits Ã¢â‚¬â€ `service_test.go`)
  - `go test -race ./internal/adhoc/...` passed Ã¢â‚¬â€ all packages pass (`app`, `infra/disclosure` [no tests], `infra/mysql`, `infra/notification`, `observability`, `transport/http`); all 10 new notifier tests pass individually (`-run "TestShadowMode|TestRollback|TestRegression_Outbox|TestCF12" -v`)
  - `go test -race ./internal/notification/... ./internal/httpserver/...`: 4 pre-existing, unrelated failures confirmed out of scope Ã¢â‚¬â€ `TestDispatchEmail_SanitisesAllSensitiveVars` (documented in the Batch 2A entry above as failing identically on baseline `90b3db5`, a `support_email` template-fixture mismatch unrelated to adhoc), and 3 `httpserver` integration tests (`TestIntegration_platformCMSPrefix_dashboardCollectionsEntries` Ã¢â‚¬â€ 401 SESSION_EXPIRED; `TestIntegration_disclosureTypeCatalog_adminUpsertAndVersioning` / `...FixedDateWarnOnlyTimezone` / `...FixedDateMoveNextWorkingDay` Ã¢â‚¬â€ `template_category must be one of [periodic, irregular]` validation drift) Ã¢â‚¬â€ none reference `adhoc`/notifier code or any file this batch touched (verified via `grep -l adhoc` returning no matches and `git diff config.go` showing only additive `Adhoc*` fields)
- remaining gaps/risks/next steps:
  - the 4 pre-existing failures above should be triaged separately (template fixture `support_email` var; CMS dashboard session/auth fixture; `template_category` enum/test-data drift) Ã¢â‚¬â€ out of this batch's scope per "khÃƒÂ´ng tÃ¡Â»Â± mÃ¡Â»Å¸ rÃ¡Â»â„¢ng scope"
  - `cobo_adhoc_email_shadow_total` "mismatch" can only fire on a genuine idempotency-key collision (two different notification intents producing the same `<event_type>.<proposalID>.<recipientMembershipID>` key) Ã¢â‚¬â€ by construction this should be vanishingly rare; the metric is a correctness tripwire, not an expected-traffic signal
  - Docker rebuild not run in this sandbox (no `docker` binary available Ã¢â‚¬â€ consistent with the Batch 2A entry's note); recommend a fresh build + smoke test in staging before flipping `EMAIL_SHADOW_MODE`/`ADHOC_EMAIL_OUTBOX_ENABLED` to true anywhere

## 2026-06-11 - OPS-APPL-01 Ã¢â‚¬â€ Migration manifest 0091Ã¢â‚¬â€œ0095

- task type: implement (migration runner only)
- objective: `run_dev_migrations.sh` must auto-apply `0091`Ã¢â‚¬â€œ`0095`
- implemented:
  - Added 5 migrations to `MIGRATIONS` list after `0090`
  - Preflight drift: 0091/0093/0094 Ã¢â€ â€™ ledger-only when target columns exist
  - Ledger insert: `INSERT IGNORE` (0091 SQL self-records)
- test (DEV `88.216.208.0`): migrate exit 0; 0091 preflight; ledger 0090Ã¢â‚¬â€œ0095; rerun idempotent; backfill 19=19
- build: `docker compose -f docker-compose.dev.yml build api` exit 0
- prod pipeline: **NOT VERIFIED** (compose + deploy-dev.sh use same script)
- verdict: **OPS-APPL-01 DONE** Ã¢â‚¬â€ Gate 0 PASS

## 2026-06-12 - Deadline Engine V2 Ã¢â‚¬â€ Batch 5A Foundations & Adapters

- task type: implement
- objective: prepare additive-only infrastructure (adapter layer, feature flag confirmation, dual-compute/audit contracts, divergence classifier, wiring-readiness markers) so Batches 5B-5E can roll out the deadlineengine SoT (Source C) safely, with zero runtime/DB/output behavior change
- what was implemented:
  - `internal/disclosure/app/deadlineengine_adapter.go` (NEW): `DeadlineEngineAdapter` interface + `deadlineEngineAdapter` impl + `nonTradingDayCheckerAdapter`; maps existing `TemplateDeadlineConfig`/`TemplateApplicabilityRules`/`CompanyDeadlineContext`/`PeriodicCycleContext` into `deadlineengine.ResolveDeadline(...)` inputs Ã¢â‚¬â€ delegates 100%, no logic copy-pasted from `deadline_calculator.go`
  - `internal/disclosure/app/deadlineengine_dualcompute.go` (NEW): `DeadlineComparison`, `DeadlineAudit`, `ClassifyDivergence`, divergence constants `DivergenceNone/DateShift/MonthShift/YearlyRollback`
  - `internal/disclosure/app/deadlineengine_adapter_test.go` + `deadlineengine_dualcompute_test.go` (NEW): 17 tests, 100% coverage of both new files
  - `READY_FOR_5B` comment-only markers added (no behavior change) to:
    - `internal/disclosure/app/deadline_calculator.go` (`calculatePeriodic` Ã¢â‚¬â€ Portal Preview / Deadline Hint, Source A)
    - `internal/disclosure/app/periodic.go` (`seedPeriodicCycles` Ã¢â‚¬â€ Periodic Worker, Source B)
    - `internal/disclosure/app/service.go` (`CreateRecord` Ã¢â‚¬â€ Manual Create, client-supplied planned_date)
    - `internal/reminder/infra/mysql/repository.go` (`MaterializeDueOccurrences` Ã¢â‚¬â€ Reminder)
    - `internal/disclosure/app/deadlineengine/hint.go` (`FormatHintLabelVI` Ã¢â‚¬â€ Deadline Hint, already wired into `Resolution.HintLabelVI`, not yet rendered anywhere)
  - confirmed Phase C already satisfied: `DEADLINE_ENGINE_V2` flag exists since Batch 2 (`internal/platform/config/config.go`, default `false`, unused in runtime) Ã¢â‚¬â€ no new flag code needed
- affected repos/files/modules: `cobo_iam_services` Ã¢â‚¬â€ `internal/disclosure/app/` (2 new impl + 2 new test files, 3 comment-only edits), `internal/reminder/infra/mysql/repository.go` (comment-only), `internal/disclosure/app/deadlineengine/hint.go` (comment-only)
- contracts/behaviors/constraints/decisions:
  - Adapter, dual-compute, audit, and divergence types are new exported surface with **zero callers outside their own test files** Ã¢â‚¬â€ not wired into any runtime path
  - `DivergenceNone`/`DateShift`/`MonthShift`/`YearlyRollback` semantics: equal dates Ã¢â€ â€™ NONE; different year Ã¢â€ â€™ YEARLY_ROLLBACK (RK-E09); same year different month Ã¢â€ â€™ MONTH_SHIFT; same year/month different day Ã¢â€ â€™ DATE_SHIFT
  - No DB migration, no event publish, no metric emission, no `planned_date`/`cycle_start` recompute anywhere in this batch
- build/verification result:
  - `go build ./...` => exit 0
  - `go test ./internal/disclosure/app/... -run 'DeadlineEngineAdapter|Divergence|DeadlineComparison|DeadlineAudit|NonTradingDayChecker' -v` => 17/17 PASS
  - `go tool cover -func` on new files => 100.0% all functions
  - `go test ./...` => 2 pre-existing FAILs (`internal/httpserver`, `internal/companyaccess/transport/http`), confirmed via `git stash`/`git stash pop` to exist identically on base commit `8fb5fae` before this batch Ã¢â‚¬â€ unrelated to Batch 5A
  - `gofmt -l` clean on all new files
- remaining gaps/risks/next steps:
  - Batch 5B: wire `DeadlineEngineAdapter.ResolveDeadline` into `calculatePeriodic` (Portal Preview) and `seedPeriodicCycles` (Periodic Worker, `cycleCtx=nil` per I-21) behind `DEADLINE_ENGINE_V2`
  - Batch 5C: render `FormatHintLabelVI`/`Resolution.HintLabelVI` in Portal response
  - Batch 5D: enable `DeadlineComparison`/`DeadlineAudit` dual-compute path for shadow comparison before cutover
  - full report: `docs/ai-cache/deadline-engine-batch5a-implementation-2026-06-12.md`
- verdict: **BATCH 5A COMPLETE Ã¢â‚¬â€ READY FOR 5B**

## 2026-06-12 - Deadline Engine V2 Ã¢â‚¬â€ Batch 5B: Shadow Runtime Wiring

- task type: implement
- objective: wire Deadline Engine V2 (Source C) as a **shadow-compute-only** path alongside the Old Runtime (Source A/B) at the three runtime entry points Ã¢â‚¬â€ Portal Preview, Periodic Worker seed tick, Manual Create Ã¢â‚¬â€ and emit a structured divergence-audit log, with zero impact on DB writes, API responses, or worker output. No cutover.
- what was implemented:
  - `internal/disclosure/app/deadlineengine_shadow.go` (NEW): `shadowSampler` (1-in-100 deterministic sampling for `NONE` divergence), `formatShadowDate`/`parseShadowDate`, `logDeadlineEngineShadow` (Phase E/F: exact `deadline_engine_shadow` JSON event, `slog`, no DB/Kafka/metrics), `deadlineEngineShadowRunner` with three methods:
    - `portalPreview` (Phase A): shadow `adapter.ResolveDeadline` alongside `DeadlineCalculator.CalculateDeadlineSummary` in `GetTypeDetail`; compares old `summary.StartDate`/`DeadlineDate` vs `res.ResolvedT0`/`PlannedDate`; result never used.
    - `periodicWorker` (Phase B): shadow-compute after `computeCycleLabelAndStart` in `seedPeriodicCycles`; compares `oldCycleStart`/`oldDueDate` (Source B, written to DB) vs `newResolution.T0`/`PlannedDate` (never persisted); deliberately passes zero-value `CompanyDeadlineContext` (Source B is anchor-blind, so this isolates the Source B vs Source C algorithm divergence Ã¢â‚¬â€ RK-E09 Ã¢â‚¬â€ without R-C confounding).
    - `manualCreate` (Phase C): after `repo.Create` succeeds in `CreateRecord`, shadow `adapter.ResolveDeadline` and compares client `planned_date` vs `res.PlannedDate`. Audit-only; request/record never mutated. Since Manual Create has no "old cycle_start", `DivergenceClass` is classified on **due-date** divergence (`ClassifyDivergence(oldDue, res.PlannedDate)`); `OldCycleStart==NewCycleStart==res.ResolvedT0` for log context only.
  - `internal/disclosure/app/service.go`: new `shadowRunner *deadlineEngineShadowRunner` + `deadlineEngineV2Shadow bool` fields, `WithDeadlineEngineV2Shadow` option, constructed in `NewService` after options applied; wired into `GetTypeDetail`, `CreateRecord` (via `shadowManualCreate`), `SeedPeriodicCycles`. All call sites guarded by `s.shadowRunner.enabled` Ã¢â‚¬â€ zero extra repo/adapter calls when disabled (default).
  - `internal/disclosure/app/periodic.go`: `seedPeriodicCycles` gained trailing `shadow *deadlineEngineShadowRunner` param; one call `shadow.periodicWorker(...)` added after `dueDate` computed, before `repo.UpsertPeriodicCycle`.
  - `internal/platform/config/config.go`: new `DeadlineEngineV2Shadow bool`, env `DEADLINE_ENGINE_V2_SHADOW`, default `false`.
  - `internal/httpserver/server.go`: passes `cfg.DeadlineEngineV2Shadow` into `disclosureapp.WithDeadlineEngineV2Shadow(...)`.
  - Tests (NEW):
    - `internal/disclosure/app/deadlineengine_shadow_test.go` Ã¢â‚¬â€ unit tests for sampler, log format (Phase E JSON shape via `slog.NewJSONHandler`), sampling rules (Phase F), date helpers, and all three runner methods (success, disabled, nil-runner, non-periodic/nil-rules skip, adapter-error-swallowed) via `fakeShadowAdapter`.
    - `internal/disclosure/app/deadlineengine_shadow_integration_test.go` (`package app_test`) Ã¢â‚¬â€ proves Behavior Before == Behavior After using `internal/disclosure/infra/inmemory.Repository`: `SeedPeriodicCycles` DB writes identical (shadow on/off), `GetTypeDetail` `DeadlineSummaryDTO`/full DTO identical, `CreateRecord` returned `RecordDTO` identical Ã¢â‚¬â€ with shadow actually executing and logging real `DATE_SHIFT` divergence (uses a `noHolidaysProvider` test double to avoid missing-fixture errors so the shadow path runs for real, not short-circuited).
    - `internal/platform/config/deadline_engine_v2_shadow_test.go` Ã¢â‚¬â€ `DEADLINE_ENGINE_V2_SHADOW` defaults false / explicit true.
- affected repos/files/modules: `cobo_iam_services` only Ã¢â‚¬â€ `internal/disclosure/app/{deadlineengine_shadow.go,deadlineengine_shadow_test.go,deadlineengine_shadow_integration_test.go,service.go,periodic.go}`, `internal/platform/config/{config.go,deadline_engine_v2_shadow_test.go}`, `internal/httpserver/server.go`.
- contracts/behaviors/constraints/decisions:
  - `planned_date`, `cycle_start`, `periodic_cycles`, `disclosure_records`, API response, reminder behavior, portal behavior, worker behavior Ã¢â‚¬â€ all unchanged (proven via integration tests, same on/off output).
  - `DEADLINE_ENGINE_V2` remains unused/untouched (Batch 2). New flag `DEADLINE_ENGINE_V2_SHADOW` (default `false`) gates the entire shadow path; after this batch's deploy, flag stays `false`.
  - Shadow compute never errors out to the caller Ã¢â‚¬â€ adapter errors are logged via `slog.WarnContext("deadline_engine_shadow_error", ...)` and swallowed.
  - Log format (Phase E, exact): `{"event":"deadline_engine_shadow","company_id":...,"type_id":...,"divergence_class":...,"old_cycle_start":...,"new_cycle_start":...,"old_due_date":...,"new_due_date":...}`. Sampling (Phase F): `NONE` Ã¢â€ â€™ 1-in-100; `DATE_SHIFT`/`MONTH_SHIFT`/`YEARLY_ROLLBACK` Ã¢â€ â€™ always logged.
- build/verification result:
  - `go build ./...` => exit 0
  - `go test ./internal/disclosure/app/...` and `./internal/platform/config/...` => all PASS (incl. new shadow unit + integration tests)
  - `deadlineengine_shadow.go` coverage: 98.4% (`go tool cover`)
  - `go test ./...` => only pre-existing failures in `internal/companyaccess/transport/http` (`TestCreateSelfServiceCompany_FeatureFlagOff`) and `internal/httpserver` (4 platform-CMS/template_category tests) Ã¢â‚¬â€ confirmed identical on `git stash` (pre-existing on `phase-300526`, unrelated to Batch 5B).
  - `gofmt -l` clean on all new/modified Batch 5B files (pre-existing gofmt findings in `service.go`/`config.go` unrelated to this batch's lines).
- remaining gaps/risks/next steps:
  - Batch 5C: cutover planning Ã¢â‚¬â€ none of Worker/Portal/Reminder cutover, `planned_date` rewrite, periodic cycle rewrite, DB migration, backfill, or remediation were performed (non-goals, by design).
  - `DEADLINE_ENGINE_V2_SHADOW` remains `false` after deploy; enabling it in a real environment requires real `configs/non_trading_days/*.json` fixtures for the target years (confirmed needed during integration test construction).
- verdict: **BATCH 5B COMPLETE Ã¢â‚¬â€ READY FOR 5C**

---

## Batch 5C.1 + 5D Ã¢â‚¬â€ Deadline Engine V2 Input Contract Fix & Shadow Divergence Verification (2026-06-12)

- scope: 5C.1 (input contract resolution, no cutover) + 5D (shadow divergence verification on DEV). 5E (cutover) explicitly NOT performed Ã¢â‚¬â€ see verdict.
- **5C.1 decisions (locked)**:
  1. `frequency_unit: "week"` (only `bao-cao-tan-suat`, DEV) Ã¢â€ â€™ **Option C (future backlog)**. `DeriveDeadlineBehavior` (resolve_t0.go) only supports `monthly|quarterly|yearly`; `ListActivePeriodicTypes` SQL already excludes `week` from the periodic worker. Adding weekly-cycle semantics to Source C is a scope-expanding engine change requiring product sign-off Ã¢â‚¬â€ out of scope for 5C.1/5D. The resulting `deadline_engine_shadow_error` (`unsupported frequency_unit: "week"`) for this single template is **expected and non-blocking** (swallowed by design, isolated to one template, old runtime unaffected).
  2. `deadline_days must be > 0` (affected ALL periodic-eligible templates, root cause of shadow success=0 in prior 5C run) Ã¢â€ â€™ **Option B (adapter mapping bug, fixed)**. DEV `applicability_rules_json` rows (authored before the V2 `use_structure_deadline` flag existed) populate `deadline_by_structure` (required by CMS validation E01-E06 for periodic templates) but leave `deadline_days=0` and `use_structure_deadline=false`. `ResolveEffectiveN` (resolve_n.go) requires `DeadlineDays>0 || UseStructureDeadline=true`, so it always returned `ErrInvalidDeadlineDays`. Legacy Source A/B (`applicability.ResolveDeadlineDays`) consults `deadline_by_structure` unconditionally Ã¢â‚¬â€ no opt-in flag. Fix: added `normalizeRulesForEngine()` in `internal/disclosure/app/deadlineengine_adapter.go` Ã¢â‚¬â€ when `DeadlineDays<=0 && !UseStructureDeadline && len(DeadlineByStructure)>0`, returns a copy with `UseStructureDeadline=true` before calling `deadlineengine.ResolveDeadline`. Read-only normalization, no DB write, no mutation of caller's rules. Covered by new test `TestDeadlineEngineAdapter_ResolveDeadline_LegacyDeadlineByStructureFallback`.
- **Files changed**:
  - `internal/disclosure/app/deadlineengine_adapter.go` Ã¢â‚¬â€ added `normalizeRulesForEngine()`, applied to `in.Rules` in `ResolveDeadline`.
  - `internal/disclosure/app/deadlineengine_adapter_test.go` Ã¢â‚¬â€ added `TestDeadlineEngineAdapter_ResolveDeadline_LegacyDeadlineByStructureFallback`.
  - `cmd/worker/main.go` Ã¢â‚¬â€ fixed Batch 5B wiring gap: `disclosureapp.WithDeadlineEngineV2Shadow(cfg.DeadlineEngineV2Shadow)` was constructed by `internal/httpserver/server.go` but **not** by the worker's `disclosureSvc` (used for `SeedPeriodicCycles`/periodic worker shadow). Added the missing option.
- **Build/test**: `go build ./...` clean. `go test ./internal/disclosure/app/... ./internal/platform/config/...` pass. `go test ./...`: only pre-existing unrelated failure `TestCreateSelfServiceCompany_FeatureFlagOff` (internal/companyaccess/transport/http, self-service company creation flag Ã¢â‚¬â€ untouched by this batch). `gofmt -l` on edited files: clean (repo-wide `gofmt -l` noise is pre-existing/unrelated).
- **Deploy**: `make deploy-be` to DEV (88.216.208.0). `cobo-iam-api`/`cobo-iam-worker` recreated, healthy (`/healthz`, `/readyz` ok). Flags after deploy: `DEADLINE_ENGINE_V2=false`, `DEADLINE_ENGINE_V2_SHADOW=true` (both containers) Ã¢â‚¬â€ unchanged from pre-deploy.
- **5D shadow verification (DEV)**:
  - Portal Preview (`GET /api/v1/disclosure-types/{type_id}`, company `08f59da2-...`):
    - `bao-cao-tai-chinh-quy-1` (DeadlineMode=PERIODIC, frequency_unit empty) Ã¢â€ â€™ **SUCCESS**, `divergence_class=NONE` (cycle_start 2026-06-12 both), `old_due_date=2026-07-09` vs `new_due_date=2026-07-11` (2-day due-date divergence logged for analysis Ã¢â‚¬â€ `ClassifyDivergence` only compares cycle_start, by design/Batch 5A).
    - `bao-cao-tan-suat` (frequency_unit=week) Ã¢â€ â€™ `shadow_error`: `unsupported frequency_unit: "week"` Ã¢â‚¬â€ expected per 5C.1 Decision 1, non-blocking.
  - Periodic Worker (`PERIODIC_SEEDING_ENABLED=true` temporarily on worker only, `.env` backed up as `.env.bak.5d_preflip.<ts>`, reverted to `false` after evidence collected; `periodic_cycles` count unchanged 27Ã¢â€ â€™27, `disclosure_records` unchanged 83/57 during this window):
    - `bao-cao-tai-chinh-quy-2` (frequency_unit=quarterly), company `08f59da2-...` Ã¢â€ â€™ **SUCCESS**, `divergence_class=NONE`, `old_due_date=2026-04-30 == new_due_date=2026-04-30` (full parity).
  - Manual Create (`POST /api/v1/disclosures`, DEV-only labeled test record `[DEV-ONLY 5D SHADOW SMOKE TEST]`, record_id `019ebac1-d199-7773-89cf-5cca89835baa`, type `bao-cao-tai-chinh-quy-1`, `planned_date=2026-07-09` submitted and persisted unchanged Ã¢â‚¬â€ **no cutover, client value used as-is**) Ã¢â€ â€™ **SUCCESS**, `divergence_class=DATE_SHIFT`, `old_due_date=2026-07-09` (client) vs `new_due_date=2026-07-11` (engine).
  - Totals: 3 `deadline_engine_shadow` events (NONE=2, DATE_SHIFT=1, MONTH_SHIFT=0, YEARLY_ROLLBACK=0), `shadow_error=1` (week, documented non-blocking per Decision 1). No panic/fatal in API or worker logs.
  - 5D required success conditions met: Ã¢â€°Â¥1 periodic-worker success Ã¢Å“â€œ, Ã¢â€°Â¥1 portal-preview success Ã¢Å“â€œ, Ã¢â€°Â¥1 manual-create success Ã¢Å“â€œ.
- **Behavior/data safety**: `periodic_cycles` 27Ã¢â€ â€™27 (unchanged). `disclosure_records` 83/57 Ã¢â€ â€™ 84/58 (the +1 is the labeled DEV-only smoke-test record; `planned_date` stored = client-submitted value, not engine value). Final DEV state restored: `DEADLINE_ENGINE_V2=false`, `DEADLINE_ENGINE_V2_SHADOW=true`, `PERIODIC_SEEDING_ENABLED=false` on both containers, health OK.
- **5E (cutover) Ã¢â‚¬â€ NOT performed.** Blocked by explicit stop condition (Part C Ã‚Â§3.4): `RecordPayload` (`internal/disclosure/app/contracts.go`) carries only `planned_date`, **no T0 input field** Ã¢â‚¬â€ Manual Create cutover (server recomputing `planned_date` from `T0 + N`) cannot be implemented without a UI/API contract change to carry T0. Additionally, the DATE_SHIFT divergence found on `bao-cao-tai-chinh-quy-1` (consistent 2-day due-date difference, Source A vs Source C N/day-type) needs product/PO review before any V2-derived date is exposed in Portal/Manual Create Ã¢â‚¬â€ applies to cutover scopes 3.4 and 3.5.
- **remaining gaps / next steps**:
  - Product decision needed: reconcile the 2-day due-date divergence (likely `deadline_day_type` default mismatch Ã¢â‚¬â€ Source A vs Source C `calendar`/`working` default, or inclusive-day-counting difference) before any cutover that changes user-visible dates.
  - API/contract change needed before Manual Create cutover (3.4): add a T0 input field to `RecordPayload`/`CreateRecordRequest`, or get explicit PO confirmation that `planned_date` itself may be treated as T0 (spec forbids silently assuming this).
  - `bao-cao-tan-suat` (`frequency_unit=week`) remains out of V2 scope (Decision 1) Ã¢â‚¬â€ revisit if/when weekly periodic templates become a product priority.
  - DEV-only smoke-test record `019ebac1-d199-7773-89cf-5cca89835baa` (`[DEV-ONLY 5D SHADOW SMOKE TEST]`, type `bao-cao-tai-chinh-quy-1`, company `08f59da2-...`) left in place Ã¢â‚¬â€ safe to delete (status=Draft).
- verdict: **PARTIAL COMPLETE Ã¢â‚¬â€ BLOCKED BEFORE CUTOVER** (5C.1 PASS, 5D PASS, 5E blocked per stop conditions above)

## 2026-06-18 - Invite user with title assignment

- task type: cross-repo implement (backend invite flow)
- objective: support assigning a membership title when inviting a new employee from Admin Center
- implemented:
  - extended `InviteUserRequest` with optional `title_id`
  - decoded `title_id` in `POST /api/v1/admin/users/invite`
  - assigned title during invite for both:
    - new invited users with a freshly created membership
    - existing active users added into a new company membership
  - enriched in-memory membership listing with titles so service tests can assert the new behavior
  - added service test covering invite + title assignment
- affected files:
  - `internal/companyaccess/app/admin.go`
  - `internal/companyaccess/app/admin_service.go`
  - `internal/companyaccess/app/admin_service_test.go`
  - `internal/companyaccess/infra/inmemory/admin_repository.go`
  - `internal/companyaccess/transport/http/admin_handler.go`
- constraints and decisions:
  - additive contract only; no migration needed
  - reused existing membership-title persistence path instead of inventing a separate invite-specific table
  - title assignment remains optional and scoped by the existing membership/company validation in repository layer
- verification:
  - run `go test ./internal/companyaccess/...` and `go build ./...`
  - try Docker API build per repo workflow; if unavailable, mark blocked by environment
- related cache:
  - `docs/ai-cache/invite-user-title-assignment-integration-summary-2026-06-18.md`

## 2026-06-18 - Direct employee creation with membership bootstrap

- task type: cross-repo implement (backend direct-create flow)
- objective: let Admin Center create active employees directly and assign membership metadata in one request
- implemented:
  - extended `POST /api/v1/admin/users` to accept:
    - `role_id`
    - `role_code`
    - `permissions[]`
    - `department_id`
    - `title_id`
  - assigned the initial role inside the same DB transaction as user + membership creation
  - applied direct permissions, default workflow-read permission, department, and title after membership creation
  - fixed company-scope department handling so an explicitly selected `department_id` is no longer dropped
  - fixed `GET /api/v1/admin/companies/{company_id}/memberships` to return flat `items[]` + `total` and pass through `page` / `page_size`
  - added service and handler regression tests covering the new flow
- affected files:
  - `internal/companyaccess/app/admin.go`
  - `internal/companyaccess/app/admin_service.go`
  - `internal/companyaccess/app/admin_service_invite_scope.go`
  - `internal/companyaccess/app/admin_service_test.go`
  - `internal/companyaccess/infra/inmemory/admin_repository.go`
  - `internal/companyaccess/infra/mysql/admin_repository.go`
  - `internal/companyaccess/transport/http/admin_handler.go`
  - `internal/companyaccess/transport/http/admin_handler_memberships_test.go`
- constraints and decisions:
  - reused the existing create-user endpoint instead of creating a second admin-staff contract
  - kept all new request fields optional to avoid breaking existing callers
  - preserved backward compatibility with the invite/title work completed earlier in the day
- verification:
  - PASS: targeted create-user service tests using workspace-local `GOCACHE`
  - PASS: targeted memberships handler test for flat payload + pagination
  - default Go build cache path was blocked by machine permissions, so verification used a workspace cache override
- related cache:
  - `docs/ai-cache/create-employee-direct-flow-integration-summary-2026-06-18.md`

## 2026-07-06 - Staff Invite T1/T2 migration ledger fix + QA closeout

- task type: deploy fix + QA verification
- objective: Ensure `0113_department_memberships_focal` applies on DEV; support DB verification for focal QA
- implemented: Added `0113_department_memberships_focal.up.sql` to `migrations/run_dev_migrations.sh`; applied on DEV via `push-migration.sh`
- verification: Column `department_memberships.is_department_focal` present on DEV; API/DB focal rows `is_department_focal=1` for invite + create paths
- evidence: `../cobo_web_design/docs/ai-cache/staff-invite-t1-t2-implementation-2026-07-06/`

## 2026-07-06 - Staff Invite T3/T4 permission validation hardening

- task type: backend validation + tests
- implemented: `validateEnterpriseInvitePermissions` denies read/rbac/system/cms/platform direct grants on invite/create
- tests: `TestInviteUser_RejectsCMSPrefixPermission`, `TestInviteUser_RejectsRbacManageDirectPermission`, `TestInviteUser_AcceptsWorkflowAndAdHocPermissions`, `TestCreateUser_RejectsRbacManageDirectPermission`, `TestInviteUser_RejectsWorkflowReadInPayload`
- evidence: `../cobo_web_design/docs/ai-cache/staff-invite-t3-t4-implementation-2026-07-06/`



## 2026-07-29 - Phase 12.6A legal basis inventory (Docker DEV read-only)

- task type: backend tooling + DEV inventory (read-only)
- objective: Phase 12.6A Groups A–E inventory + in-memory dry-run using Docker DEV compose DB config (not MYSQL_READONLY_DSN)
- implemented/discovered:
  - SQL allowlist interceptor (`ValidateReadOnlySQL` + `AllowlistConnector`)
  - CLI `--docker-dev` → published `127.0.0.1:3306` / db `cobo_iam` / masked user; app credential allowed only with READ ONLY tx + allowlist
  - Live inventory: total=6, A=6, B=C=D=E=0; dry-run WRAP_LEGACY_FLAT×6; idempotent; mutations=0
  - DEV missing `is_released` (0122); probe + approximate via active version pointer
- affected: `cmd/legal-basis-inventory`, `internal/disclosure/app/legal_basis_inventory/*`, plan evidence under `docs/ai-cache/legal-basis-contract-alignment-plan-2026-07-29/`
- constraints: no `--apply`; no Compose edits; no container recreate for inventory; no Phase 12.6B
- verification: `go test ./internal/disclosure/app/legal_basis_inventory/` PASS; tool build PASS; `docker compose -f docker-compose.dev.yml build api` EXIT 0; verdict **PASS_READ_ONLY_DRY_RUN**
- remaining: human review dry-run preview before 12.6B; apply migration 0122 on DEV separately if product needs `is_released`

## 2026-07-29 - Phase 12.6B-Plan Controlled DEV backfill (docs only)

- task type: planning / docs
- objective: lock exact 6-record allowlist, dataset boundary ALL_6, snapshot/CAS/txn/rollback/runbook; close 12.6A dual verdict
- implemented: `phase-12-6b-*` design pack + 12.6A scope-exception; no apply tool; no DB write; no Docker build
- dataset: 6 Group A GLOBAL active v1; WRAP_LEGACY_FLAT; flat-after-wrap=OD-7 projection
- verdict: **BACKFILL_PLAN_READY**
- remaining: Approvals 3–4 + explicit mutate phrase before implementation/execution

## 2026-07-29 - Phase 12.6B-I guarded backfill tooling

- task type: backend implement (no DEV mutate)
- objective: implement fail-closed backfill/rollback/verify tooling for exact 6-record allowlist
- implemented: legal_basis_backfill package + cmds; inventory SQL safety committed; updated_by PRESERVE locked
- verification: targeted/disclosure PASS; full suite pre-existing unrelated fails; vet/build PASS; DEV writes=0
- verdict: TOOL_READY_FOR_CONTROLLED_EXECUTION
- next: Phase 12.6B-E after explicit mutate approval + SQL wiring
