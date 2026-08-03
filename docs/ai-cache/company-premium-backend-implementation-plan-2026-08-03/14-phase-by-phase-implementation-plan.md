# Phase-by-phase implementation plan

## Phase 0 — Contract approval gate
**Goal:** Lock SoT + semantics before code.  
**Steps:** Product signs Case B1 vs C; Security signs read authz; Backend signs schema/status/null.  
**Exit:** Written ADR/approval in ai-cache; open decisions closed or explicitly deferred with interim rules.  
**Risk:** Coding before gate → wrong badge semantics.  
**Rollback:** n/a.  
**Evidence:** signed checklist in handoff.

## Phase 1 — Domain/repository foundation
**Goal:** `companyplan.Reader` + entitlement adapter (and/or MySQL company_subscriptions).  
**Files:** see map rows Phase 1.  
**Steps:** types → interface → entitlement_reader → tests for max-tier mapping & omit Free.  
**Exit:** unit tests green; no HTTP yet.  
**Risk:** leaking entitlement as “paid”. Mitigate via `source` field.  
**Rollback:** delete new package.

## Phase 2 — API exposure
**Goal:** Additive `plan` on GetOwnCompany + `/me/companies`.  
**Files:** platform_company.go, admin_service.go, admin_options.go, server.go, me_handler.go.  
**Steps:** DI → attach on GetOwnCompany → batch attach on companies → consistency test.  
**Exit:** handler tests 200 with/without plan; old fields unchanged.  
**Risk:** N+1 on companies list.  
**Rollback:** stop wiring Reader (omit field).

## Phase 3 — Tests
**Goal:** Matrix in `15-test-strategy.md`.  
**Exit:** CI package tests pass for companyplan + owncompany + me companies.

## Phase 4 — Migration/fixture (Case C only)
**Goal:** table + DEV seed companies with known Premium/non-Premium.  
**Exit:** migrate up/down dry-run; fixtures documented.  
**Rollback:** down migration.

## Phase 5 — Backend DEV deploy (plan only)
`make deploy-be` or approved compose build api — **do not run in this task**.  
Verify API StartedAt change expected; FE old still works.

## Phase 6 — API smoke
curl GetOwnCompany + me/companies for Premium & Free tenants; IDOR negative; schema assert.

## Phase 7 — FE follow-up
Map types → company overview badge → remove personal Premium → switch tests → `make deploy-fe` → smoke.  
**Hard rule:** no fallback to `user.subscriptionTier`.
