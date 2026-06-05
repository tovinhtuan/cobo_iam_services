# Batch 1 Report: Listed Company Lookup & Sync

**Date:** 2026-06-05  
**Scope:** A1 (interface) + A2 (repository) + A3 (cache + service)  
**Status:** ✅ COMPLETE

---

## Scope Completed

| Task | Description | Status |
|---|---|---|
| A1 | Extend `ListedCompanyReader` interface with `GetByBusinessCode` | ✅ Done |
| A1 | Update `fakeListedRepo` in `service_test.go` (same package) | ✅ Done |
| A1 | Update `listedFakeRepo` in CMS handler test (implements interface) | ✅ Done |
| A2 | `Repository.GetByBusinessCode` with JOIN query + input trim | ✅ Done |
| A2 | Exported test helpers: `NormalizeBusinessCode`, `getByBusinessCodeQuery` | ✅ Done |
| A3 | `businessCodeCache` struct (RWMutex, double-checked locking, LRU eviction) | ✅ Done |
| A3 | `Service.GetByBusinessCode` returns `(detail, cacheHit bool, error)` | ✅ Done |
| A3 | Cache init in `NewService` and `NewDisabledService` | ✅ Done |

---

## Files Changed

| File | Change |
|---|---|
| `internal/marketreference/app/types.go` | Added `GetByBusinessCode` to `ListedCompanyReader` interface |
| `internal/marketreference/app/service.go` | Added `businessCodeCache` struct + constants + `GetByBusinessCode` method + cache field |
| `internal/marketreference/app/service_test.go` | Added `getByBusinessCodeFn`+`callCount` to `fakeListedRepo`; added 9 new test cases |
| `internal/marketreference/infra/mysql/repository.go` | Added `NormalizeBusinessCode`, `getByBusinessCodeQuery` constant, `GetByBusinessCode` method |
| `internal/marketreference/infra/mysql/repository_test.go` | Added `TestNormalizeBusinessCode` and `TestBuildGetByBusinessCodeQuery` |
| `internal/platformcms/transport/http/listed_companies_handlers_test.go` | Added `getByBusinessCodeFn` + `GetByBusinessCode` method to `listedFakeRepo` (interface compliance) |

## Files Created

None. All changes are additive modifications to existing files.

---

## Tests Added

### Service package (`internal/marketreference/app`)

| Test | Coverage |
|---|---|
| `TestService_GetByBusinessCode_Disabled` | `NewDisabledService()` returns ErrUnavailable |
| `TestService_GetByBusinessCode_EmptyCode` | Empty businessCode → ErrInvalidRequest |
| `TestService_GetByBusinessCode_WhitespaceCode` | Whitespace-only → ErrInvalidRequest |
| `TestService_GetByBusinessCode_Found` | Happy path, first call is cache miss |
| `TestService_GetByBusinessCode_NotFound` | Repo miss → ErrNotFound |
| `TestService_GetByBusinessCode_CacheHit` | Second call returns from cache, repo called only once |
| `TestService_GetByBusinessCode_NegativeCacheHit` | ErrNotFound result is cached, repo called only once |
| `TestService_GetByBusinessCode_TTLExpiry` | Expired entry is treated as cache miss |
| `TestService_GetByBusinessCode_ConcurrentMissSameKey` | 20 goroutines, cache is warm after all complete |
| `TestService_GetByBusinessCode_CacheEvictsOldest` | Cache at maxSize evicts oldest entry on new insert |

### Repository package (`internal/marketreference/infra/mysql`)

| Test | Coverage |
|---|---|
| `TestNormalizeBusinessCode` | Trim whitespace variants (spaces, tabs, newlines, empty) |
| `TestBuildGetByBusinessCodeQuery` | SQL query contains required clauses (JOIN not LEFT JOIN, JSON_EXTRACT, LIMIT 1) |

---

## Tests Run

```
go test ./internal/marketreference/... -count=1
```
```
go test -race ./internal/marketreference/... -count=3
```
```
go build ./...
```
```
go test ./internal/marketreference/... ./internal/platformcms/transport/http/... -count=1
```

---

## Test Results

