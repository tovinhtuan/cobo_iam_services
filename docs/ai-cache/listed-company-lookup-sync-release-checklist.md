# Release Checklist: Listed Company Lookup & Sync
**Feature:** Listed Company Lookup & Sync  
**Date:** 2026-06-05  
**Execution plan:** `docs/ai-cache/listed-company-lookup-sync-execution-plan.md`

---

## Pre-Implementation Gates

> Complete before writing any code.

- [x] **D-1 confirmed:** No `golang.org/x/sync` in go.mod — stdlib cache implementation to be used
- [x] **D-2 confirmed:** `COMPANY_ALREADY_EXISTS` message update covers both cases (membership exists + tax_code conflict)
- [x] **D-3 confirmed:** Cross-slice FE import from `cms-core/services/cmsApi.ts` is accepted
- [x] **[P1-3 RESOLVED] httpx.WriteJSON envelope format:** AdminHandler uses `httpx.WriteJSON` directly → **flat JSON** (no `data:` wrapper). FE parser reads from `root` directly, NOT `root.data`. CMS handlers use `writeEnvelope` which is different.
- [ ] **Vnstock DB accessible** in dev environment (`VNSTOCK_MARKET_ENABLED=true`, `VNSTOCK_MYSQL_DSN` set and reachable)
- [ ] **Nginx location** in project confirmed — where to commit `listed-lookup-rate-limit.conf`

### Hardening Changes Applied (from P1/P2 review)

- [x] **P1-1:** Cache-Control header is set per-response-type: `public max-age=3600` on 200, `no-store` on 400/503
- [x] **P1-2:** `cache_hit` field added to audit log spec in A5
- [x] **P1-3:** Envelope format resolved — flat JSON, FE parser updated accordingly
- [x] **P2-1:** Double-checked locking + `evictOldestLocked` added to cache pseudocode
- [x] **P2-2:** Cache eviction strategy specified: expired first, then LRU fallback
- [x] **P2-3:** Production monitoring updated — container-aware log commands
- [x] **P2-4:** Rollback procedure now has specific commands per component

---

## Build Gates

### Backend

- [ ] `go build ./...` — zero errors
- [ ] `go vet ./...` — zero warnings
- [ ] `go build ./cmd/api` — binary builds successfully

### Frontend

- [ ] `npm run lint` (tsc --noEmit) — zero type errors
- [ ] `npm run build` — production build succeeds

**If any build gate fails → NO GO**

---

## Unit Test Gates

### Backend — Must Pass

- [ ] `go test ./internal/marketreference/app/...` — incl. cache hit/miss, TTL, negative cache
- [ ] `go test ./internal/marketreference/infra/mysql/...` — incl. `GetByBusinessCode` exact, trim, not found
- [ ] `go test ./internal/companyaccess/transport/http/...` — incl. all 10 handler test cases
- [ ] `go test -race ./internal/marketreference/app/...` — no race conditions in cache

### Frontend — Must Pass

- [ ] `npx vitest run src/features/company/useListedCompanyLookup.test.ts`
- [ ] `npx vitest run src/features/company/ListedCompanyPreviewCard.test.tsx`
- [ ] `npx vitest run src/features/company/provisionErrors.test.ts` — updated message verified
- [ ] `npx vitest run src/features/company/CompanyCreateForm.test.tsx` — **NO REGRESSION**
- [ ] `npx vitest run src/features/company/CreateCompanyModal.test.tsx` — **NO REGRESSION**
- [ ] `npm run test` — full suite passes

**If any unit test fails → NO GO**

---

## Handler Test Cases — Must Pass

| # | Test Case | Expected |
|---|---|---|
| HT-01 | `business_code` found, full profile | 200 `found:true`, sync populated |
| HT-02 | `business_code` found, leading/trailing spaces | 200 `found:true` (trimmed) |
| HT-03 | `business_code` not found | 200 `found:false` |
| HT-04 | `business_code` empty string | 400 INVALID_REQUEST |
| HT-05 | `business_code` whitespace only | 400 INVALID_REQUEST |
| HT-06 | `business_code` > 50 chars | 400 INVALID_REQUEST |
| HT-07 | `business_code` with SQL special char (`'`) | 400 INVALID_REQUEST |
| HT-08 | vnstock service unavailable | 503 SERVICE_UNAVAILABLE |
| HT-09 | vnstock disabled (nil service) | 503 SERVICE_UNAVAILABLE |
| HT-10 | equity_list row exists, no company_profiles | 200 `found:false` |
| HT-11 | null phone in profile | sync object omits `phone` key |
| HT-12 | audit log emitted on every call | `listed_lookup_requested` event in log |
| HT-13 | audit log includes `cache_hit` field | `cache_hit=true` on second call for same code |
| HT-14 | `Cache-Control: public` on 200 found:true | Header present in 200 response |
| HT-15 | `Cache-Control: public` on 200 found:false | Header present in not_found 200 response |
| HT-16 | `Cache-Control: no-store` on 400 | Error responses NOT cached by browser/CDN |
| HT-17 | `Cache-Control: no-store` on 503 | Error responses NOT cached by browser/CDN |

---

## FE Hook Test Cases — Must Pass

| # | Test Case | Expected |
|---|---|---|
| FH-01 | Input < 8 chars | No API call |
| FH-02 | Input changes: preview resets immediately | State → idle before debounce |
| FH-03 | Debounce: no call before 500ms | 0 fetch calls |
| FH-04 | Debounce: call fires after 500ms | 1 fetch call |
| FH-05 | API returns found:true | Status → found |
| FH-06 | API returns found:false | Status → not_found |
| FH-07 | API returns 503 | Status → error |
| FH-08 | Smart merge: empty field gets filled | Patch includes field |
| FH-09 | Smart merge: filled field preserved | Patch excludes field |
| FH-10 | Smart merge: null sync field skipped | Patch excludes null |
| FH-11 | Input cleared | State → idle, no API call |

---

## FE Component Test Cases — Must Pass

| # | Test Case | Expected |
|---|---|---|
| FC-01 | Disclaimer rendered before buttons | Disclaimer text present |
| FC-02 | Button label "Đồng bộ thông tin" | Exact text match |
| FC-03 | Sub-text "(Chỉ điền vào các ô còn trống)" | Sub-text present |
| FC-04 | Click sync → onSync called | onSync invoked |
| FC-05 | Click sync → form NOT submitted | onSubmit NOT called |
| FC-06 | Click bỏ qua → onDismiss called | onDismiss invoked |
| FC-07 | Preview fields rendered | symbol, exchange present |

---

## Integration Test Gates

### E2E Scenarios — All must pass before production

| # | Scenario | Status |
|---|---|---|
| E1 | Initialize flow — lookup found → sync → create | ☐ |
| E2 | Initialize flow — lookup found → dismiss → manual create | ☐ |
| E3 | Initialize flow — lookup not found | ☐ |
| E4 | Create Nth company — lookup found → sync → create | ☐ |
| E5 | 503 graceful — form still submits | ☐ |
| E6 | Smart merge — filled fields preserved | ☐ |
| E7 | tax_code conflict after sync | ☐ |
| E8 | Stale preview dismissed on input change | ☐ |
| E9 | Rate limit (Nginx) fires after burst | ☐ |
| E10 | Cache hit — second request has cache_hit=true in log | ☐ |
| E11 | NoCompanyPage (initialize mode) — lookup + sync | ☐ |

**If any E2E scenario fails → NO GO until fixed**

---

## Nginx Gate

- [ ] `nginx -t` passes with new config included
- [ ] `nginx -s reload` succeeds
- [ ] Rate limit zone `listed_lookup` active: `curl -s -o /dev/null -w "%{http_code}" ... ` returns 429 after burst
- [ ] `Retry-After: 60` header present on 429 responses

**If Nginx gate fails → NO GO**

---

## Dev Smoke Tests — Must Pass Before Staging

```bash
# 1. Endpoint up
curl -s "http://dev.local:8080/api/v1/company/listed-lookup?business_code=TEST"
# Expected: 200 (found:true or found:false)

# 2. Empty → 400
curl -s -o /dev/null -w "%{http_code}" "http://dev.local:8080/api/v1/company/listed-lookup?business_code="
# Expected: 400

# 3. Audit log present
grep "listed_lookup_requested" /var/log/app.log | tail -1 | jq .
# Expected: JSON with event, request_id, ip, result, duration_ms

# 4. Rate limit firing
for i in {1..10}; do curl -s -o /dev/null -w "%{http_code}\n" "http://dev.local/api/v1/company/listed-lookup?business_code=TEST"; done
# Expected: 200 200 200 429 429 429 ... (burst=3 then limited)

# 5. Cache working
curl -s "http://dev.local:8080/api/v1/company/listed-lookup?business_code=SOMEVALID"
curl -s "http://dev.local:8080/api/v1/company/listed-lookup?business_code=SOMEVALID"
grep "listed_lookup_requested" /var/log/app.log | tail -2 | jq '.cache_hit'
# Expected: false then true
```

- [ ] Test 1 passes
- [ ] Test 2 passes
- [ ] Test 3 passes (audit log present and formatted correctly)
- [ ] Test 4 passes (rate limit fires)
- [ ] Test 5 passes (cache_hit=true on 2nd request)

**If any dev smoke fails → NO GO to staging**

---

## Staging Validation Gates

### Functional

- [ ] E1–E11 all pass on staging with real KBS data
- [ ] `business_code` in KBS data confirmed as ĐKKD format (10-digit, e.g. `0101234567`) — log at least 3 successful lookups with found:true

### Security

- [ ] Rate limit fires correctly (20 requests → 429s after burst)
- [ ] SQL injection attempt → 400 (not 500 or 200)
- [ ] Audit log has no full business_code (only prefix ≤ 4 chars)

### Performance

- [ ] Cache hit rate ≥ 80% after 20 identical requests (check logs)
- [ ] P95 latency (cache hit) < 50ms
- [ ] P95 latency (cache miss) < 200ms

### Monitoring

- [ ] `listed_lookup_requested` events flowing to log pipeline
- [ ] No unhandled errors or panics in log stream
- [ ] Nginx access log shows mix of 200 and 429 (no unexpected 500s)

**If any staging gate fails → NO GO to production**

---

## Production Release Gates

### Final Check — Release Manager

- [ ] All build gates ✅
- [ ] All unit test gates ✅
- [ ] All E2E scenarios ✅
- [ ] Dev smoke tests ✅
- [ ] Staging validation ✅
- [ ] Nginx config deployed to production Nginx (before backend deploy)
- [ ] Rollback plan confirmed (see below)

### Deploy Order (strict)

1. **Nginx config** — deploy rate limit conf, reload Nginx
2. **Backend binary** — deploy new API server build
3. **Frontend build** — deploy new FE assets

### GO / NO GO Decision

| Gate | Status | Blocker? |
|---|---|---|
| Build | ☐ | YES |
| Unit tests | ☐ | YES |
| E2E | ☐ | YES |
| Dev smoke | ☐ | YES |
| Staging functional | ☐ | YES |
| Staging security | ☐ | YES |
| Staging performance | ☐ | YES |
| Nginx deployed | ☐ | YES |

**All must be ✅ for GO. Any ☒ = NO GO.**

---

## Production Monitoring — First 30 Minutes

Run after deploy:

```bash
# Stream audit log
tail -f /var/log/app.log | grep "listed_lookup_requested"

# Check result distribution
grep "listed_lookup_requested" /var/log/app.log | jq -r '.result' | sort | uniq -c

# Check error rate
grep "listed_lookup_requested" /var/log/app.log | jq -r 'select(.result == "unavailable")' | wc -l
# Should be < 1% of total requests

# Check latency
grep "listed_lookup_requested" /var/log/app.log | jq '.duration_ms' | sort -n | tail -5
# P95 should be < 200ms
```

Rollback triggers:
- `result=unavailable` > 10% of requests in 5 min window → investigate vnstock DB
- `duration_ms` P95 > 500ms sustained 5 min → investigate cache or DB
- Any panic log from `listed_lookup` or `admin_handler_listed_lookup` → immediate rollback
- 500 responses from endpoint (not 429, not 400) → investigate

---

## Rollback Procedure

### Backend rollback

```bash
# Deploy previous binary
# The previous binary does not have the listed-lookup route
# The Nginx rate limit config is harmless (404 fallback) — do not remove unless causing issues
```

### Frontend rollback

```bash
# Redeploy previous frontend build
# No state stored client-side related to this feature
```

### Database rollback

**Not applicable.** No schema changes in this feature.

---

## Known Limitations (Accepted)

| Limitation | Decision |
|---|---|
| `COMPANY_ALREADY_EXISTS` error covers two different backend scenarios | Accepted — message updated to cover both (D-2) |
| `business_code` JSON field has no DB index — full scan on cache miss | Accepted at 1700 rows. Tech debt: generated column + index when > 10K rows |
| Cross-slice FE import from `cms-core` | Accepted — documented, not blocking |
| No singleflight — concurrent cache misses for same key hit DB | Accepted — stdlib mutex gates writes, at most 1 concurrent query per key in practice |
| Vnstock data may be up to 1 hour stale (cache TTL) | Accepted — matches disclaimer to user |

---

## Document Links

| Document | Path |
|---|---|
| Feature spec | `docs/feature-listed-company-lookup-sync.md` |
| Implementation plan | `docs/ai-cache/listed-company-lookup-sync-implementation-plan.md` |
| Architecture review | `docs/ai-cache/listed-company-lookup-sync-principal-architecture-review.md` |
| Execution plan | `docs/ai-cache/listed-company-lookup-sync-execution-plan.md` |
| This checklist | `docs/ai-cache/listed-company-lookup-sync-release-checklist.md` |
