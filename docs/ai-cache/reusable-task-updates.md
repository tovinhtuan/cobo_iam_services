# Reusable Task Updates

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
