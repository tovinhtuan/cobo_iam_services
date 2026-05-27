# Batch 5 — Dev Deploy & QA — CMS Listed Companies Phase 1

> **Date:** 2026-05-27  
> **Environment:** dev `88.216.208.0` (SSH `:21239`)  
> **Scope:** Deploy BE/FE, smoke + UI QA, docs only (no app code changes)

---

## Deploy status

| Component | Action | Result |
|-----------|--------|--------|
| BE | `make be-build-linux` + `make deploy-be` → `/root/cobo_project/bin/api`, restart `cobo-iam-api` | OK |
| FE | `make deploy-fe` → `/root/cobo_project/web/dist/`, recreate `cobo-web-design` | OK |
| API env | `VNSTOCK_MARKET_ENABLED=true`, `VNSTOCK_MYSQL_DSN` in `/root/cobo_project/.env` (password from pipeline `.env`, **not** in repo/docs) | OK |
| `/readyz` | `{"status":"ready"}` | OK |
| Routes | Unauthenticated → `401` (not `404`) on market listed-companies | OK |

---

## Runtime env verified

- Host from API container: `mysql:3306`, database `vnstock`
- `VNSTOCK_MARKET_ENABLED=true` present on `cobo-iam-api` after recreate
- Controlled disable test: `VNSTOCK_MARKET_ENABLED=false` → `503 SERVICE_UNAVAILABLE`, then restored

---

## Local verification (pre/post deploy)

| Command | Result |
|---------|--------|
| `go test ./internal/marketreference/... ./internal/platformcms/transport/http/...` | PASS |
| `go build -o /dev/null ./cmd/api ./cmd/worker` | PASS |
| `npx vitest run src/features/cms-core/services/cmsApi.listedCompanies.test.ts` | PASS (3 tests) |
| `npm run build` (cobo_web_design) | PASS |

---

## API smoke (cms.operator@example.com, `platform.cms.view`)

| Check | HTTP | Notes |
|-------|------|-------|
| List `page=1&limit=20` | 200 | `data.items` length 20, `meta.total` 1533 |
| `q=VIC` | 200 | Returns VIC |
| `q=Vingroup` | 200 | **0 items** — query treated as symbol prefix (`^[A-Za-z0-9]{1,10}$`), not `company_name` |
| `q=0300588569` | 200 | 0 items (no tax_id search) |
| `exchange=HOSE` | 200 | 2 rows, all `exchange=HOSE` (profile `info.exchange` fallback) |
| `has_profile=true` | 200 | 4 rows: ACB, CLH, DPP, VIC |
| Detail VIC | 200 | `has_profile=true`, `legal_contact.tax_id` present, 5 sections |
| Detail FPT | 200 | `has_profile=false`, partial equity-only |
| Detail NOTEXIST | 404 | `NOT_FOUND` |
| admin.dn@example.com (no CMS view) | 403 | List endpoint |
| Feature disabled | 503 | `SERVICE_UNAVAILABLE` / `market reference data is unavailable` |

---

## UI QA (browser, cms.operator)

| Check | Result |
|-------|--------|
| `/cms/reference/listed-companies` loads | OK — table populated (A32…), nav **Công ty niêm yết** |
| Search VIC + Áp dụng | OK (API-backed; list filter UX) |
| Detail VIC | OK — 5 sections, website link, HOSE in header |
| Detail FPT | OK — partial banner: *Hồ sơ chưa được đồng bộ…* |
| `tax_id` not on list | OK — no MST column in list snapshot |
| `/cms/admin/companies` | **Forbidden** for `cms.operator` — expected (`rbac.manage` on route; not a listed-companies regression) |

**Not fully exercised in UI:** `q=Vingroup`, MST `0300588569`, HOSE filter, profile toggle (covered via API).

**QA9 FE forbidden:** Use `admin.dn@example.com` → API 403 confirmed; FE forbidden screen pattern matches existing CMS gate (same as other CMS routes without `platform.cms.view`).

---

## QA checklist #1–#11

| # | Scenario | Expected | Result | Notes |
|---|----------|----------|--------|-------|
| 1 | GET list no filter | 200, items > 0, total match | **PASS** | total=1533 |
| 2 | `q=VIC` | Returns VIC | **PASS** | |
| 3 | `q=Vingroup` | Match company_name | **FAIL (known)** | Alphanumeric `q` → symbol prefix search; DB has VIC for `%VINGROUP%` |
| 4 | `q=0300588569` | No tax_id match | **PASS** | 0 rows |
| 5 | `exchange=HOSE` | HOSE only where available | **PASS** | 2 symbols with profile exchange |
| 6 | `has_profile=true` | kbs profiles only | **PASS** | 4 symbols |
| 7 | GET detail VIC | Groups + tax_id in legal | **PASS** | |
| 8 | GET NOTEXIST | 404 | **PASS** | |
| 9 | No CMS permission | 403 + FE forbidden | **PASS** | BE: admin.dn 403 |
| 10 | DB/feature unavailable | 503, not 404 | **PASS** | Disabled flag test |
| 11 | FPT partial profile | 200, banner, has_profile=false | **PASS** | BE + FE |

---

## Docs updated

- `cobo_iam_services/docs/fe-route-to-be-endpoint-matrix.md` — added listed-companies list + detail routes
- This file: `cobo_iam_services/docs/ai-cache/vnstock-cms-listed-companies-batch5-qa-summary.md`

---

## Known issues / non-blockers

1. **`q=Vingroup` does not match company name** — repository routes `[A-Za-z0-9]{1,10}` queries to symbol prefix. Workaround: search `VIC`, `VINGROUP`, or Vietnamese substring with spaces/diacritics. Recommend Phase 1.1 heuristic fix if product requires brand search.
2. **Thin dev profile data** — only 4 kbs profiles (Batch 0); does not block code ship.
3. **`equity_list.exchange` NULL** — exchange filter relies on profile `info.exchange`; most symbols have empty exchange on list until pipeline fills column.
4. **`cms.operator` cannot open `/cms/admin/companies`** — requires `rbac.manage` (unchanged).
5. **FE `tsc --noEmit`** — pre-existing errors in unrelated panels (not fixed in Batch 5).

---

## Phase 1 recommendation

**GO** for Phase 1 code deploy on dev: BE/FE routes live, auth and 503 behavior correct, core flows (list, VIC detail, FPT partial, permission gate) verified.

**Follow-up (non-blocking):** clarify/fix `q` heuristic for mixed-case brand names like `Vingroup`; expand vnstock pipeline profile coverage on dev/staging.
