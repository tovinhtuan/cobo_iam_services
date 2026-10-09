---
name: api-compatibility-rolling-deploy
description: Dùng khi đổi API giữa cobo_web_design và cobo_iam_services hoặc trước deploy - phát hiện breaking change về request/response schema, kiểu dữ liệu, enum, error format, nullable; backward compatibility, API versioning /v1, thứ tự deploy và tương thích FE/BE khác phiên bản khi rolling deployment.
---

# API Compatibility & Rolling Deploy

## Why this matters here
- FE and BE live in separate repos and deploy separately
  (`make deploy-be`, `make deploy-fe`; `deploy-fe` never recreates the API).
- Users keep SPA tabs open for hours: an **old bundle keeps calling the new
  API** after every backend deploy.
- A partially failed deploy can leave **new FE + old BE**.
- API and worker binaries may run different versions during a restart.
- MySQL migrations run before the new binary starts (old code sees new schema).

## Compatibility rules

Safe (additive):
- New endpoint; new optional request field with server default; new response
  field; new error code that old clients treat as generic error.

Breaking (need a plan):
- Remove/rename a field, change its type (string↔number, object↔array),
  change nullability, change units/format (date, money, timezone).
- New **required** request field.
- New enum value in a response consumed by a `switch` without default.
- Changing error `code` strings or HTTP status for an existing condition.
- Changing pagination, sort default, or envelope (`{"error":...}`,
  `{"data","meta"}`).
- Tightening validation on existing inputs.

## Plan for a breaking change (expand → migrate → contract)
1. **Expand**: BE accepts old and new shapes, returns both fields if needed.
2. Deploy BE. Old FE keeps working.
3. **Migrate**: FE switches to the new field/value; deploy FE.
4. Wait at least one session lifetime / until old tabs are gone (or force
   reload via version check, see `cache-versioning-review`).
5. **Contract**: remove the old field in a later release.
Use a new path version (`/api/v2/...`) only when expand/contract is impossible.

## Frontend tolerance rules (cobo_web_design)
- Unknown enum → explicit fallback label/state, never crash or blank screen.
- Missing optional field → defined default; don't assume presence.
- Parse errors via `ApiError.payload.error.code`; unknown code → generic
  message, keep `message` from server only if safe to show.
- Contract tests (`*Api.contract.test.ts`) cover old and new shapes during the
  transition.

## Backend rules (cobo_iam_services)
- Error codes in `internal/platform/errors` are public API: never rename.
- Response structs: add fields with `omitempty` only when absence is
  meaningful; keep JSON names stable.
- Feature flags: new behavior behind env flag until both sides are deployed.
- Migrations follow `backend-db-migration-safe` [iam] (old binary must run on the
  new schema).
- Worker/outbox payloads are a contract too: new handlers must accept old
  payload versions; add `version` to event payloads when shape changes.

## Detecting changes
```bash
# backend contract artifacts
git diff origin/main -- docs/api-v1-implemented-contracts.json docs/openapi/ docs/api-contracts-json.md
# Go response/request structs touched
git diff origin/main --stat -- 'internal/**/transport/**' 'internal/**/http*.go'
# frontend contracts touched
git diff origin/main --stat -- 'src/**/contracts/**' 'src/services/**'
```
(Use the actual base ref; fetch only when permitted.)

## Compatibility matrix (fill in for every API change)

| Scenario | Works? | Evidence |
|---|---|---|
| Old FE → New BE | | |
| New FE → Old BE | | |
| Old worker ↔ New API (shared tables/outbox) | | |
| Old binary on new schema | | |
| Rollback BE only | | |

## Output format
- Changes classified (safe / breaking)
- Matrix above
- Deploy order and flags
- Cleanup (contract) step and when
- Tests added (both repos)