| Package | Tests | Result |
|---|---|---|
| `marketreference/app` | 14 | ✅ PASS |
| `marketreference/infra/mysql` | 11 | ✅ PASS |
| `platformcms/transport/http` | 17 (regression check) | ✅ PASS |
| Race detector (count=3) | marketreference/* | ✅ PASS — no races |
| Full build `go build ./...` | — | ✅ PASS |

**Pre-existing failures (not caused by Batch 1):**
- `TestCreateSelfServiceCompany_FeatureFlagOff` (companyaccess transport) — confirmed pre-existing by git stash test
- `TestIntegration_*` (httpserver) — confirmed pre-existing
- `TestDispatchEmail_SanitisesAllSensitiveVars` (notification/app) — confirmed pre-existing

---

## Verification Matrix

| Scenario | Result |
|---|---|
| Found (repo returns detail) | ✅ PASS |
| Not Found (repo returns ErrNotFound) | ✅ PASS |
| Trim Input (whitespace business code) | ✅ PASS |
| Empty Input | ✅ PASS |
| Cache Hit (positive) | ✅ PASS |
| Cache Miss (first call) | ✅ PASS |
| Negative Cache Hit | ✅ PASS |
| TTL Expire (expired entry = miss) | ✅ PASS |
| Cache Eviction at maxSize | ✅ PASS |
| Race Test (-race, count=3) | ✅ PASS |
| Disabled service → ErrUnavailable | ✅ PASS |
| Query uses JOIN not LEFT JOIN | ✅ PASS |
| Query uses JSON_EXTRACT + LIMIT 1 | ✅ PASS |

---

## Implementation Notes

### Cache design
- `businessCodeCache` uses `sync.RWMutex` — stdlib only, no new dependency
- Double-checked locking: RLock for read path (fast), Lock for write with re-check after DB call
- Negative cache TTL: 10 minutes (prevents DB spam for invalid codes)
- Positive cache TTL: 60 minutes (company profiles change ≤monthly)
- Eviction: expired entries first, then oldest (earliest expiry) as LRU fallback
- Max size: 2000 entries (~1700 listed companies + headroom)

### Repository design
- `getByBusinessCodeQuery` exported as package-level constant for testability
- `NormalizeBusinessCode` exported for tests (consistent with `NormalizeSymbol` pattern)
- Query uses `JOIN` not `LEFT JOIN` — business_code lives in company_profiles, so no profile = no match
- Input trimmed at both repository (defensive) and service (primary) layers

### Service signature
`GetByBusinessCode(ctx, businessCode) (ListedCompanyDetail, bool, error)`
- `bool` = `cacheHit` — required by handler audit logging (Phase B gate and Phase H monitoring)

---

## Risks

| Risk | Severity | Notes |
|---|---|---|
| Concurrent cache miss (N goroutines same key) | Low | Double-checked locking reduces but does not eliminate duplicate DB calls. In test, 20 goroutines complete with final cache hit verified. Acceptable at 1700 rows. |
| Cache eviction when all 2000 entries are valid TTL | Low | `evictOldestLocked` handles this case. Tested via `TestService_GetByBusinessCode_CacheEvictsOldest`. |

---

## Technical Debt

| Item | Priority | Notes |
|---|---|---|
| `business_code` lacks DB index (full JSON scan per miss) | Low | ~1700 rows, <5ms per scan. Mitigated by cache. Track for generated column migration when data grows. |
| vnstock DB schema ownership unclear | Low | Generated column index roadmap depends on whether project controls the vnstock schema. |

---

## Blockers

None.

---

## Ready For Batch 2?

**YES**

All Batch 1 acceptance criteria met:
- ✅ `GetByBusinessCode` on `ListedCompanyReader` interface
- ✅ Repository implementation with JOIN query, trim, error sentinel returns
- ✅ In-process cache with TTL, negative cache, LRU eviction, double-checked locking
- ✅ Service returns `(detail, cacheHit bool, error)` for handler audit log
- ✅ All new tests pass including race detector
- ✅ No regressions in affected packages
- ✅ Full build passes

Batch 2 scope (per execution plan): Phase A4 + A5 + A6 + A7
- `AdminHandler` field + `WithListedCompaniesLookup` method
- `admin_handler_listed_lookup.go` (handler + route + audit log + input sanitization + Cache-Control headers)
- Server wiring in `httpserver/server.go`
- Nginx rate limit config file
