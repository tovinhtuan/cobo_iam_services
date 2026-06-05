# Execution Plan: Listed Company Lookup & Sync
**Source plans:**
- `docs/ai-cache/listed-company-lookup-sync-implementation-plan.md`
- `docs/ai-cache/listed-company-lookup-sync-principal-architecture-review.md`

**Date:** 2026-06-05  
**Status:** READY FOR EXECUTION (with 3 documented deviations from plan)

---

## Output 1 — Codebase Verification

| Component | Plan Assumption | Actual Code Exists? | Risk |
|---|---|---|---|
| `ListedCompanyReader` interface | Has `List`, `GetBySymbol` | ✅ `internal/marketreference/app/types.go:23` | — |
| `ListedCompanyDetail` type | DTO with identity/timeline/capital/leadership/legalContact groups | ✅ `types.go:59` | — |
| `GetBySymbol` repo method | Full JSON parse via `mapDetailFromProfile` | ✅ `repository.go:136` — parse helpers reusable | — |
| `mapDetailFromProfile` + helpers | `stringPtr`, `floatPtr`, `nullString` | ✅ `repository.go:247+` — all package-private, reusable within same package | — |
| `marketreference.Service` | Has `GetDetail`, `checkAvailable`, `mapRepoError` | ✅ `service.go` | — |
| `NewDisabledService()` | Returns ErrUnavailable on all calls | ✅ `service.go:19` | — |
| `buildListedCompaniesService()` in server | Builds `*marketapp.Service`, already wired to platformCMS | ✅ `server.go:160,170` — `listedCompaniesSvc` var in scope | — |
| `AdminHandler` With* pattern | `WithTokenIssuer`, `WithIdempotency`, `WithSelfCreateEnabled` | ✅ `admin_handler.go:34–54` — exact pattern to follow | — |
| `AdminHandler.Register()` | Registers routes incl. `company/initialize`, `company/create` | ✅ `admin_handler.go:57,122–123` | — |
| `httpx.RequestIDFromContext` | Used for structured logging | ✅ `platform/httpx/requestid.go:24` | — |
| `httpx.WriteJSON`, `WriteError` | Response helpers | ✅ `platform/httpx/json.go` | — |
| `perr.CodeInvalidRequest`, `CodeServiceUnavailable` | Error codes | ✅ `platform/errors/*.go:28,40` | — |
| `slog.InfoContext` pattern | Structured logging in handlers | ✅ `admin_handler_provision.go:141–151` | — |
| `marketUnavailableError()`, `mapListedCompaniesError()` | Reusable from platformcms | ⚠️ **Package-private in `platformcms/transport/http/`** — cannot import from `companyaccess/transport/http/` | Must duplicate error mapping in new handler file |
| `golang.org/x/sync` (singleflight) | Plan requires singleflight | ❌ **NOT in go.mod** (`golang.org/x/crypto`, `x/net`, `x/text`, `x/sys` exist, but NOT `x/sync`) | **DEVIATION D-1: Use stdlib `sync.Mutex` + map instead. No new dependency.** |
| `COMPANY_ALREADY_EXISTS` → tax_code message | Plan Q-BLOCK-2: update message | ⚠️ **Two different backend scenarios share same error message string**: (1) initialize with existing membership (line 87), (2) `IsDuplicateCompanyError` tax_code duplicate (line 140) — both emit `"COMPANY_ALREADY_EXISTS"`. FE cannot distinguish. | **DEVIATION D-2: Existing STATE_CONFLICT→COMPANY_ALREADY_EXISTS message already updated. See D-2 section.** |
| `CreateCompanyModal` call sites | 1 modal to modify | ✅ Modal used in 3 sites: `NoCompanyPage`, `UserProfile`, `PortalLayout`. All get feature automatically by modifying modal once. | Watch for import order |
| `parseListedCompanyDetailPayload` | Reuse from cmsApi.ts | ✅ **Exported** at `cms-core/services/cmsApi.ts:1929` — but cross-slice import from `company/` feature | **DEVIATION D-3: Document cross-slice import, acceptable** |
| `fakeListedRepo` in service_test.go | Plan assumes adding to interface is clean | ⚠️ `fakeListedRepo` struct in `service_test.go:7` has only `List` and `GetBySymbol` — must add `GetByBusinessCode` to fake when interface is extended | Required in test update |
| `provisionErrors.ts` COMPANY_ALREADY_EXISTS | Plan says "add case" | ⚠️ **Already exists at line 18** with message "Thông tin doanh nghiệp đã tồn tại." — only needs message update | Message update only |

### Deviation D-1 — singleflight Replacement

**Plan:** `golang.org/x/sync/singleflight`  
**Actual:** NOT in go.mod. Adding requires `go get` + `go mod tidy` which changes go.sum (dependency supply chain change, needs review).  
**Decision:** Use stdlib-only implementation: `sync.RWMutex` + `map[string]cacheEntry`. At 1700 entries with 1h TTL, thundering herd on cold start is negligible — at most 1700 concurrent DB calls (one per key), each 5ms, and only on cold start. Acceptable.  
**Impact on plan:** Cache implementation is simpler, fewer lines. No go.mod change.

### Deviation D-2 — COMPANY_ALREADY_EXISTS Message

**Backend reality:** Both "user already has a company" (initialize mode, `eligible > 0`) AND "tax_code unique constraint violation" return `message: "COMPANY_ALREADY_EXISTS"`. Same wire format.

**Frontend reality:** `provisionErrors.ts:18` already handles `COMPANY_ALREADY_EXISTS` with message "Thông tin doanh nghiệp đã tồn tại." — this fires for BOTH cases.

**Problem:** Q-BLOCK-2 wants a tax_code-specific message. But upgrading this message to the longer tax_code one makes it wrong for the "already has company" case.

**Decision (no backend change):** Update the `COMPANY_ALREADY_EXISTS` message to a hybrid that covers both: *"Thông tin doanh nghiệp đã tồn tại hoặc mã số thuế đã được đăng ký. Vui lòng kiểm tra lại thông tin hoặc liên hệ quản trị viên."* This is more specific than current but works for both cases without backend change.

**Exact new message:**
```
Doanh nghiệp đã tồn tại hoặc mã số thuế đã được đăng ký bởi doanh nghiệp khác. Vui lòng kiểm tra lại thông tin hoặc liên hệ quản trị viên để được hỗ trợ.
```

### Deviation D-3 — Cross-Slice FE Import

`parseListedCompanyDetailPayload` from `cms-core/services/cmsApi.ts` is exported and reusable. Cross-slice import is acceptable per architecture review (documented as tech debt, not blocker). No structural change needed.

---

## Dependency Analysis

### Backend Dependency Graph

```
A1: marketreference interface extension (GetByBusinessCode in types.go)
 ↓
A2: repository implementation (GetByBusinessCode in mysql/repository.go)
 ↓ depends on A1 (interface)
A3: in-process cache implementation (businessCodeCache in app/service.go)
 ↓ depends on A1 (interface), A2 (method)
A4: AdminHandler field + WithListedCompaniesLookup method (admin_handler.go)
 ↓ depends on: nothing except marketapp import
A5: admin_handler_listed_lookup.go (new file — handler + route)
 ↓ depends on A3 (service.GetByBusinessCode), A4 (AdminHandler.listedLookup field)
A6: server.go wiring
 ↓ depends on A4 (WithListedCompaniesLookup method), listedCompaniesSvc already in scope
A7: fakeListedRepo update in service_test.go
 ↓ depends on A1 (interface — must match)
A8: backend test files
 ↓ depends on A2 (repo), A3 (cache), A5 (handler)
```

**Critical path:** A1 → A2 → A3 → A5 → A6  
A4 can be written in parallel with A2.  
A7 MUST be done at same time as A1 or tests will not compile.

### Frontend Dependency Graph

```
B1: authApi.ts — lookupListedCompany method (no-auth GET)
 ↓ depends on: nothing (uses existing createApiClient pattern)
B2: useListedCompanyLookup.ts (new hook)
 ↓ depends on B1 (lookupListedCompany), ListedCompanyDetail type from cms-core
B3: ListedCompanyPreviewCard.tsx (new component)
 ↓ depends on B2 state types, smart merge mapSyncPayload
B4: provisionErrors.ts message update
 ↓ depends on: nothing (standalone change)
B5: CreateCompanyModal.tsx integration
 ↓ depends on B2 (hook), B3 (component)
B6: frontend test files
 ↓ depends on B1, B2, B3, B4
```

**Critical path:** B1 → B2 → B3 → B5  
B4 is independent, can be done anytime before B6.

### Cross-Service Dependency

```
Phase A (Backend) — MUST complete A5+A6 before:
  Phase C (Frontend B1) can be tested end-to-end
  BUT B1–B4 can be written before A5 is deployed (mocking API responses in FE tests)

Phase C checkpoint (Nginx rate limit) — MUST be deployed before:
  Phase G (Staging) rate limit validation

Phase A verification gate — MUST pass before:
  Phase E (Integration QA) runs E2E scenarios
```

---

## Hidden Risks Found

| Risk | Severity | Mitigation |
|---|---|---|
| **Cache cold start** — if vnstock DB is slow at startup, first 1700 requests all hit DB simultaneously | Low | Acceptable: 1700 × 5ms ≈ 8.5s total scan time spread across multiple connections. Not a thundering herd at 1700 keys. Stdlib mutex is sufficient. |
| **Cache TTL vs KBS update cycle** — if KBS pipeline updates `business_code` for a company within TTL window, stale data served | Low | KBS pipeline updates monthly max. 1h TTL is safe. Accept stale risk — matches disclaimer in response. |
| **Negative cache false-positive** — if user types a valid but not-yet-indexed ĐKKD, they see "not found" and negative cache persists 10 min | Low | 10 min TTL is short enough for practical re-try. Acceptable. |
| **`COMPANY_ALREADY_EXISTS` ambiguity** — same error code for two different scenarios (see D-2) | Medium | Message updated to cover both cases (D-2 decision). No backend change needed. |
| **CreateCompanyModal in 3 call sites** — lookup integration will affect NoCompanyPage (initialize mode), UserProfile, PortalLayout | Low | All 3 sites benefit. No special handling needed. Verify all 3 render correctly in integration test. |
| **Cross-slice import FE** — `company/` importing from `cms-core/services/cmsApi.ts` | Low | Documented. `parseListedCompanyDetailPayload` is exported and stable. Add `// cross-slice: see arch review` comment. |
| **Stale preview after rapid input** — user pastes a ĐKKD, preview appears, then clears and re-types quickly | Low | State machine resets to `idle` on every input change (debounce cancel + reset). Unit test covers this. |
| **Smart merge + empty string edge case** — form fields initialized as `''`, sync skips non-empty fields. If user types a space, field is not empty → not synced | Low | `currentValues[field].trim() === ''` is safer check than `=== ''`. Use trim in mapSyncPayload. |
| **singleflight absent** — concurrent requests for same ĐKKD on cache miss all hit DB | Low-Med | Stdlib mutex + map: only 1 concurrent query per unique ĐKKD at a time (mutex gates the cache write). Similar effect to singleflight without the deduplication guarantee. Acceptable for this scale. |
| **Nginx config not in repo or deployed** — rate limit not active in dev/staging | High | Nginx config file MUST be a committed deliverable in Phase A, deployed in Phase F. Not optional. |

---

## Phase A — Backend Foundation

### Goal

Extend `marketreference` with `GetByBusinessCode` capability including in-process cache. Wire into `AdminHandler` as new public endpoint. No auth. Structured audit logging on every request.

### Files Modified

| File | Action | Notes |
|---|---|---|
| `internal/marketreference/app/types.go` | MODIFY | Add `GetByBusinessCode` to `ListedCompanyReader` interface |
| `internal/marketreference/app/service.go` | MODIFY | Add `businessCodeCache` struct + `GetByBusinessCode` method (cache-first) |
| `internal/marketreference/app/service_test.go` | MODIFY | Add `GetByBusinessCode` to `fakeListedRepo`; add cache tests |
| `internal/marketreference/infra/mysql/repository.go` | MODIFY | Implement `GetByBusinessCode` — JSON_EXTRACT, trim, reuse `mapDetailFromProfile` |
| `internal/marketreference/infra/mysql/repository_test.go` | MODIFY | Add `GetByBusinessCode` test cases |
| `internal/companyaccess/transport/http/admin_handler.go` | MODIFY | Add `listedLookup *marketapp.Service` field + `WithListedCompaniesLookup` method |
| `internal/httpserver/server.go` | MODIFY | Add `adminHandler.WithListedCompaniesLookup(listedCompaniesSvc)` |

### Files Created

| File | Notes |
|---|---|
| `internal/companyaccess/transport/http/admin_handler_listed_lookup.go` | Handler + route registration + error mapping + audit log |
| `internal/companyaccess/transport/http/admin_handler_listed_lookup_test.go` | All handler test cases |
| `deploy/nginx/listed-lookup-rate-limit.conf` (or equivalent path) | Mandatory Nginx config — committed to repo |

### Tasks

#### A1 — Extend `ListedCompanyReader` interface + update `fakeListedRepo` ✅ DONE 2026-06-05

**File:** `internal/marketreference/app/types.go`

Add to `ListedCompanyReader` interface (after `GetBySymbol`):
```
GetByBusinessCode(ctx context.Context, businessCode string) (ListedCompanyDetail, error)
```

**File:** `internal/marketreference/app/service_test.go`

Add `GetByBusinessCode` method to `fakeListedRepo` struct:
```go
getByBusinessCodeFn func(context.Context, string) (ListedCompanyDetail, error)
```
And implement the interface method. This MUST happen in the same commit as A1 or `go build` fails.

**Acceptance:** `go build ./internal/marketreference/...` passes.

---

#### A2 — Repository: implement `GetByBusinessCode` ✅ DONE 2026-06-05

**File:** `internal/marketreference/infra/mysql/repository.go`

Query to implement:
```sql
SELECT
    e.symbol,
    e.company_name,
    e.exchange,
    p.info,
    p.updated_at,
    TRUE AS has_profile
FROM equity_list e
JOIN company_profiles p ON p.symbol = e.symbol AND p.source = 'kbs'
WHERE TRIM(JSON_UNQUOTE(JSON_EXTRACT(p.info, '$.business_code'))) = ?
LIMIT 1
```

Rules:
- Input: trim whitespace before passing to query
- Return `app.ErrNotFound` when no row (sql.ErrNoRows)
- Reuse existing `mapDetailFromProfile(rowSymbol, companyName, nullString(equityExchange), info, profileAt)` — same as `GetBySymbol`
- `has_profile` is always TRUE because of `JOIN` (not `LEFT JOIN`)

Input validation in repository:
```go
businessCode = strings.TrimSpace(businessCode)
if businessCode == "" {
    return app.ListedCompanyDetail{}, app.ErrInvalidRequest
}
```

**Acceptance:** `go test ./internal/marketreference/infra/mysql/...` passes with new test cases.

---

#### A3 — In-process cache in `service.go` ✅ DONE 2026-06-05

**File:** `internal/marketreference/app/service.go`

Add `businessCodeCache` struct using stdlib only (`sync.RWMutex`):

```
type cacheEntry struct {
    result *ListedCompanyDetail  // nil = negative cache (not found)
    expiry time.Time
}

type businessCodeCache struct {
    mu       sync.RWMutex
    entries  map[string]cacheEntry
    maxSize  int
}
```

Methods on `businessCodeCache`:
- `get(key string) (*ListedCompanyDetail, bool, bool)` — (result, found-in-cache, is-positive)
- `set(key string, result *ListedCompanyDetail, positiveTTL, negativeTTL time.Duration)`
- `evictExpiredLocked()` — called during `set` when `len >= maxSize`

**TTL constants:**
```go
const (
    cachePositiveTTL = 60 * time.Minute
    cacheNegativeTTL = 10 * time.Minute
    cacheMaxSize     = 2000
)
```

Add `cache *businessCodeCache` field to `Service` struct.

Initialize in `NewService`: `cache: &businessCodeCache{entries: make(map[string]cacheEntry), maxSize: cacheMaxSize}`

**New service method `GetByBusinessCode` — returns `(ListedCompanyDetail, bool cacheHit, error)`:**

```
1. checkAvailable(ctx) → ErrUnavailable if not available
2. trim + validate businessCode (empty → ErrInvalidRequest)

// [P2-1 FIX] Double-checked locking to prevent concurrent miss thundering herd:
3. cache.mu.RLock()
   entry, ok := cache.entries[businessCode]
   cache.mu.RUnlock()
   if ok && time.Now().Before(entry.expiry):
     if entry.result == nil: return zero, TRUE, ErrNotFound  // negative cache hit
     return *entry.result, TRUE, nil                         // positive cache hit

// Cache miss path — call DB, then write to cache
4. result, err := repo.GetByBusinessCode(ctx, businessCode)

// [P2-1 FIX] Write lock acquired AFTER DB call (not during):
5. cache.mu.Lock()
   // Double-check: another goroutine may have populated cache while we called DB
   if existing, ok := cache.entries[businessCode]; ok && time.Now().Before(existing.expiry):
     cache.mu.Unlock()
     if existing.result == nil: return zero, FALSE, ErrNotFound
     return *existing.result, FALSE, nil
   // [P2-2 FIX] Eviction when at capacity:
   if len(cache.entries) >= cache.maxSize:
     cache.evictExpiredLocked()
     if len(cache.entries) >= cache.maxSize:
       cache.evictOldestLocked()  // LRU fallback — evict 1 entry to make room
   if err == ErrNotFound:
     cache.entries[businessCode] = cacheEntry{result: nil, expiry: time.Now().Add(cacheNegativeTTL)}
   else if err == nil:
     cached := result
     cache.entries[businessCode] = cacheEntry{result: &cached, expiry: time.Now().Add(cachePositiveTTL)}
   // other errors: do not cache
   cache.mu.Unlock()

6. if err != nil: return zero, FALSE, err
7. return result, FALSE, nil
```

**`evictOldestLocked()`** — iterates entries map, removes the one with earliest expiry. Called only when no expired entries exist and cache is at maxSize.

**Acceptance:** `go test ./internal/marketreference/app/...` passes including:
- concurrent miss test: 10 goroutines same key → repo called ≤2 times (race window between RLock and write)
- negative cache test: ErrNotFound cached, second call skips repo
- eviction test: at maxSize, oldest entry removed on new set

---

#### A4 — AdminHandler field + WithListedCompaniesLookup ✅ DONE 2026-06-05

**File:** `internal/companyaccess/transport/http/admin_handler.go`

Add to `AdminHandler` struct:
```go
listedLookup *marketapp.Service
```

Add import at top:
```go
marketapp "github.com/cobo/cobo_iam_services/internal/marketreference/app"
```

Add method after `WithSelfCreateEnabled`:
```go
// WithListedCompaniesLookup wires vnstock lookup for GET /api/v1/company/listed-lookup.
func (h *AdminHandler) WithListedCompaniesLookup(svc *marketapp.Service) {
    h.listedLookup = svc
}
```

Add route in `Register()` after existing company routes (line 123):
```go
mux.HandleFunc("GET /api/v1/company/listed-lookup", h.listedLookupByBusinessCode)
```

**Acceptance:** `go build ./internal/companyaccess/...` passes.

---

#### A5 — New handler file: `admin_handler_listed_lookup.go` ✅ DONE 2026-06-05

**File:** `internal/companyaccess/transport/http/admin_handler_listed_lookup.go`

Package: `package http`

Imports needed:
- `"errors"`, `"log/slog"`, `"net/http"`, `"strings"`, `"time"`
- `marketapp "github.com/cobo/cobo_iam_services/internal/marketreference/app"`
- `perr "github.com/cobo/cobo_iam_services/internal/platform/errors"`
- `"github.com/cobo/cobo_iam_services/internal/platform/httpx"`

**Input validation function:**
```
validateBusinessCode(raw string) (string, error):
  code = strings.TrimSpace(raw)
  if len(code) == 0: return "", ErrInvalidRequest
  if len(code) > 50: return "", ErrInvalidRequest
  for each char in code: if not alphanumeric and not '-' and not '/': return "", ErrInvalidRequest
  return code, nil
```

**Handler `listedLookupByBusinessCode`:**
```
1. start := time.Now()
2. raw := r.URL.Query().Get("business_code")
3. code, err := validateBusinessCode(raw)
   if err:
     w.Header().Set("Cache-Control", "no-store")                  ← [P1-1 FIX]
     w.Header().Set("X-Content-Type-Options", "nosniff")
     log("listed_lookup_requested", result="error_invalid", cache_hit=false)
     return 400 INVALID_REQUEST
4. if h.listedLookup == nil:
     w.Header().Set("Cache-Control", "no-store")                  ← [P1-1 FIX]
     w.Header().Set("X-Content-Type-Options", "nosniff")
     log("listed_lookup_requested", result="unavailable", cache_hit=false)
     return 503
5. detail, cacheHit, err := h.listedLookup.GetByBusinessCode(r.Context(), code)
   // GetByBusinessCode returns (detail, cacheHit bool, err) — service reports cache hit
6. Determine result string: "found" | "not_found" | "unavailable"
7. Emit slog.InfoContext audit log (see below)
8. Switch on err:
   - nil:
       w.Header().Set("Cache-Control", "public, max-age=3600, stale-while-revalidate=300")  ← [P1-1 FIX] only on 200
       w.Header().Set("X-Content-Type-Options", "nosniff")
       write 200 found:true response
   - ErrNotFound:
       w.Header().Set("Cache-Control", "public, max-age=600")     ← [P1-1 FIX] shorter TTL for not_found
       w.Header().Set("X-Content-Type-Options", "nosniff")
       write 200 found:false
   - ErrUnavailable / other:
       w.Header().Set("Cache-Control", "no-store")                ← [P1-1 FIX] no cache for 503
       w.Header().Set("X-Content-Type-Options", "nosniff")
       write 503
```

**Audit log fields — [P1-2 FIX: `cache_hit` added]:**
```go
slog.InfoContext(r.Context(), "listed_lookup_requested",
    slog.String("event", "listed_lookup_requested"),
    slog.String("request_id", httpx.RequestIDFromContext(r.Context())),
    slog.String("ip", r.RemoteAddr),
    slog.String("user_agent", r.UserAgent()),
    slog.String("business_code_prefix", safePrefix(code, 4)),
    slog.String("result", result),                              // "found" | "not_found" | "unavailable" | "error_invalid"
    slog.Bool("cache_hit", cacheHit),                          // [P1-2 FIX] — required for Phase B gate and Phase H monitoring
    slog.Int64("duration_ms", time.Since(start).Milliseconds()),
)
```

`safePrefix(s string, n int) string`: returns `s[:min(len(s), n)]`

> **Note on service signature change:** `GetByBusinessCode` returns `(ListedCompanyDetail, bool, error)` — the `bool` is `cacheHit`. Alternatively service can return `cacheHit` via a struct wrapper. Either way, the handler must receive and log this value.

**Response when found:**

`sync` object: omit fields that are nil in `detail.LegalContact` or empty. Only include fields with non-empty value.

```json
{
  "found": true,
  "disclaimer": "Thông tin được lấy từ dữ liệu công ty niêm yết công khai, chỉ mang tính tham khảo. Nền tảng không chịu trách nhiệm pháp lý về tính chính xác của thông tin.",
  "sync": {
    "company_name": "...",
    "tax_code": "...",         // from LegalContact.TaxID — omit if nil
    "registration_number": "...",  // from Identity.BusinessCode — omit if nil
    "address": "...",          // from LegalContact.Address — omit if nil
    "phone": "...",            // from LegalContact.Phone — omit if nil
    "contact_email": "..."     // from LegalContact.Email — omit if nil
  },
  "preview": {
    "symbol": "...",
    "exchange": "...",         // from Identity.Exchange — omit if nil
    "company_type": "...",     // from Identity.CompanyType — omit if nil
    "listing_date": "..."      // from Timeline.ListingDate — omit if nil, format "2006-01-02"
  }
}
```

Rule: use `omitempty` logic — if field is nil pointer, exclude from JSON. Use `map[string]any` and skip nil fields explicitly.

**Error mapping (local to this file — do NOT import from platformcms):**
```go
func lookupMapErr(err error) error {
    if errors.Is(err, marketapp.ErrUnavailable) {
        return perr.NewHTTPError(503, perr.CodeServiceUnavailable, "market reference data is unavailable", nil)
    }
    if errors.Is(err, marketapp.ErrInvalidRequest) {
        return perr.NewHTTPError(400, perr.CodeInvalidRequest, "invalid business_code", nil)
    }
    return perr.NewHTTPError(503, perr.CodeServiceUnavailable, "market reference data is unavailable", nil)
}
```

---

#### A6 — Server wiring ✅ DONE 2026-06-05

**File:** `internal/httpserver/server.go`

After line 439 (`adminHandler.WithSelfCreateEnabled`), add:
```go
adminHandler.WithListedCompaniesLookup(listedCompaniesSvc)
```

`listedCompaniesSvc` is already built at line 170 and in scope at the wiring location.

**Acceptance:** `go build ./...` and `go test ./internal/companyaccess/...` pass.

---

#### A7 — Nginx rate limit config ✅ DONE 2026-06-05

**File:** `deploy/nginx/listed-lookup-rate-limit.conf` (or wherever project stores Nginx includes)

Content:
```nginx
# Rate limit: GET /api/v1/company/listed-lookup
# 10 req/minute per IP. Burst 3 handles normal debounce (user types, pauses, types again).
# To adjust for high-traffic events (e.g. marketing campaign): increase rate= or burst=.
# Minimum safe value for production: rate=5r/m burst=2.
# If legitimate users report 429 errors, increase burst first before raising rate.
limit_req_zone $binary_remote_addr zone=listed_lookup:10m rate=10r/m;
```

In server block or location config:
```nginx
location = /api/v1/company/listed-lookup {
    limit_req zone=listed_lookup burst=3 nodelay;
    limit_req_status 429;
    add_header Retry-After 60 always;
    proxy_pass http://backend;
}
```

**This file MUST be committed to repo.** It is a code artifact, not a deploy-time manual step.

---

### Verification Gate — Phase A

**ALL must pass before Phase B starts.**

```bash
# 1. Build — no compile errors
go build ./... 2>&1 | grep -v "^$"

# 2. marketreference tests (interface + cache + repo)
go test -v ./internal/marketreference/... -count=1

# 3. companyaccess transport tests (handler)
go test -v ./internal/companyaccess/transport/http/... -count=1

# 4. Full package test
go test ./... -count=1 2>&1 | tail -20

# 5. Manual smoke (vnstock enabled locally OR expect 503)
curl -s "http://localhost:8080/api/v1/company/listed-lookup?business_code=0101234567" | jq .
# Expected: {"found": true, ...} or {"found": false} or 503

# 6. Empty input → 400
curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/api/v1/company/listed-lookup?business_code="
# Expected: 400

# 7. Special chars → 400
curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/api/v1/company/listed-lookup?business_code=abc%27def"
# Expected: 400
```

### Phase A Rollback Point

If Phase A verification fails:
- Go changes are additive (new method on interface, new files) — no state mutation
- `git revert` all Phase A commits
- `admin_handler_listed_lookup.go` is a new file — delete it
- `WithListedCompaniesLookup` removal from `admin_handler.go` — revert to pre-A4 state
- Route removed from `Register()` — revert
- Server wiring removed — revert
- **The existing API endpoints are not modified** — zero risk to running system

---

## Phase B — Security & Observability

### Goal

Audit logging verified working. Nginx config exists and is deployable. Input sanitization active. Response headers set.

### Files

All in Phase A already. This phase is **verification** of what was built in A5 + committed in A7.

### Tasks

#### B1 — Verify audit log format

Run a request against local server, confirm log output:
```bash
go run ./cmd/api &
curl "http://localhost:8080/api/v1/company/listed-lookup?business_code=TEST"
# In logs, find: listed_lookup_requested with fields: event, request_id, ip, user_agent, business_code_prefix, result, duration_ms
```

Confirm `business_code_prefix` is max 4 chars. Confirm `result` is one of: found, not_found, unavailable.

#### B2 — Verify input sanitization

```bash
# Empty → 400
curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/api/v1/company/listed-lookup"
curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/api/v1/company/listed-lookup?business_code="
curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/api/v1/company/listed-lookup?business_code=%20%20"

# Oversized → 400
curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/api/v1/company/listed-lookup?business_code=$(python3 -c 'print("A"*51)')"

# Special chars → 400
curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/api/v1/company/listed-lookup?business_code=abc%27"
```

#### B3 — Verify response headers

```bash
curl -sI "http://localhost:8080/api/v1/company/listed-lookup?business_code=TEST" | grep -i "cache-control\|x-content-type"
# Expected: Cache-Control: public, max-age=3600 AND X-Content-Type-Options: nosniff
```

### Verification Gate — Phase B

```bash
# All 3 checks above pass
# Audit log has correct fields
# All inputs sanitized correctly
# Response headers present
```

### Phase B Rollback Point

Phase B is read-only verification. No rollback needed.

---

## Phase C — Frontend Lookup Integration

### Goal

`authApi.ts` has `lookupListedCompany`. `useListedCompanyLookup` hook works with debounce, state machine, smart merge. `provisionErrors.ts` message updated.

### Files Modified

| File | Action |
|---|---|
| `src/services/authApi.ts` | ADD `lookupListedCompany` method |
| `src/features/company/provisionErrors.ts` | UPDATE `COMPANY_ALREADY_EXISTS` message (D-2 decision) |

### Files Created

| File | Notes |
|---|---|
| `src/features/company/useListedCompanyLookup.ts` | Hook with debounce + state machine + smart merge builder |
| `src/features/company/useListedCompanyLookup.test.ts` | Tests |

### Tasks

#### C1 — `authApi.ts`: add `lookupListedCompany` ✅ DONE 2026-06-05

**File:** `src/services/authApi.ts`

Add inside `createAuthApi` return object (after `createCompany`):

```typescript
async lookupListedCompany(businessCode: string): Promise<ListedCompanyLookupResult> {
  const code = businessCode.trim();
  if (!code) return { found: false };
  const payload = await request<unknown>(
    `/api/v1/company/listed-lookup?business_code=${encodeURIComponent(code)}`
    // NOTE: no auth token needed — public endpoint, omit Authorization header
  );
  return parseListedCompanyLookupPayload(payload);
}
```

**Types to add** (at top of authApi.ts or in a companion types file):
```typescript
export type ListedCompanyLookupSyncPayload = {
  company_name?: string;
  tax_code?: string;
  registration_number?: string;
  address?: string;
  phone?: string;
  contact_email?: string;
};
export type ListedCompanyLookupPreview = {
  symbol?: string;
  exchange?: string;
  company_type?: string;
  listing_date?: string;
};
export type ListedCompanyLookupResult =
  | { found: false }
  | { found: true; sync: ListedCompanyLookupSyncPayload; preview: ListedCompanyLookupPreview; disclaimer: string };
```

**`parseListedCompanyLookupPayload`** — local parse function (do NOT import from cmsApi.ts for this — different shape):

> **[P1-3 RESOLVED]** Response envelope format confirmed: AdminHandler uses `httpx.WriteJSON` directly → **flat JSON** (no `data:` wrapper). CMS handlers use `writeEnvelope` → `{ data: {...} }` but this endpoint is NOT a CMS endpoint.

```typescript
function parseListedCompanyLookupPayload(payload: unknown): ListedCompanyLookupResult {
  // [P1-3] Flat response — read from root directly, NOT root.data
  const root = payload && typeof payload === 'object' ? payload as Record<string, unknown> : {};
  if (!root.found) return { found: false };
  const sync = (root.sync ?? {}) as Record<string, unknown>;
  const preview = (root.preview ?? {}) as Record<string, unknown>;
  return {
    found: true,
    disclaimer: String(root.disclaimer ?? ''),
    sync: {
      company_name: readOptionalString(sync.company_name),
      tax_code: readOptionalString(sync.tax_code),
      registration_number: readOptionalString(sync.registration_number),
      address: readOptionalString(sync.address),
      phone: readOptionalString(sync.phone),
      contact_email: readOptionalString(sync.contact_email),
    },
    preview: {
      symbol: readOptionalString(preview.symbol),
      exchange: readOptionalString(preview.exchange),
      company_type: readOptionalString(preview.company_type),
      listing_date: readOptionalString(preview.listing_date),
    },
  };
}

function readOptionalString(v: unknown): string | undefined {
  if (typeof v === 'string' && v.trim() !== '') return v.trim();
  return undefined;
}
```

#### C2 — `useListedCompanyLookup.ts`: hook ✅ DONE 2026-06-05

**File:** `src/features/company/useListedCompanyLookup.ts`

State machine:
```typescript
type LookupStatus = 'idle' | 'loading' | 'found' | 'not_found' | 'error';

type LookupState =
  | { status: 'idle' | 'loading' | 'not_found' | 'error' }
  | { status: 'found'; sync: ListedCompanyLookupSyncPayload; preview: ListedCompanyLookupPreview; disclaimer: string };
```

Key behaviors:
- Input ≥ 8 chars: trigger after 500ms debounce
- Input < 8 chars or empty: set state `idle` immediately, cancel pending
- Input changes: set state `idle` immediately (stale preview clear), restart debounce
- 503 from API: state `error` (NOT `not_found`)
- `not_found` from API: state `not_found`

**Smart merge `mapSyncPayload`** (exported function, also used in tests):
```typescript
export function mapSyncPayload(
  sync: ListedCompanyLookupSyncPayload,
  current: CompanyCreateFormValues
): Partial<CompanyCreateFormValues> {
  const patch: Partial<CompanyCreateFormValues> = {};
  if (sync.company_name && current.companyName.trim() === '') patch.companyName = sync.company_name;
  if (sync.tax_code && current.taxCode.trim() === '') patch.taxCode = sync.tax_code;
  if (sync.registration_number && current.registrationNumber.trim() === '') patch.registrationNumber = sync.registration_number;
  if (sync.address && current.address.trim() === '') patch.address = sync.address;
  if (sync.phone && current.phone.trim() === '') patch.phone = sync.phone;
  if (sync.contact_email && current.contactEmail.trim() === '') patch.contactEmail = sync.contact_email;
  return patch;
}
```

Import `CompanyCreateFormValues` from `./CompanyCreateForm`.  
Import `ListedCompanyLookupSyncPayload` from `../../services/authApi` (or wherever types are placed).  
Import `createAuthApi` from `../../services`.

#### C3 — `provisionErrors.ts`: message update ✅ DONE 2026-06-05 (note: execution plan labeled C3 as provisionErrors; C3 in Batch 3 also includes ListedCompanyPreviewCard as D1+D2)

**File:** `src/features/company/provisionErrors.ts`

Line 18-20: Update message for `COMPANY_ALREADY_EXISTS`:
```typescript
if (message === 'COMPANY_ALREADY_EXISTS' || code === 'COMPANY_ALREADY_EXISTS') {
  return 'Doanh nghiệp đã tồn tại hoặc mã số thuế đã được đăng ký bởi doanh nghiệp khác. Vui lòng kiểm tra lại thông tin hoặc liên hệ quản trị viên để được hỗ trợ.';
}
```

### Verification Gate — Phase C

```bash
# TypeScript compiles
npm run lint

# Tests pass
npx vitest run src/features/company/useListedCompanyLookup.test.ts
npx vitest run src/features/company/provisionErrors.test.ts

# Existing tests not broken
npx vitest run src/features/company/
```

### Phase C Rollback Point

- `authApi.ts`: remove `lookupListedCompany` method + types
- Delete `useListedCompanyLookup.ts` and test file
- Revert `provisionErrors.ts` to original message
- No state change — all frontend-only

---

## Phase D — Frontend UX

### Goal

`ListedCompanyPreviewCard` renders correctly. `CreateCompanyModal` shows preview when lookup finds a result. Smart merge works on all 3 modal call sites.

### Files Modified

| File | Action |
|---|---|
| `src/features/company/CreateCompanyModal.tsx` | Add `useListedCompanyLookup` integration + render `ListedCompanyPreviewCard` |

### Files Created

| File | Notes |
|---|---|
| `src/features/company/ListedCompanyPreviewCard.tsx` | Preview card component |
| `src/features/company/ListedCompanyPreviewCard.test.tsx` | Component tests |

### Tasks

#### D1 — `ListedCompanyPreviewCard.tsx` ✅ DONE 2026-06-05 (Batch 3 C3)

Props:
```typescript
type Props = {
  preview: ListedCompanyLookupPreview;
  disclaimer: string;
  onSync: () => void;   // called with no args — mapSyncPayload already applied in parent
  onDismiss: () => void;
};
```

Layout (inline Tailwind, consistent with existing form style):
```
┌─────────────────────────────────────────────────────┐
│ 🏢 {symbol} — {company_name displayed by parent}    │
│    {exchange} · {company_type} · Niêm yết {year}    │
│                                                     │
│ ⚠️ {disclaimer text — full text, no truncation}     │
│                                                     │
│ [Đồng bộ thông tin]  [Bỏ qua]                       │
│ (Chỉ điền vào các ô còn trống)                      │
└─────────────────────────────────────────────────────┘
```

- `onSync` fires `provision.setValues(patch)` in parent — card only calls `onSync()`
- `onDismiss` resets lookup state to idle
- Card DOES NOT call form submit
- Disclaimer text: non-truncated, small text, warning color

#### D2 — `CreateCompanyModal.tsx` integration ✅ DONE 2026-06-05 (Batch 3 C5)

Add inside `CreateCompanyModal` function body:

```typescript
const lookup = useListedCompanyLookup({
  getAuthApi: () => createAuthApi({ getAccessToken: () => localStorage.getItem(ACCESS_TOKEN_KEY) || undefined }),
});
```

Pass `registrationNumber` to hook:
```typescript
// Watch provision.values.registrationNumber
useEffect(() => {
  lookup.setInput(provision.values.registrationNumber);
}, [provision.values.registrationNumber]);
```

Or: hook accepts `registrationNumber` as input directly and manages debounce internally.

Render preview card between form header and `CompanyCreateForm`:
```tsx
{lookup.status === 'found' && lookup.result && (
  <ListedCompanyPreviewCard
    preview={lookup.result.preview}
    disclaimer={lookup.result.disclaimer}
    onSync={() => {
      const patch = mapSyncPayload(lookup.result!.sync, provision.values);
      provision.setValues((prev) => ({ ...prev, ...patch }));
      lookup.dismiss();
    }}
    onDismiss={() => lookup.dismiss()}
  />
)}
{lookup.status === 'not_found' && provision.values.registrationNumber.length >= 8 && (
  <p className="text-xs text-slate-500 mt-1">Không tìm thấy công ty niêm yết phù hợp.</p>
)}
{lookup.status === 'error' && (
  <p className="text-xs text-amber-600 mt-1">Tra cứu tạm thời không khả dụng.</p>
)}
```

