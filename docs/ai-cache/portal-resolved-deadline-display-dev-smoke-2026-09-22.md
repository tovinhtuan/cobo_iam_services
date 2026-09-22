# Portal company-resolved deadline display — DEV smoke (2026-09-22)

```text
task_type: dev-smoke
plan: docs/ai-cache/portal-resolved-deadline-display-implementation-plan-2026-09-22.md
implementation: docs/ai-cache/portal-resolved-deadline-display-implementation-2026-09-22.md
repos: cobo_iam_services, cobo_web_design
environment: DEV only (88.216.208.0)
verdict: PASS WITH LIMITATIONS
migration: none
new_api: none
commit: not performed
data_mutation: none (read-only smoke; no config/cycle rewrite)
```

## Final verdict

**PASS WITH LIMITATIONS**

Core absolute-due SoT (List↔Detail parity), persisted `PLANNED_DATE`, preview `DEADLINE_SUMMARY_PREVIEW`, irregular skip, CMS labels, company-scoped sessions, and related tests all pass on live DEV after redeploy.

Limitations below are non-blocking for SoT but prevent a pure `PASS`.

---

## Environment

| Item | Evidence |
|------|----------|
| DEV host | `http://88.216.208.0:3000` (FE), `:8080` (API), SSH `:21239` |
| Deploy | `deploy-dev.sh be --skip-tests` then `fe --skip-tests` (2026-09-22 ~17:21–17:23 +07) |
| Why `--skip-tests` on deploy | `deploy-dev.sh all` blocked by **pre-existing** FE `tsc --noEmit` errors unrelated to deadline (admin/roles/channelConfigs). Related go/vitest run **before** deploy. |
| `/healthz` | 200 ok |
| `/readyz` | 200 ready |
| FE index | 200; asset `index-DWmk2Tu6.js` contains `Hạn chót:`, `Theo chu kỳ đã tạo`, `Ngày dự kiến theo cấu hình`, `list-card-deadline-due` |
| BE binary | symbols `attachDetailAbsoluteResolvedDue`, `resolved_due_source`, `DEADLINE_SUMMARY_PREVIEW` present on `/root/cobo_project/bin/api` |
| MySQL | `cobo_iam` healthy; no new migration |
| Redis | `DBSIZE=3`; keys only `cobo_iam:effective_access:{company_id}:{membership_id}` — **no disclosure-type deadline cache** |

Local verification before deploy:

```text
go test ./internal/disclosure/... → PASS
vitest deadline helpers + DisclosureDeadlineSection → 32 PASS
npm run build (FE) → PASS (during fe deploy)
```

---

## Test identities (no secrets)

### Company A — persisted cycle

| Field | Value |
|-------|--------|
| company_id | `c_001` |
| name | Company X |
| profile | `has_subsidiaries=1`, `has_subordinate_accounting_units=1`, `is_listed=1`, sectors commercial/service/manufacturing |
| user | `platform.tenant.admin@example.com` → membership `m_107` |
| template | `qa-final-focused-20260904222350` |
| cycle_label | `2026-09-22` |
| due source | `PLANNED_DATE` → `2026-09-26T23:59:59+07:00` |

### Company B — preview (API) / secondary UI switch

| Field | Value |
|-------|--------|
| company_id (API) | `bd1f02f7-2a54-4848-b0f3-b7cd9dfb9a55` |
| name | QA Manual QR Pkg 20260814 |
| profile | simple structure (`has_subsidiaries=0`, `has_subordinate=0`), `is_listed=1`, sector commercial |
| user | `admin.dn@example.com` |
| template (preview) | `qa-af-option-a-ui-1788961913245` |
| no cycle for current monthly slot `2026-09` | confirmed (only cycle_label `2026-10` exists) |
| due source | `DEADLINE_SUMMARY_PREVIEW` → `2026-10-14T23:59:59+07:00` |

UI company switcher (same platform admin session) also exercised **Company Y (`c_002`)** for isolation refresh (0 cycles for preview template; opening Detail did **not** insert cycles).

**Assumption:** QA fixtures above are safe DEV data; no business production tenants mutated.

---

## Company A result (persisted)

### API

| Surface | resolved_due_at | resolved_due_source | semantic |
|---------|-----------------|---------------------|----------|
| List | `2026-09-26T23:59:59+07:00` | `PLANNED_DATE` | DEFAULT / 5 CALENDAR_DAYS |
| Detail | `2026-09-26T23:59:59+07:00` | `PLANNED_DATE` | same |
| Parity | **PASS** due | **PASS** source | |

`deadline_summary.deadline_date` = `2026-09-26` (same calendar day as planned in this fixture — cannot prove diverge numerically here; unit tests cover cycle≠preview).

### Browser (Company X)

- List card: semantic “Trong vòng 5 ngày theo lịch…” + `Hạn chót: 26/09/2026`
- Detail § Kỳ hạn: Thời hạn áp dụng + `Hạn chót kỳ 26/09/2026` + `Nguồn: Theo ngày kế hoạch`
- No invented `T+5` sentence on List when resolved DTO present

---

## Company B result (preview)

### API (`bd1f02f7…` + `qa-af-option-a-ui-…`)

