# SPEC: <Feature name>

- Date: YYYY-MM-DD
- Branch: <branch>
- Scope: <repos/modules>
- Status: draft | agreed | implemented
- Legal basis (if CBTT related): <TT96 Điều/khoản>

## 1. Objective
Business goal, users/roles, success criteria.

## 2. Current status
| Area | Exists (file path) | Missing |
|---|---|---|

## 3. Domain model and state transitions
Entities, invariants, transition table (from, event, guard, to, side effects).

## 4. Data model / migration
Tables/columns, indexes, migration files (NNNN), backfill, compatibility.

## 5. API contract
Per endpoint: method + path, permission, request, response, error codes
(`{"error":{"code","message","details"}}`), idempotency, pagination.

## 6. Frontend
Routes, guards (`routePermissionMatrix` / `RequirePlatformAccess`), screens,
state machine (loading/empty/error/forbidden/success), flags (`VITE_*`).

## 7. Security invariants / constraints
Tenant isolation, platform vs company admin, sensitive data, audit.

## 8. Failure modes
Slow/down dependencies, retries, duplicates, stale cache, mixed-version deploy.

## 9. Known gaps / open questions

## 10. Project structure
Files to add/change, grouped by repo.

## 11. Testing strategy
BE (Go tests), FE (Vitest/contract), E2E smoke, manual checks.

## 12. Rollout
Phase 0: contract lock (HARD STOP) → phases with CHECKPOINT commands → flags
→ cleanup.

## 13. Boundaries
- Always:
- Ask first:
- Never:
