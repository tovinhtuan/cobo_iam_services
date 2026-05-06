# Reusable Task Updates

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
  - expanded integration test to assert audit feed contains expected write-action events after full entries→reviews→schedules flow
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
  - minor documentation update debt remains to align “stub/partial” notes with current live CMS state
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
  - recommended backend rule baseline for “template chuẩn”:
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
- objective: bổ sung thêm 3 loại template báo cáo trong disclosure catalog để CMS có thể tạo/chọn ngay: nghĩa vụ, tổ chức đại hội cổ đông, công bố thông tin về giao dịch
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
    - nghĩa vụ: `Định kỳ` / `group-001`
    - tổ chức đại hội cổ đông: `Định kỳ` / `group-001`
    - công bố thông tin về giao dịch: `Bất thường` / `group-002`
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
- objective: chuẩn hóa lifecycle template disclosure bằng matrix validation theo loại (periodic/irregular/custom), enum hóa field reference và bổ sung test matrix integration
- what was implemented/discovered:
  - applied running-stack migrations and verified catalog data:
    - `0014_disclosure_catalog_extra_types.up.sql` (3 loại mới)
    - `0015_disclosure_template_enums.up.sql` (chuẩn hóa enum + thêm `deadline_strategy`)
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
