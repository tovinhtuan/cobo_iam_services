# Batch 0 Verification — CMS Listed Companies (vnstock)

> **Date:** 2026-05-27  
> **Environment:** dev `88.216.208.0` (SSH port `21239`)  
> **Scope:** Data / dev env only — no application code changes  
> **Source plan:** [vnstock-cms-listed-companies-phase1-implementation-plan.md](./vnstock-cms-listed-companies-phase1-implementation-plan.md)

---

## 1. DB / table status

| Check | Result |
|-------|--------|
| MySQL container | `cobo-iam-mysql` running |
| Database `vnstock` | Accessible (user `vnstock`, credentials in server `.env` / pipeline config — **not stored in this doc**) |
| Table `equity_list` | **Exists** — matches DDL in [vnstock/pipeline/db/mysql.py](../../../vnstock/pipeline/db/mysql.py) |
| Table `company_profiles` | **Exists** — matches DDL (PK `symbol`, `source`; JSON `info`, `officers`, …) |

**Schema confirmation (repo vs live)**

| Table | Expected (DDL) | Live |
|-------|----------------|------|
| `equity_list` | PK(`symbol`), `company_name`, `exchange`, `source`, `updated_at` | OK |
| `company_profiles` | PK(`symbol`,`source`), `info` JSON + related JSON columns, `updated_at` | OK |

---

## 2. Row counts

| Metric | Count |
|--------|------:|
| `equity_list` (all) | **1533** |
| `company_profiles` where `source = 'kbs'` | **4** |
| `company_profiles` (all sources) | **4** |
| `equity_list.exchange` non-null / non-empty | **0** |

**Symbols with `company_profiles` (kbs):** `ACB`, `CLH`, `DPP`, `VIC`

**Sample `equity_list` (VIC / ACB / FPT):**

| symbol | company_name (truncated) | exchange (column) | profile row |
|--------|--------------------------|-------------------|-------------|
| VIC | Tập đoàn VINGROUP - CTCP | NULL | Yes |
| ACB | Ngân hàng TMCP Á Châu | NULL | Yes |
| FPT | CTCP FPT | NULL | No |

UTF-8 display in terminal may show mojibake; DB charset is `utf8mb4`.

---

## 3. Sample symbols & JSON key coverage

### `JSON_KEYS(info)` — VIC and ACB (kbs)

Both include all Phase 1 keys from KBS map ([vnstock/explorer/kbs/const.py](../../../vnstock/vnstock/explorer/kbs/const.py)):

`company_type`, `business_model`, `business_code`, `founded_date`, `listing_date`, `as_of_date`, `charter_capital`, `par_value`, `listing_price`, `listed_volume`, `outstanding_shares`, `free_float`, `free_float_percentage`, `ceo_name`, `ceo_position`, `inspector_name`, `inspector_position`, `number_of_employees`, `establishment_license`, `tax_id`, `auditor`, `address`, `phone`, `fax`, `email`, `website`

Additional keys in JSON (non-blocking): `symbol`, `history`, `branches`, `exchange`

### `JSON_CONTAINS_PATH` — VIC (all Phase 1 keys)

All **25** required Phase 1 paths present (`1` for each).

### Sample field sanity (VIC)

| Field | Present | Notes |
|-------|---------|-------|
| `tax_id` | Yes | Value present (display-only in Phase 1) |
| `ceo_name` | Yes | Value present |
| `exchange` in `info` | Yes | `HOSE` (while `equity_list.exchange` is NULL) |

### List without profile (examples)

`A32`, `AAA`, `AAH`, `AAM`, `AAN` — in `equity_list`, no `company_profiles` row → supports **200 partial + `has_profile=false`** UX for ~1529 symbols.

---

## 4. DSN recommendation (API container)

| Item | Finding |
|------|---------|
| API container | `cobo-iam-api` |
| MySQL container hostname from API | `mysql` resolves (`172.18.0.2`) |
| Docker network | `cobo-iam-api` and `cobo-iam-mysql` on **`cobo_project_default`** |
| Connectivity test | From API network namespace: `SELECT COUNT(*)` on `equity_list` → **1533**, `company_profiles` (kbs) → **4** |
| `mysql` CLI inside API image | **Not installed** (expected; Go uses `database/sql` + driver) |
| Current `MYSQL_DSN` | Points to `cobo_iam` DB on `tcp(mysql:3306)` — **not** `vnstock` |
| `VNSTOCK_MYSQL_DSN` / `VNSTOCK_MARKET_ENABLED` | **Not set** in API container env today |

**Recommended `VNSTOCK_MYSQL_DSN` (API / docker-compose — do not commit password):**

