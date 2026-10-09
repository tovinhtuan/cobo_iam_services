---
name: market-reference-integration
description: Dùng khi làm việc với dữ liệu thị trường/doanh nghiệp niêm yết trong Cobo - module marketreference đọc DB vnstock (equity_list, company_profiles, nguồn KBS), API listed-lookup, nạp dữ liệu từ vnstock pipeline hoặc crawl_company_info (Vietstock) vào hồ sơ công ty; rate limit, độ tươi dữ liệu, ánh xạ mã CK/MST.
---

# Market Reference Integration

## Current state (verify; as of 2026-10)
- `internal/marketreference`: read-only access to the vnstock MySQL DB
  (`VNSTOCK_MYSQL_DSN`, `VNSTOCK_MARKET_ENABLED`): tables `equity_list`,
  `company_profiles` (KBS source).
- Public `GET /api/v1/company/listed-lookup`: rate-limited only by nginx
  (10 r/m per IP, burst 3, 429 + `Retry-After`).
- Data producers (separate repos in the workspace):
  - `vnstock/pipeline/` CLI ETL (KBS via vnstock → MySQL), see `vnstock-provider`
    in that repo.
  - `crawl_company_info` (Vietstock profile crawler by name/tax code/URL), see
    `crawl-source-adapter` in that repo.

## Rules
1. Treat reference data as untrusted external input: validate, normalize
   (stock code upper-case, tax code digits only, Unicode NFC names), cap sizes.
2. Read-only DB user for the vnstock DB; separate pool; per-query timeout;
   API must degrade gracefully (feature off / empty result) when it is down.
3. Public endpoints: app-level rate limit or nginx limit documented; minimal
   fields; no enumeration of the full list without paging and limits.
4. Matching company ↔ listed entity: prefer tax code exact match, then stock
   code, then fuzzy name with confidence; never auto-overwrite a company's
   verified profile without user/admin confirmation.
5. Record provenance (source, fetched_at) and freshness; show staleness in UI.
6. Licensing: vnstock is licensed for personal/research, non-commercial use -
   flag commercial usage to the user; scraping must respect source terms and
   rate limits.
7. Caching: bounded in-process caches with TTL (see `cache-versioning-review`).

## Tests
- vnstock DB unavailable → API returns defined fallback, no 500 storm.
- Lookup by tax code / stock code / partial name; Vietnamese diacritics.
- Rate-limit behavior documented (nginx) or tested (app).

## Output format
- Data flow (source → store → API → UI)
- Matching and provenance rules
- Failure behavior
- Tests
