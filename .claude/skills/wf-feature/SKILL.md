---
name: wf-feature
description: Workflow end-to-end để xây một feature Cobo (FE, BE hoặc cả hai repo) - brief, thiết kế và chốt contract (gate người dùng duyệt), migration, backend, frontend, test, E2E, review song song bằng subagent, premerge, Postman và ai-cache. Dùng khi được yêu cầu làm feature/chức năng mới, mở rộng API/màn hình, hoặc lệnh /feature.
---

# Workflow: Feature Delivery

Each phase produces an artifact in `docs/ai-cache/<feature-slug>-YYYY-MM-DD/`
(numbered files). Do not skip a gate; if the user explicitly waives one,
record that in the artifact.

## Phase 0 - Intake  → `00-brief.md`
- Restate the goal, users/roles, success criteria, repos affected.
- Load context per `cobo-task-workflow` Step 2 (ai-cache README, related
  ai-cache files, specs, code).
- Domain check: CBTT rules (`cobo-domain-cbtt`), role boundary
  (`cobo-admin-role-guard`).
- Size it: **small** (one repo, no API/schema change) → go to Phase 3/4
  directly after a short plan; **normal** → all phases.

## Phase 1 - Design  → `01-design.md`
- `system-design-feature` (cross-repo) or the narrow design part of
  `backend-feature-delivery` / `frontend-feature-slice`.
- Write or update the spec with `spec-authoring` when the feature spans
  several PRs.

## Phase 2 - Contract lock  → `02-contract.md`  **GATE A**
Contents: endpoints, request/response, error codes, permissions, company
scope, idempotency, flags, migration list, compatibility matrix
(`api-compatibility-rolling-deploy`), test matrix.
**Stop and ask the user to approve the contract** before writing
implementation code. Proceed without approval only if the user said so.

## Phase 3 - Backend (if in scope)  [iam]
Order: migration (`backend-db-migration-safe`, add to
`run_dev_migrations.sh`) → repository (MySQL + in-memory) → service →
handler (`backend-api-contract`, auth/permission check in handler) →
worker/outbox (`backend-worker-idempotent`) → flags/config/compose.
After each slice: `go test ./<pkg>/...`; at the end `go vet ./...`,
`go test ./...`, `docker compose -f docker-compose.dev.yml build api`.

## Phase 4 - Frontend (if in scope)  [web]
`frontend-feature-slice` → `frontend-route-screen` (routing) →
`frontend-ui-ux-quality` (+ `taste-frontend-cobo` for visual work) →
`frontend-ui-state-guard` → `frontend-test-regression`.
Checks: `npm run lint`, focused `npx vitest run`, `npm run build`,
`npm run check:mojibake`.

## Phase 5 - Integration  → `05-verification.md`
- Contract tests on both sides; Postman updated (`api-postman-sync`).
- Local full stack (`local-dev-stack`) + smoke via `e2e-playwright-cobo`
  for user-visible flows (authorized environments only).

## Phase 6 - Parallel review  → `06-review.md`
Pick reviewers with the path map in `wf-risk-review` Phase 1 and launch them
in one message. Fix every CRITICAL/HIGH (or get explicit user acceptance),
then re-run only affected reviewers.

## Phase 7 - Premerge  **GATE B**
`premerge-system-review` on the full change. Do not report "done" with an
open Critical finding or a failing required check (use `BLOCKED:` honestly).

## Phase 8 - Close  → `99-summary.md`
`ai-cache-maintenance` summary; list follow-ups; no commit/push unless the
user asked for that exact operation.

## Completion report
```text
Feature:
Gates: A (contract) approved by/at:, B (premerge) result:
Files changed by repo:
Checks run and results / BLOCKED:
Review findings fixed / accepted:
Rollout: flags, deploy order, migrations:
Follow-ups:
```