```text
VNSTOCK_MYSQL_DSN=vnstock:<PASSWORD>@tcp(mysql:3306)/vnstock?parseTime=true&loc=UTC&tls=false
VNSTOCK_MARKET_ENABLED=true
```

- Use host **`mysql`**, port **`3306`**, database **`vnstock`** (same MySQL instance as `cobo_iam`, different schema).
- Do **not** use `127.0.0.1` from inside `cobo-iam-api` (that is the API container itself).
- From **host SSH** or cron on server, `127.0.0.1:3306` remains valid for pipeline CLI.

**503 behavior (Phase 1 decision):** When DSN unset or DB unreachable, API returns `503 SERVICE_UNAVAILABLE` — aligns with current plan; env not configured yet.

---

## 5. Index recommendation

**Current indexes (live):**

| Table | Indexes |
|-------|---------|
| `equity_list` | PRIMARY (`symbol`) only |
| `company_profiles` | PRIMARY (`symbol`, `source`) only |

**Recommended (not applied — awaiting approval):**

```sql
-- List filter by exchange (once column populated OR if filtering via generated column later)
CREATE INDEX idx_equity_list_exchange ON equity_list (exchange);

-- Search by company name (LIKE / prefix)
CREATE INDEX idx_equity_list_company_name ON equity_list (company_name(120));

-- Sort / freshness on profile join
CREATE INDEX idx_company_profiles_updated ON company_profiles (updated_at);
```

**Note:** With `equity_list.exchange` currently all NULL, exchange filter for list should use `COALESCE(e.exchange, JSON_UNQUOTE(JSON_EXTRACT(p.info,'$.exchange')))` until pipeline populates `equity_list.exchange`.

---

## 6. Pipeline / ops notes

| Item | Status |
|------|--------|
| Cron | `0 2 * * *` — `pipeline.run --all-symbols --tasks company` → `pipeline.log` |
| `pipeline.log` | **Not created yet** (cron may not have fired since install) |
| Pipeline dir | `/root/cron_company_vnstock` present with `.venv` |
| Full `--all-symbols` run in this batch | **Not executed** (per scope; data partially filled from manual/smoke runs) |

---

## 7. Blockers / risks

| ID | Severity | Issue | Impact | Suggested action |
|----|----------|-------|--------|------------------|
| B0-1 | **High (data)** | Only **4 / 1533** `company_profiles` (kbs) | CMS list works; almost all details are partial (`has_profile=false`) | Run company pipeline for all symbols (or approved subset) on dev — **requires your approval** for long job |
| B0-2 | **Medium (data)** | `equity_list.exchange` **NULL for all rows** | Exchange filter on list table alone will not work | Fix upsert from listing API or derive exchange in BE query from `info` when joined |
| B0-3 | **Low (ops)** | `VNSTOCK_*` env not on API | Market endpoints cannot work until Batch 1 deploy + env | Batch 1 + dev deploy config |
| B0-4 | **Low** | No secondary indexes | Acceptable for ~1.5k rows MVP; add indexes before prod scale | Apply recommended SQL after approval |

**Not blockers for starting Batch 1 (code):** Schema exists, JSON shape verified, network path API→MySQL confirmed, Phase 1 field map validated on VIC/ACB.

---

## 8. Batch 1 readiness

| Criterion | Ready? |
|-----------|--------|
| Tables + DDL align with plan | Yes |
| Read path technically feasible (second DSN) | Yes |
| Phase 1 JSON field contract on sample data | Yes |
| Representative dev data volume for **profiles** | **No** — only 4 symbols |
| Exchange column ready for filter | **No** — use JSON fallback or backfill |
| Permission / route decisions locked | Yes (`platform.cms.view`, 200 partial, 503 if DSN down) |

**Verdict:** **Batch 1 BE config + repository may start.** Parallel or follow-up **data backfill** strongly recommended before Batch 5 QA sign-off.

---

## 9. QA implications (preview)

| Plan # | Expectation on current dev data |
|--------|----------------------------------|
| 1 List no filter | 1533 rows; most without profile metadata |
| 7 Detail VIC | Full nested groups |
| 8 Detail NOTEXIST | 404 |
| 11 List yes, profile no | ~1529 symbols (e.g. FPT) — banner + partial |

---

## 10. Docs consulted

- [vnstock/pipeline/README.md](../../../vnstock/pipeline/README.md)
- [vnstock/pipeline/db/mysql.py](../../../vnstock/pipeline/db/mysql.py)
- [vnstock/vnstock/explorer/kbs/const.py](../../../vnstock/vnstock/explorer/kbs/const.py)

**Cache:** `cobo_iam_services/docs/ai-cache/vnstock-cms-listed-companies-batch0-verification.md`
