# Batch 2 Report: Listed Company Lookup & Sync

**Date:** 2026-06-05  
**Scope:** A4 (AdminHandler wiring) + A5 (lookup handler) + A6 (server wiring) + A7 (Nginx config)  
**Status:** ✅ COMPLETE

---

## Architecture Verification

| Assumption | Verified | File | Notes |
|---|---|---|---|
| `AdminHandler` uses With* injection pattern | ✅ | `admin_handler.go:34–54` | `WithTokenIssuer`, `WithIdempotency`, `WithSelfCreateEnabled` — exact pattern followed |
| `httpx.WriteJSON` is flat (no data: envelope) | ✅ | `platform/httpx/json.go:31` | CMS uses `writeEnvelope`, AdminHandler uses `WriteJSON` directly — flat JSON |
| `httpx.WriteError` maps `*perr.HTTPError` → `{"error":{...}}` | ✅ | `platform/httpx/json.go:39` | Consistent error shape |
| `perr.CodeInvalidRequest`, `perr.CodeServiceUnavailable` exist | ✅ | `platform/errors/errors.go:28,40` | Used in handler |
| `slog.InfoContext` pattern for structured logging | ✅ | `admin_handler_provision.go:151` | Same pattern followed |
| `httpx.RequestIDFromContext` available | ✅ | `platform/httpx/requestid.go:24` | Used in audit log |
| `r.RemoteAddr` for IP capture (consistent across codebase) | ✅ | `admin_handler.go:178,280` | Same approach used |
| Nginx deploy path is `deploy/nginx/` | ✅ | Existing: `deploy/nginx/cobo-api.example.conf` | New file follows `.example.conf` pattern |
| `listedCompaniesSvc` var in scope at A6 injection point | ✅ | `server.go:170,439` | Built at line 170, wiring at line 439 — correct order |

---

## Security Review

| Threat | Analysis | Mitigation |
|---|---|---|
| SQL Injection | `business_code` reaches `database/sql` parameterized query — no interpolation | Character allowlist `[0-9A-Za-z\-/]` + parameterized query in repo |
| Log Injection (`\n`, `\r`) | Attacker injects newline → forges log lines | Allowlist rejects all control chars (ASCII < 32). Tested via `TestListedLookup_SpecialCharsRejected` |
| XSS | Response is JSON, not HTML. `httpx.WriteJSON` uses `json.Encoder.SetEscapeHTML(true)` | Safe — angle brackets, quotes escaped in JSON |
| Header Injection | `business_code` is query param, not set into response headers | Not applicable |
| Path Traversal | Used only as SQL param, no filesystem access | Not applicable |
| Unicode / non-ASCII homoglyphs | Could create cache key confusion (e.g. fullwidth digits look like ASCII) | Allowlist only allows `[0-9A-Za-z\-/]` — pure ASCII, no Unicode |
| PII in audit log | Full business_code, email, phone exposed in logs | Only `safePrefix(code, 4)` logged — max 4 chars. No email/phone/name logged |
| Cache-Control on error responses | P1-1 fix: 400/503 must not be cached by CDN/browser | `no-store` on all 4xx/5xx responses. `public max-age=3600` only on 200 found, `public max-age=600` on 200 not_found |

---

## Abuse Prevention Review

| Scenario | Risk | Mitigation |
|---|---|---|
| Enumeration of valid ĐKKD | Low — data is public market data | Nginx rate limit 10r/m/IP, burst=3 |
| Scraping email/phone/address | Low-Med — public info but can harvest at scale | Rate limit + audit log captures IP pattern |
| Bot traffic / automated scanning | Medium | Nginx rate limit zone `listed_lookup` (separate from other zones) |
| Cache bypass (submit 2000 unique invalid codes) | Low | 10-min negative TTL + `evictOldestLocked()` prevents unbounded growth |
| High-cardinality DoS (exhaust DB connection pool) | Low | Rate limit + in-process cache absorbs repeat hits |

---

## Files Changed

| File | Change |
|---|---|
| `internal/companyaccess/transport/http/admin_handler.go` | Added `listedLookup *marketapp.Service` field + `WithListedCompaniesLookup` method + route `GET /api/v1/company/listed-lookup` |
| `internal/httpserver/server.go` | Added `adminHandler.WithListedCompaniesLookup(listedCompaniesSvc)` |

## Files Created

