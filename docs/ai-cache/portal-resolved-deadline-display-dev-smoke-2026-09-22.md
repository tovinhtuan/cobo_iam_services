# Portal company-resolved deadline display — DEV smoke (2026-09-22)

```text
task_type: dev-smoke
plan: docs/ai-cache/portal-resolved-deadline-display-implementation-plan-2026-09-22.md
implementation: docs/ai-cache/portal-resolved-deadline-display-implementation-2026-09-22.md
repos: cobo_iam_services, cobo_web_design
environment: DEV only (88.216.208.0)
verdict: PASS
migration: none
new_api: none
commit: not performed
data_mutation: QA fixture activate only (see Fixture notes); no hard delete; no prod-like config rewrite
closeout: structure resolved_days A≠B + TypeScript baseline (2026-09-22 ~17:50–17:58 +07)
```

## Final verdict

**PASS**

Live DEV chứng minh Company-specific `resolved_days` khác nhau (`30` vs `20`, `STRUCTURE_OVERRIDE`), List/Detail semantic parity, absolute-due SoT (smoke trước), preview path, isolation không stale, UI click-through, related tests + builds. TypeScript `tsc` errors được chứng minh baseline unrelated tới feature production files.

---

## Environment

| Item | Evidence |
|------|----------|
| DEV host | `http://88.216.208.0:3000` (FE), `:8080` (API), SSH `:21239` |
| Deploy (initial smoke) | `deploy-dev.sh be/fe --skip-tests` (~17:21–17:23 +07) |
| Closeout redeploy | Not required — same DEV build; fixture activate only |
| `/healthz` | 200 ok |
| `/readyz` | 200 ready |
| MySQL | `cobo_iam`; no new migration |
| Redis | keys only `cobo_iam:effective_access:{company_id}:{membership_id}` — **no type-only deadline cache** |

---

## Fixture notes (structure closeout)

Existing QA import template already had correct structure map + `use_structure_deadline=true` but was draft (`active_version_no=0`).

| Step | Detail |
|------|--------|
| Template | `qa-import-periodic-1788864118253` |
| Config | `deadline_days=20`, `use_structure_deadline=true`, map `has_subsidiaries=30` / `has_subordinate_units=25` / `simple_structure=20` |
| Prep | Added missing `reminder_config.days_before=[1]` on workflow steps (QA fixture only; was blocking activate) |
| Activate | `POST /api/v1/admin/disclosure-types/{type_id}/activate` `version_no=1` → 200 |
| Cycles after List/Detail | **0** — semantic resolution only; no accidental materialize |
| Cleanup | Leave active for future QA; do **not** hard-delete unless requested |

---

## Test identities (no secrets)

### Company A — structure subsidiaries

| Field | Value |
|-------|--------|
| company_id | `c_001` |
| name | Company X |
| profile | `has_subsidiaries=1`, `has_subordinate_accounting_units=1`, `is_listed=1`, sector commercial |
| expected | `resolved_days=30`, `rule_code=has_subsidiaries`, `resolution_source=STRUCTURE_OVERRIDE` |

### Company B — simple structure

| Field | Value |
|-------|--------|
| company_id | `bd1f02f7-2a54-4848-b0f3-b7cd9dfb9a55` |
| name | QA Manual QR Pkg 20260814 |
| profile | `has_subsidiaries=0`, `has_subordinate=0`, `is_listed=1`, sector commercial |
| expected | `resolved_days=20`, `rule_code=simple_structure`, `resolution_source=STRUCTURE_OVERRIDE` |

**Assumption:** No DEV company with *only* `has_subordinate_units=true` (without subsidiaries). Used allowed alternate: simple structure → 20 days. Path `has_subordinate_units=25` covered by unit/engine tests, not live UI.

### Prior smoke identities (absolute due / preview) — still valid

