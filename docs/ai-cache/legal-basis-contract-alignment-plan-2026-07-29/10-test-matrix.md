# 10 — Test matrix

**Status:** Updated Phase 12.1A — golden cases in `16-projection-golden-cases.md`

## Counts (planned)

| Suite | Cases (planned) |
| --- | --- |
| Unit | 18+ |
| API | 14 |
| Migration | 10 |
| Frontend | 12 |
| E2E | 10 |
| Golden (projection/divergence/lifecycle/migration) | see `16` |
| **Total** | **64+** |

## 19.1 Unit (examples)

1. sanitize drops empty items
2. sanitize keeps title-only / summary-only
3. projection: title else summary; join `\n\n` (P1–P10)
4. read priority: bases wins
5. read priority: fallback synthesize title=`Cơ sở pháp lý`
6. read priority: empty
7. link reject `#` / javascript
8. date format reject
9. max length **reject** (no silent truncate)
10. exact duplicate **block**; soft-warn same title
11–18. normalizer FE parity + no risk cross-fill regressions

## 19.2 API

create/update/detail admin; tenant detail; activate; versions; company create; copy; reject invalid URL; empty bases+flat; bases-only; structured write ignores client flat / overwrites projection

## 19.3 Migration

A–E detection; A apply idempotent; **D report-only no auto mutate**; dry-run no writes; rollback snapshot; batch resume

## 19.4 FE

CMS add/edit/remove/reorder; legacy card import; validation; dirty guard; tenant cards/modal/empty; permission gate CMS

## 19.5 E2E (mandatory)

| ID | Scenario |
| --- | --- |
| E2E-01 | CMS create structured → tenant same items |
| E2E-02 | CMS edit → after publish tenant updated |
| E2E-03 | legacy flat only → tenant fallback text |
| E2E-04 | legacy → CMS convert → round-trip |
| E2E-05 | clone/version preserves content (new ids) |
| E2E-06 | global → company copy preserves (regen ids) |
| E2E-07 | divergence detection (no silent Group D overwrite) |
| E2E-08 | permission denied on CMS write |
| E2E-09 | mobile 390 card display |
| E2E-10 | flag-off rollback compatibility |

## Validation / RBAC (LOCKED Phase 12.1A)

### Validation MVP

| Rule | Draft | Publish/Activate |
| --- | --- | --- |
| Min items | **0** | **0** (no silent required) |
| Max items | 20 | 20 |
| title/summary | at least one non-empty per item | same |
| URL | if set, must be valid | same |
| Field lengths | OD-6 table | same |

### RBAC (reuse existing)

| Role | CMS view | CMS edit | Tenant view | Tenant edit | Publish |
| --- | --- | --- | --- | --- | --- |
| Platform CMS | Y | Y | N/A | N | Y (platform) |
| Enterprise admin | N | N | Y (types) | company create only | company lifecycle per existing |
| Standard member | N | N | if permitted | N | N |

**No new permission** unless gap proven in implement discovery.

## 19.6 Phase 12.5 lifecycle (implemented 2026-07-29)

| Area | Status |
| --- | --- |
| Unit deep-copy / ID regen / legacy-only / malformed | PASS (`legal_basis_lifecycle_test.go`) |
| In-memory same-draft + new-version + activate | PASS (inmemory `legal_basis_lifecycle_test.go`) |
| Clone / global→company | N/A (no endpoint) |
| Live MySQL httpserver E2E | Pre-existing failures — not required for 12.5 gate |


## 19.7 Phase 12.6A / 12.6B-Plan (2026-07-29)

| Area | Status |
| --- | --- |
| Analyzer + SQL allowlist unit tests | PASS (12.6A) |
| DEV RO inventory dry-run | PASS_READ_ONLY_DRY_RUN (gov: FAIL_SCOPE_CREEP) |
| Controlled backfill apply tests | **Designed** in `phase-12-6b-test-plan.md` — **not executed** |
| DEV mutation | **0** (Plan docs only; `BACKFILL_PLAN_READY`) |