| Surface | resolved_due_at | resolved_due_source | parity |
|---------|-----------------|---------------------|--------|
| List | `2026-10-14T23:59:59+07:00` | `DEADLINE_SUMMARY_PREVIEW` | |
| Detail | same | same | **PASS** |

### Browser (Company X still valid for same preview template; also Company Y)

- Detail: `Hạn chót kỳ 14/10/2026`
- `Nguồn: Ngày dự kiến theo cấu hình`
- Disclaimer: “Ngày dự kiến theo cấu hình — chưa phải hạn đã chốt theo chu kỳ.”
- DB: `c_002` still **0** cycles for this type after open List/Detail → preview **not** persisted

---

## Isolation result

| Check | Result |
|-------|--------|
| Separate access tokens per company (select-company) | PASS |
| Redis keys include `company_id` + `membership_id` | PASS |
| No Redis key caching resolved due by `type_id` alone | PASS |
| A→Y→A company switcher UI | PASS (context label updates; no hard refresh required) |
| Same template due identical when both companies share same planned cycle | Expected for shared fixture — **not** a leak |
| Structure-based different `resolved_days` A vs B | **NOT DEMONSTRATED** — QA templates have `use_structure_deadline` off → both DEFAULT 5 |

---

## Legacy / irregular

| Case | Result |
|------|--------|
| Irregular `qa-irregular-alert-20260904a` | `resolved_due_at=null`, no source; raw `Trong vòng 24 giờ kể từ sự kiện` |
| List irregular UI | Shows semantic from DTO days when present (“20 ngày theo lịch”) / no `Hạn chót` line when due null |
| Raw `T+5` on periodic with resolved DTO | FE shows sentence from DTO, **not** parse/expand `T+5` |
| CMS Portal preview of raw | Editor shows `Portal sẽ hiển thị: T+5` as display-only |

Raw-only fallback (no `resolved_deadline_rule` at all): **NOT RUN** — current active QA templates always return resolved DTO when profile+rules exist. Covered by unit tests.

---

## Config / cycle consistency

```text
NOT RUN — DEV data safety constraint
```

No Template config mutation / cycle rewrite attempted. Read path confirmed: preview does not insert cycles.

---

## CMS Template Editor

Observed on `qa-af-option-a-ui-1788961913245`:

- Help: engine config vs Portal display text (`Áp dụng theo doanh nghiệp`)
- Help: `deadline_rule` = fallback text; engine = `deadline_config` + `applicability_rules`
- Section: `Cấu hình deadline (deadline_config)`
- No real Company profile preview in Phase 1

---

## Cache verification

- Disclosure list/detail: MySQL read-through; no catalog Redis cache for deadline resolution.
- Effective-access Redis keys are company-scoped.
- FE index `Cache-Control: no-store, no-cache, must-revalidate`.
- Company switch refreshed context without stale cross-company due invent.

---

## Commands

```bash
# Deploy
sh deploy-dev.sh verify
go test ./internal/disclosure/... -count=1
sh deploy-dev.sh be --skip-tests
npx vitest run …deadline…  # 32 PASS
sh deploy-dev.sh fe --skip-tests

# Health
curl http://88.216.208.0:8080/healthz
curl http://88.216.208.0:8080/readyz

# API (tokens redacted in this doc)
POST /api/v1/auth/login → select-company
GET /api/v1/disclosure-types?page_size=50
GET /api/v1/disclosure-types/{type_id}

# DB read-only checks via docker exec mysql
# Redis KEYS / DBSIZE
```

Related tests: **PASS**  
Full `npm test` suite: **NOT RUN** (time/scope; focused deadline suites run)  
Multi-goroutine race harness: **NOT RUN**

---

## Limitations

1. Deploy script FE lint gate blocked by unrelated pre-existing TS errors → used `--skip-tests` after local related verification.
2. No Template config mutation smoke (safety).
3. No live fixture proving structure override yields different `resolved_days` for A vs B (toggle off on QA templates).
4. No fixture where `deadline_summary.deadline_date` ≠ persisted planned date on Detail (same day coincidence).
5. UI Company B for preview used Company Y switch + API company `bd1f02f7…` (platform admin switcher lacks QR company).
6. Full repo `npm test` not run.

---

## Acceptance mapping

| Criterion | Status |
|-----------|--------|
| A persisted cycle List/Detail same due+source | PASS |
| B preview List/Detail same due+source | PASS |
| Preview labeled as dự kiến | PASS |
| No preview DB write | PASS |
| Irregular no absolute due | PASS |
| No T+N invent on List when resolved | PASS |
| Company-scoped auth / redis keys | PASS |
| UI click-through A + preview + CMS | PASS |
| Related tests | PASS |
| Config rewrite / structure multi-company days | LIMITED / NOT RUN |

---

## Verdict rules check

Not pure `PASS` due to limitations 1–5 above → **PASS WITH LIMITATIONS**.

```text
Không tạo API deadline-resolution mới.
Không tạo migration.
Không tính deadline ở frontend.
List và Detail dùng cùng absolute-due source-of-truth trên DEV (verified).
Không commit / không push.
```
