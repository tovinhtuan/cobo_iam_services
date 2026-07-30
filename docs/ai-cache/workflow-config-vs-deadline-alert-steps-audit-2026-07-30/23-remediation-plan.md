# 23 — Remediation plan (stop before implementation)

## Phase A — Contract lock
Decide: CMS Workflow tab = global-only (document dual-SoT) OR must mirror effective/enterprise_workflow.

## Phase B — Source fix (after A)
If document-only: Option D/E + A empty-state.
If unify SoT: gated publish path for global from enterprise_workflow for **new** versions only.
Separate ticket: fix `GET .../instances/{id}/tasks` 500.

## Phase C — Tests
Unit: empty global + effective 4 steps messaging.
E2E: CMS banner + alert card current step + detail timeline when tasks fixed.

## Phase D — Local/browser QA
Exact QA type + one new type after any SoT change.

## Phase E — DEV deploy
Isolated FE (and API if tasks fix).

## Phase F — New QA record after config (only if unifying)
Compare old snapshot vs new; **no** mutate old.

## Phase G — Old/new comparison evidence

### Acceptance
- Documented SoT; code/order/actor on runtime match effective-at-materialize
- No stale hardcoded linear fallback as primary path
- Old-record immutable snapshot documented
- New records use correct active effective
- No duplicate tasks; no historical auto-mutation