| File | Description |
|---|---|
| `internal/companyaccess/transport/http/admin_handler_listed_lookup.go` | Handler: `validateBusinessCode`, `safePrefix`, `lookupMapErr`, `listedLookupByBusinessCode`, `buildLookupResponse` |
| `internal/companyaccess/transport/http/admin_handler_listed_lookup_test.go` | 12 handler test cases |
| `deploy/nginx/rate-limit-lookup.conf.example` | Nginx rate limit config (follows existing `.example.conf` pattern) |

---

## Tests Added

| Test | Purpose |
|---|---|
| `TestListedLookup_Found` | Happy path: full profile, sync populated, Cache-Control public, nosniff |
| `TestListedLookup_NotFound` | ErrNotFound → 200 found:false, Cache-Control public short TTL |
| `TestListedLookup_EmptyBusinessCode` | Empty input → 400, Cache-Control no-store |
| `TestListedLookup_MissingBusinessCode` | No query param → 400 |
| `TestListedLookup_WhitespaceBusinessCode` | Whitespace-only (URL-encoded spaces) → 400 |
| `TestListedLookup_OversizedBusinessCode` | 51-char input → 400 |
| `TestListedLookup_SpecialCharsRejected` | SQL inject (`'`), log inject (`\n`), semicolons → 400 each |
| `TestListedLookup_ServiceUnavailable` | ErrUnavailable → 503, Cache-Control no-store |
| `TestListedLookup_NilService503` | Nil listedLookup field → 503 (nil-safe) |
| `TestListedLookup_SyncOmitsNilFields` | Nil phone/email/address → omitted from sync object |
| `TestListedLookup_ErrorResponseShape` | Error response has `error.code` field |
| `TestListedLookup_CacheHitSecondRequest` | Second request → `cache_hit=true` in log, same found:true response |

---

## Tests Run

```
go test ./internal/companyaccess/transport/http/... -count=1 -run TestListedLookup -v
go test ./internal/companyaccess/transport/http/... -count=1 -v
go test ./internal/marketreference/... -count=1
go test -race ./internal/companyaccess/... -count=3 -run "TestListedLookup|TestInitialize|TestCreateSelfServiceCompany_Success|TestOwnCompany|TestProvision"
go build ./...
go vet ./...
```

---

## Test Results

| Package / Suite | Tests | Result |
|---|---|---|
| `companyaccess/transport/http` — TestListedLookup* | 12/12 | ✅ PASS |
| `companyaccess/transport/http` — full suite | 25/26 | ✅ 25 PASS; 1 pre-existing FAIL |
| `marketreference/app` | 14/14 | ✅ PASS |
| `marketreference/infra/mysql` | 11/11 | ✅ PASS |
| Race detector (count=3) — lookup + provision tests | All | ✅ PASS — no races |
| `go build ./...` | — | ✅ PASS |
| `go vet ./...` | — | ✅ PASS — no warnings |

**Pre-existing failure (not caused by Batch 2):**
- `TestCreateSelfServiceCompany_FeatureFlagOff` — documented in Batch 1 report, confirmed pre-existing in git HEAD `d10906c` before any changes.

---

## Verification Matrix

| Scenario | Result |
|---|---|
| Found → 200 found:true, sync + preview populated | ✅ PASS |
| Not Found → 200 found:false | ✅ PASS |
| Empty input → 400 INVALID_REQUEST | ✅ PASS |
| Oversized input → 400 | ✅ PASS |
| Whitespace-only → 400 | ✅ PASS |
| SQL injection attempt → 400 | ✅ PASS |
| Log injection attempt (\n) → 400 | ✅ PASS |
| Special chars (semicolons) → 400 | ✅ PASS |
| Service unavailable → 503 | ✅ PASS |
| Nil listedLookup field → 503 (nil-safe) | ✅ PASS |
| Cache-Control: public on 200 found | ✅ PASS |
| Cache-Control: public on 200 not_found | ✅ PASS |
| Cache-Control: no-store on 400 | ✅ PASS |
| Cache-Control: no-store on 503 | ✅ PASS |
| X-Content-Type-Options: nosniff on all responses | ✅ PASS |
| Nil fields omitted from sync object | ✅ PASS |
| Error response shape: `{"error":{"code":...}}` | ✅ PASS |
| Cache hit → second request successful | ✅ PASS |
| Race test — no data races | ✅ PASS |
| go build clean | ✅ PASS |
| go vet clean | ✅ PASS |

---

## Privacy Verification