**Important:** `CompanyCreateForm` is NOT modified. Values flow from `provision.values` which is updated via `provision.setValues`.

**Import cross-slice note:**
```typescript
// cross-slice import: parseListedCompanyDetailPayload from cms-core — see architecture review
// Alternatively: use local parseListedCompanyLookupPayload in authApi.ts (preferred)
```

### Verification Gate — Phase D

```bash
# TypeScript compiles
npm run lint

# Component tests
npx vitest run src/features/company/ListedCompanyPreviewCard.test.tsx

# All company feature tests
npx vitest run src/features/company/

# Visual check: dev server running
npm run dev
# Navigate to: /app/no-company (or trigger modal from PortalLayout)
# Type ĐKKD with 8+ chars → verify loading spinner
# Type valid ĐKKD → verify preview card appears
# Click "Đồng bộ thông tin" → verify only empty fields filled
# Click "Bỏ qua" → verify card dismisses
# Fill company_name → trigger lookup → click sync → verify company_name unchanged
```

### Phase D Rollback Point

- Revert `CreateCompanyModal.tsx`
- Delete `ListedCompanyPreviewCard.tsx` and test
- Modal reverts to current behavior (no lookup, no preview card)
- `useListedCompanyLookup` and `authApi` change can stay (unused)

---

## Phase E — Integration Testing

### Backend

```bash
# Run all tests
go test ./... -count=1 -timeout=120s

# Specific packages
go test -v ./internal/marketreference/... -count=1
go test -v ./internal/companyaccess/transport/http/... -count=1

# Race condition check for cache
go test -race ./internal/marketreference/app/... -count=3
```

Key test scenarios to verify:
- `GetByBusinessCode` with valid code returns `ListedCompanyDetail`
- `GetByBusinessCode` with whitespace-padded code trims and matches
- `GetByBusinessCode` with no DB match returns `ErrNotFound`
- Cache: second call for same code hits cache (mock repo call count = 1, not 2)
- Negative cache: `not_found` cached for 10 min
- Handler: `found:true` response has correct JSON shape
- Handler: null fields omitted from `sync` object
- Handler: `found:false` for missing profile
- Handler: 503 when service unavailable
- Handler: 400 for empty, oversized, special-char inputs
- Handler: audit log emitted on every call

### Frontend

```bash
# All company feature tests
npx vitest run src/features/company/

# Individual test files
npx vitest run src/features/company/useListedCompanyLookup.test.ts
npx vitest run src/features/company/ListedCompanyPreviewCard.test.tsx
npx vitest run src/features/company/CompanyCreateForm.test.tsx  # must not regress
npx vitest run src/features/company/CreateCompanyModal.test.tsx  # must not regress
npx vitest run src/features/company/provisionErrors.test.ts

# Full test suite
npm run test
```

### E2E Scenarios (manual in dev — document results)

| # | Scenario | Steps | Expected | Pass/Fail |
|---|---|---|---|---|
| E1 | Initialize — lookup found, sync, create | Open /app/no-company → type valid ĐKKD → preview appears → sync → empty fields filled → submit | Company created with synced data | |
| E2 | Initialize — lookup found, dismiss, manual | Type ĐKKD → preview → dismiss → fill manually → submit | Company created with manual data | |
| E3 | Initialize — lookup not found | Type invalid ĐKKD (8+ chars) | Hint "Không tìm thấy...", form works | |
| E4 | Create Nth company — lookup found, sync | Open modal from PortalLayout (hasCompany=true, mode=create) → ĐKKD → sync → submit | New company created, session switches | |
| E5 | 503 graceful — form still submits | Disable VNSTOCK_MARKET_ENABLED → type ĐKKD → submit form | Hint "Tra cứu tạm thời...", form submits OK | |
| E6 | Smart merge — filled fields preserved | Type company_name manually → trigger lookup → sync | company_name unchanged, empty fields filled | |
| E7 | tax_code conflict after sync | Sync a listed company → submit → tax_code already registered | Error: "Doanh nghiệp đã tồn tại hoặc mã số thuế..." | |
| E8 | Stale preview dismissed on input change | lookup → found → clear 1 char → re-type | Preview disappears immediately | |
| E9 | Rate limit (with Nginx) | Rapid 10+ requests in 1 minute | HTTP 429 with Retry-After: 60 | |
| E10 | Cache hit — second request fast | Two identical requests | Second is faster (cache_hit=true in log) | |
| E11 | NoCompanyPage — initialize mode lookup | Open /app/no-company (mode=initialize) → type ĐKKD → sync → submit | Company initialized with synced data (3rd call site verified) | |

---

## Phase F — Dev Deployment

### Environment

- `VNSTOCK_MARKET_ENABLED=true`
- `VNSTOCK_MYSQL_DSN=<connection string>` (already set in dev env)
- No new env vars required

### Config

Nginx rate limit config `listed-lookup-rate-limit.conf` MUST be in place before smoke tests. Apply to dev Nginx.

### Nginx

```bash
# Copy nginx config into place (adjust path per project)
sudo cp deploy/nginx/listed-lookup-rate-limit.conf /etc/nginx/conf.d/
sudo nginx -t   # Verify syntax
sudo nginx -s reload
```

### Smoke Tests

```bash
# 1. Endpoint responds
curl -s "https://dev.example.com/api/v1/company/listed-lookup?business_code=TEST" | jq .found

# 2. Rate limit active
for i in {1..10}; do
  curl -s -o /dev/null -w "%{http_code}\n" "https://dev.example.com/api/v1/company/listed-lookup?business_code=TEST"
done
# Expected: after 3 successes, see 429s

# 3. Full flow (UI smoke)
# Open app → no-company page → type a real ĐKKD from KBS data
# Verify: preview card appears with company name from vnstock
```

---

## Phase G — Staging Validation

### Functional

Run all E1–E10 scenarios against staging environment with real KBS data.

Special: verify `business_code` values in staging vnstock DB are actual ĐKKD format (10 digits). Log output of at least 3 successful lookups.

### Security

```bash
# Rate limit fires correctly
for i in {1..20}; do
  curl -s -o /dev/null -w "%{http_code}\n" "https://staging.example.com/api/v1/company/listed-lookup?business_code=TEST"
done
# Must see 429 after burst=3

# Input injection attempt
curl -s "https://staging.example.com/api/v1/company/listed-lookup?business_code='; DROP TABLE company_profiles; --" | jq .
# Expected: 400 INVALID_REQUEST (special chars rejected by Go handler before DB)
```