| Role | company | template | due source |
|------|---------|----------|------------|
| Persisted cycle | `c_001` | `qa-final-focused-20260904222350` | `PLANNED_DATE` `2026-09-26` |
| Preview | `bd1f…` | `qa-af-option-a-ui-1788961913245` | `DEADLINE_SUMMARY_PREVIEW` `2026-10-14` |

---

## Company-specific resolution

Template: `qa-import-periodic-1788864118253`

| Company | Profile | Rule code | Resolved days | Source |
|---------|---------|-----------|--------------:|--------|
| A `c_001` | has_subsidiaries=1 | `has_subsidiaries` | **30** | `STRUCTURE_OVERRIDE` |
| B `bd1f…` | simple (0/0) | `simple_structure` | **20** | `STRUCTURE_OVERRIDE` |

```text
A.resolved_days (30) != B.resolved_days (20)  → PASS
A.rule_code != B.rule_code                    → PASS
resolution_source = STRUCTURE_OVERRIDE        → PASS (both)
```

Raw `deadline_rule` unchanged (display fallback text): `Trong vòng 20 ngày kể từ ngày kết thúc quý.` — FE shows resolved semantic, not invent from raw alone.

`resolved_due_at` / `resolved_due_source` / `deadline_summary`: **null** on this fixture (no cycle / no preview slot yet). Absolute-due SoT verified on prior templates in same report session.

---

## List/Detail parity

### Structure template (semantic)

| Company | List days | Detail days | List due | Detail due | Result |
|---------|----------:|------------:|----------|------------|--------|
| A | 30 | 30 | null | null | **PASS** |
| B | 20 | 20 | null | null | **PASS** |

### Prior absolute-due (unchanged evidence)

| Company | List due | Detail due | source | Result |
|---------|----------|------------|--------|--------|
| A persisted | `2026-09-26T23:59:59+07:00` | same | `PLANNED_DATE` | **PASS** |
| B preview | `2026-10-14T23:59:59+07:00` | same | `DEADLINE_SUMMARY_PREVIEW` | **PASS** |

---

## Browser UI (structure closeout)

### Company A (`c_001`)

- List card `Báo cáo tài chính quý…`: **Trong vòng 30 ngày theo lịch.**
- Detail § Kỳ hạn: Thời hạn áp dụng **30 ngày**; Căn cứ **Có công ty con.**
- No absolute “Hạn chót kỳ” (backend null) — consistent.

### Company B (`bd1f…`)

- List: **Trong vòng 20 ngày theo lịch.** (not 30)
- Detail: Thời hạn **20 ngày**; Căn cứ **Không có công ty con và không có đơn vị kế toán trực thuộc.**

### Isolation

| Check | Result |
|-------|--------|
| A → B (logout / login admin.dn / select QR) | PASS — B shows 20, not A’s 30 |
| B → A (in-app company switcher, no hard refresh) | PASS — Detail refreshes to 30 + “Có công ty con” |
| Redis keys company-scoped | PASS |
| No Redis deadline-by-`type_id` | PASS |
| Cycles after open List/Detail | **0** — PASS (no mutate) |

---

## TypeScript baseline

| Item | Evidence |
|------|----------|
| Command | `cd cobo_web_design && npm run lint` (= `tsc --noEmit`) |
| Exit code | **2** |
| Error count | **57** |
| Feature production files | **CLEAN** — no errors in `deadlineDisplayHelpers.ts`, `DisclosureTypeList/Detail.tsx`, `DisclosureDeadlineSection.tsx`, `resolvedDeadlineRuleDisplay.ts`, `disclosureDetailViewModel.ts` |
| `normalizers.ts:917` TS2677 | Present since `dca29bc2` (2026-08-22) — workflow document type predicate; **not** introduced by resolved-due fields |
| `resolvedDeadlineConsistency.crossLayer.test.tsx` | `tc.frontend \|\| {}` typing noise; Vitest **PASS**; predates absolute-due closeout |
| Other errors | admin roles, channelConfigs `readonly`, AdminCenter tests, ContentDocumentsSection tests, etc. — outside feature |
| `npm run build` | **PASS** (vite; exit 0) |
| Related Vitest | **PASS** (see matrix) |
| Classification | **PASS — baseline unrelated**; deploy may still need `--skip-tests` until unrelated `tsc` cleaned separately |