| Field | Logged? | Notes |
|---|---|---|
| Full `business_code` | ❌ NO | Only `safePrefix(code, 4)` — max 4 chars |
| `email` | ❌ NO | Not in any log field |
| `phone` | ❌ NO | Not in any log field |
| `company_name` | ❌ NO | Not in any log field |
| `tax_id` | ❌ NO | Not in any log field |
| IP address (`r.RemoteAddr`) | ✅ YES | Standard audit field, consistent with existing handlers |
| `user_agent` | ✅ YES | Standard audit field, no PII |
| `request_id` | ✅ YES | Correlation ID, no PII |
| `result` | ✅ YES | Enum: found/not_found/unavailable/error_invalid |
| `cache_hit` | ✅ YES | Bool, no PII |
| `duration_ms` | ✅ YES | Latency metric, no PII |

---

## Observability Verification

| Field | Present in log | Verified by |
|---|---|---|
| `event` = "listed_lookup_requested" | ✅ | Log output in test run |
| `request_id` | ✅ | Emitted on every call |
| `ip` | ✅ | `r.RemoteAddr` |
| `user_agent` | ✅ | On valid requests |
| `business_code_prefix` | ✅ | 4-char safe prefix |
| `result` | ✅ | found/not_found/unavailable/error_invalid |
| `cache_hit` | ✅ | `cache_hit=true` visible on 2nd request in TestListedLookup_CacheHitSecondRequest |
| `duration_ms` | ✅ | Emitted on every call |

Log output verified in test run:
```
listed_lookup_requested event=listed_lookup_requested request_id="" ip=192.0.2.1:1234 user_agent="" business_code_prefix=0101 result=found cache_hit=false duration_ms=0
listed_lookup_requested event=listed_lookup_requested request_id="" ip=192.0.2.1:1234 user_agent="" business_code_prefix=0101 result=found cache_hit=true duration_ms=0
```

---

## Nginx Verification

| Config | Path | Pattern |
|---|---|---|
| Rate limit config | `deploy/nginx/rate-limit-lookup.conf.example` | Follows existing `cobo-api.example.conf` pattern in `deploy/nginx/` |
| Zone name | `listed_lookup` | Separate zone — no interference with other rate limits |
| Rate | `10r/m` per IP | Tuning notes included in file |
| Burst | `burst=3 nodelay` | Handles normal debounce behavior |
| 429 status | `limit_req_status 429` | Explicit (Nginx default is 503) |
| Retry-After | `add_header Retry-After 60 always` | Guides client retry timing |
| Proxy headers | Matches existing `cobo-api.example.conf` pattern | `X-Forwarded-For`, `X-Forwarded-Proto` |

Note: Nginx config is an `.example` file — must be adapted per deployment environment before activation. `limit_req_zone` directive goes in `http {}` context (noted in file comment).

---

## Risks

| Risk | Severity | Notes |
|---|---|---|
| Pre-existing `TestCreateSelfServiceCompany_FeatureFlagOff` | Low | Unrelated to this feature. Documented in Batch 1. No action needed. |
| Nginx `.example.conf` requires manual adaptation per environment | Low | Clear instructions in file. Deploy team responsibility. |
| `r.RemoteAddr` vs `X-Forwarded-For` for rate limiting | Low | Nginx uses `$binary_remote_addr` which reads from TCP connection, not header. Safe for reverse proxy setup. Go handler logs `r.RemoteAddr` (same as what Nginx sees after proxy). |

---

## Technical Debt

| Item | Priority |
|---|---|
| `deploy/nginx/rate-limit-lookup.conf.example` — `limit_req_zone` goes in http {} context separately | Low — documented in file, ops responsibility |
| `r.RemoteAddr` in audit log may show Nginx IP in production (if not configuring `real_ip_module`) | Low — standard behavior across all existing handlers |

---

## Blockers

None.

---

## Ready For Batch 3

**YES**

All Batch 2 acceptance criteria met:
- ✅ A4: `AdminHandler.listedLookup` field + `WithListedCompaniesLookup` method + route registered
- ✅ A5: Handler with input validation (7 rejection cases), audit logging (all required fields, no PII), per-response Cache-Control, nil-safe service call
- ✅ A6: Server wiring `adminHandler.WithListedCompaniesLookup(listedCompaniesSvc)` — correct startup order
- ✅ A7: Nginx rate limit config committed to repo as `deploy/nginx/rate-limit-lookup.conf.example`
- ✅ 12 new handler tests, all passing including security and Cache-Control tests
- ✅ Race detector clean
- ✅ `go build ./...` and `go vet ./...` clean

Batch 3 scope (per execution plan): Frontend — authApi lookup method, useListedCompanyLookup hook, ListedCompanyPreviewCard component, provisionErrors update, CreateCompanyModal integration.