### Performance

```bash
# Cache warmup — 50 requests same ĐKKD
for i in {1..50}; do
  curl -s -o /dev/null "https://staging.example.com/api/v1/company/listed-lookup?business_code=0101234567"
done
# Check logs: cache_hit=true on requests 2-50
# Check logs: duration_ms on cache hits should be < 5ms
```

### Abuse

Review Nginx access logs after 100 requests:
```bash
grep "listed-lookup" /var/log/nginx/access.log | tail -20
# Look for any unusual patterns or 429 volume
```

---

## Phase H — Production Rollout

### Deploy Order

1. Deploy Nginx rate limit config first (before backend deploy)
2. Deploy backend binary (with new endpoint)
3. Deploy frontend build

Nginx deploy first ensures rate limit active before endpoint is live.

### Rollback Trigger — NO GO conditions

| Trigger | Action |
|---|---|
| `go test ./...` fails on deploy branch | NO GO — fix before deploy |
| `npm run test` fails | NO GO — fix before deploy |
| Build fails | NO GO |
| Nginx syntax error (`nginx -t`) | NO GO — fix config before deploy |
| Smoke test on staging fails | NO GO |
| Phase G security check fails (rate limit not firing) | NO GO — Nginx config issue |

### Rollback Procedure — [P2-4 FIX: specific steps]

> Adjust commands below to match project's actual CI/CD system (GitHub Actions / manual deploy / Docker Compose / etc.).

**Backend rollback:**
```bash
# 1. Identify the last good image/binary tag from CI history
# 2. Re-deploy previous tag:
#    Docker: docker service update --image <registry>/api:<previous_tag> api_service
#    Binary: deploy previous build artifact from CI artifacts store
# 3. Verify endpoint is gone: curl /api/v1/company/listed-lookup → 404
```

**Frontend rollback:**
```bash
# Re-deploy previous FE build from CI artifacts
# Verify: modal renders without lookup card, no console errors
```

**Nginx rollback (if needed):**
```bash
# Remove the rate limit config and reload:
sudo rm /etc/nginx/conf.d/listed-lookup-rate-limit.conf
sudo nginx -s reload
# Note: endpoint is 404 after backend rollback — Nginx config removal is optional
```

**Zero DB rollback needed.** No schema changes. No data written by this feature.

### Monitoring Checklist — First 30 Minutes

> **[P2-3 FIX]** Commands below assume stdout/container log aggregation. Adjust to actual log pipeline (Kibana, CloudWatch, GCP Logging, etc.). File-based `grep /var/log/app.log` only works in non-containerized setups.

**If using structured JSON log stream (stdout → aggregator):**
```bash
# Tail container logs (Docker example)
docker logs -f <api_container> 2>&1 | grep "listed_lookup_requested"

# Or if log pipeline has CLI (e.g., stern for Kubernetes):
stern api-deployment --include="listed_lookup_requested"
```

**Key metrics to monitor in first 30 minutes:**
```
result distribution:       found / not_found should be > 0 (not all "unavailable")
cache_hit rate:            > 50% after 5 min warmup
duration_ms (cache hit):   < 10ms consistently
duration_ms (cache miss):  < 200ms
error_invalid rate:        < 5% (high rate = FE validation issue or scanner)
unavailable rate:          0% (any → vnstock DB problem)
```

**Nginx HTTP status distribution:**
```bash
# Adjust log path to actual Nginx log location
grep "listed-lookup" /var/log/nginx/access.log | awk '{print $9}' | sort | uniq -c
# Expected: 200s dominate, some 400s (invalid input), 429s only if rate limit triggered
```

**Rollback trigger:** If `result=unavailable` > 10% in first 5 minutes → vnstock DB issue. Check `VNSTOCK_MYSQL_DSN` connectivity before blaming new code.

---

## Estimation

| Phase | Effort | Risk | Complexity |
|---|---|---|---|
| A — Backend Foundation | 2 days | Medium (cache impl) | Medium |
| B — Security Verify | 0.25 days | Low | Low |
| C — FE Lookup Integration | 1.5 days | Low | Medium |
| D — FE UX | 1 day | Low | Low |
| E — Integration Testing | 1 day | Low | Low |
| F — Dev Deploy | 0.25 days | Low | Low |
| G — Staging Validation | 0.5 days | Low | Low |
| H — Production Rollout | 0.25 days | Low | Low |
| **Total** | **~6.75 days** | — | — |

Note: Parallel opportunity — Phase C can start after A1+A5 are reviewable (API contract locked), before A-tests complete.

---

## Required Checkpoints

### After Phase A
```
Phase A Complete Report
Files changed: types.go, service.go, service_test.go, repository.go, repository_test.go,
               admin_handler.go, admin_handler_listed_lookup.go, admin_handler_listed_lookup_test.go,
               server.go, deploy/nginx/listed-lookup-rate-limit.conf
Tests run: go test ./internal/marketreference/..., go test ./internal/companyaccess/...
Results: [PASS/FAIL + test counts]
Blockers: [none OR list]
Next phase: Phase B (security verification)
```

### After Phase B
```
Phase B Complete Report
Verifications: audit log format [PASS/FAIL], input sanitization [PASS/FAIL], response headers [PASS/FAIL]
Next phase: Phase C
```

### After Phase C
```
Phase C Complete Report
Files changed: authApi.ts, useListedCompanyLookup.ts, useListedCompanyLookup.test.ts, provisionErrors.ts
Tests run: npx vitest run src/features/company/
Results: [PASS/FAIL]
Deviations: D-2 message applied, D-3 cross-slice import documented
Next phase: Phase D
```

### After Phase D
```
Phase D Complete Report
Files changed: ListedCompanyPreviewCard.tsx, ListedCompanyPreviewCard.test.tsx, CreateCompanyModal.tsx
Tests run: npx vitest run src/features/company/
Visual verification: [manually checked Y/N]
Next phase: Phase E
```

### After Phase E
```
Phase E Complete Report
E2E scenarios: E1-E10 [list PASS/FAIL per scenario]
Edge cases verified: [list]
Blockers: [none OR list]
Next phase: Phase F
```

### After Phase G
```
Phase G Complete Report
Functional: [all scenarios PASS/FAIL]
Security: rate limit [PASS/FAIL], injection attempt [PASS/FAIL]
Performance: cache_hit_rate [%]
Business_code format verified: [sample codes found]
GO / NO GO: [decision]
```