---

## Test matrix

| Check | Result |
|-------|--------|
| `go test ./internal/disclosure/...` | **PASS** |
| Related Vitest (portal-disclosure + disclosure-detail + normalizers package) | **136 PASS** / 17 files |
| Focused deadline helpers + section + consistency + normalizers.disclosureTypes | **90 PASS** / 5 files |
| `npm run build` | **PASS** |
| `docker compose -f docker-compose.dev.yml build api` | **PASS** (exit 0) |
| `npm run lint` / typecheck | **FAIL exit 2** — baseline unrelated (documented) |
| Full `npm test` | **NOT RUN** (time/scope) |
| Multi-goroutine race harness | **NOT RUN** |
| Config/cycle mutation smoke | **NOT RUN — DEV data safety** |

---

## Isolation / cache (summary)

- Sessions via `select-company` per company.
- Redis: only `effective_access:{company}:{membership}`.
- FE index `Cache-Control: no-store`.
- Company switcher B→A refreshed Detail without hard refresh; no cross-company semantic leak.

---

## Legacy / irregular / CMS (from initial smoke — still valid)

| Case | Result |
|------|--------|
| Irregular no absolute due | PASS |
| CMS labels raw vs engine | PASS |
| Raw-only no resolved DTO | NOT RUN live (unit covered) |

---

## Commands

```bash
# Structure fixture (DEV QA only)
# SQL: fix reminder_config.days_before on qa-import-periodic… v1
# API: activate version_no=1
# API: List/Detail for c_001 and bd1f…

go test ./internal/disclosure/... -count=1
cd ../cobo_web_design && npm run lint          # exit 2 baseline
cd ../cobo_web_design && npm run build         # PASS
cd ../cobo_web_design && npx vitest run …      # related PASS
docker compose -f docker-compose.dev.yml build api  # PASS

curl http://88.216.208.0:8080/healthz
curl http://88.216.208.0:8080/readyz
```

---

## Remaining limitations (non-blocking)

1. Full repo `npm test` not run.
2. Config mutation / cycle rewrite smoke still **NOT RUN** (safety).
3. Live path `has_subordinate_units` → 25 days not exercised (no matching company profile on DEV); simple_structure 20 used as allowed alternate.
4. Unrelated FE `tsc` baseline still fails `deploy-dev.sh` lint gate → `--skip-tests` still needed until cleaned in a separate task.
5. Structure QA template left **active** on DEV (intentional; cleanup only on request).

None of the above contradict SoT, company-specific resolution, isolation, or related verification gates for this feature.

---

## Acceptance mapping

| Criterion | Status |
|-----------|--------|
| A/B different `resolved_days` live | **PASS** (30 vs 20) |
| `STRUCTURE_OVERRIDE` | **PASS** |
| List/Detail semantic parity | **PASS** |
| List/Detail absolute due parity (persisted) | **PASS** (prior) |
| Preview path + disclaimer | **PASS** (prior) |
| Cross-company isolation | **PASS** |
| Browser UI | **PASS** |
| Related tests + builds | **PASS** |
| TypeScript baseline unrelated | **PASS** (classified) |
| No out-of-scope API/migration/engine change | **PASS** |

---

## Verdict rules check

All required PASS gates for closeout met → **PASS**.

```text
Không tạo API deadline-resolution mới.
Không tạo migration.
Không tính deadline ở frontend.
List và Detail dùng cùng absolute-due + company-resolved semantic SoT trên DEV (verified).
Company A/B resolved_days khác nhau trên cùng Template (verified).
Không commit / không push.
```
