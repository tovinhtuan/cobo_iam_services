# Principal Architecture Review: Listed Company Lookup & Sync
**Reviewed by:** Principal Solution Architect + Principal Backend Engineer + Principal Security Engineer + Principal Product Engineer  
**Date:** 2026-06-05  
**Input plan:** `docs/ai-cache/listed-company-lookup-sync-implementation-plan.md`  
**Feature spec:** `docs/feature-listed-company-lookup-sync.md`  
**Verdict:** ⚠️ READY WITH ARCHITECTURE IMPROVEMENTS

---

## Executive Summary

Plan hiện tại đạt mức "đủ chạy trong dev" nhưng chưa đạt "production-grade". Có **5 gaps nghiêm trọng** cần address trước khi ship:

1. **Security:** Rate limiting đang "optional" — đây là sai. Public endpoint + DB scan = DOS vector. Phải mandatory trước production.
2. **Data Access:** JSON_EXTRACT full scan không có cache = mỗi user keystroke là một DB hit. Với debounce 500ms, một user gõ chậm có thể fire 5–10 scans. Không chấp nhận được ở production.
3. **Cache:** Hoàn toàn vắng mặt trong plan — đây là thiếu sót nghiêm trọng nhất về performance.
4. **UX:** "Đồng bộ và ghi đè thông tin" overwrite toàn bộ fields không phải là UX tốt nhất. Smart merge (chỉ điền field trống) là đúng hơn và không cần thêm complexity.
5. **Observability:** "Monitor: log found vs not_found" không phải là production observability. Không có metrics, không có alerting, không có structured events.

**Không có thay đổi nào trong 5 điểm trên làm tăng scope đáng kể.** Tất cả có thể implement trong cùng sprint với cost thấp.

---

## Phase 1 — Architecture Challenge Review

| Topic | Current Plan | Risk | Better Alternative | Recommendation |
|---|---|---|---|---|
| Public endpoint security | Nginx rate limit "optional cho dev, recommended cho staging/prod" | **HIGH** — Public endpoint gọi DB scan trên mỗi request, không có mandatory enforcement | Nginx rate limit **mandatory** trước production, bắt buộc config trước Phase D (Dev Deploy) | CHANGE — Đưa Nginx config vào Phase A deliverable |
| Data access — JSON_EXTRACT | `WHERE JSON_EXTRACT(info,'$.business_code') = ?` — full scan mỗi request, gọi là "acceptable" | **MEDIUM** — Đúng với 1700 rows hiện tại, sai khi concurrent load tăng. 100 req/s × 6ms/query = DB busy 60% thời gian | In-process TTL cache trong Go service layer — loại bỏ 95%+ DB calls | CHANGE — Thêm in-process cache vào Phase A |
| Cache strategy | None | **HIGH** — Debounce 500ms không đủ để bảo vệ DB. User gõ chậm, di chuyển qua lại field sẽ fire nhiều requests | In-process sync.Map cache với TTL 1 giờ, bounded size 2000 entries | ADD — Cache là required, không phải optional |
| Sync UX — overwrite behavior | Full overwrite tất cả non-null fields với single button | **MEDIUM** — User đã nhập company_name, address → trigger lookup → sync → mất hết, chỉ có button label làm "protection" | Smart merge: chỉ điền fields hiện tại đang trống. Fields đã có value không bị touch | CHANGE — Không cần confirmation dialog, smart merge đơn giản hơn và safer |
| Observability | 1 log line ("listed-lookup: found/not_found") | **HIGH** — Không có cách detect abuse, measure latency, hoặc validate adoption | Structured slog events với request_id, ip, duration_ms, result | CHANGE — Structured logging là production requirement |
| Cross-slice FE dependency | `parseListedCompanyDetailPayload` reused từ `cms-core/services/cmsApi.ts` | **LOW** — Cross-feature-slice dependency. `company/` feature import từ `cms-core/` là architecture smell | Extract shared parsing logic thành standalone function trong `features/company/` hoặc `services/` | CONSIDER — Not blocking nhưng nên address |
| Generated column/index | Ghi nhận tech debt, plan "khi data grow" | Underspecified | Clarify: vnstock DB có phải là externally managed không? Nếu có thì generated column không apply | CLARIFY — Xác định ownership của vnstock DB schema |

---

## Phase 2 — Security Review

### Threat Model

| Threat | Vector | Current Mitigation | Gap |
|---|---|---|---|
| Enumeration | Iterate valid business codes | Public data — "acceptable" | Không có audit log, không detect pattern |
| Scraping | Automated harvest of email/phone | Nginx rate limit "optional" | Chưa được enforce |
| DOS via DB amplification | Flood endpoint → flood JSON_EXTRACT scans → exhaust vnstock DB connection pool | Nginx "optional" + table nhỏ | Connection pool exhaustion khi Nginx bypass hoặc bị ddos |
| Data exfiltration | Harvest tất cả company info | Nginx rate limit | Không audit ai đã query gì |
| Internal bypass | Direct service call (trong internal network) | Không có | Nginx không bảo vệ được internal calls |

### Option Analysis

| Option | Security | UX | Complexity | Recommendation |
|---|---|---|---|---|
| A — Public, no auth | Low (current) | Best | Lowest | ❌ Không chấp nhận ở production nếu không có mandatory rate limit + audit |
| B — Public with mandatory rate limit + audit log | Medium-High | Best | Low | ✅ **Recommended** |
| C — Auth required (access token) | High | Medium (user phải login trước) | Low | ❌ Over-restrict: spec nói public cho mọi user |
| D — Signed request / captcha | High | Poor | High | ❌ Quá phức tạp cho feature này |

**Chọn: Option B — Public endpoint với mandatory rate limit + structured audit log**

### Required Security Controls (Mandatory)

#### 1. Nginx Rate Limiting — MANDATORY trước production

```nginx
# Đây không phải "gợi ý" — đây là required config trước Phase D
limit_req_zone $binary_remote_addr zone=listed_lookup:10m rate=10r/m;

location = /api/v1/company/listed-lookup {
    limit_req zone=listed_lookup burst=3 nodelay;
    limit_req_status 429;
    add_header Retry-After 60 always;
    proxy_pass http://backend;
}
```

**Rationale:** 10 req/phút/IP là đủ cho legitimate use (user gõ ĐKKD trong form). Burst=3 handles normal debounce behavior. Scraper sẽ bị block ngay.

#### 2. Structured Audit Logging — MANDATORY trong handler

Mỗi lookup request phải log:
```go
slog.InfoContext(ctx, "listed_lookup_requested",
    slog.String("business_code_prefix", redactBusinessCode(businessCode)), // log 4 ký tự đầu, an toàn hơn log full
    slog.String("ip", r.RemoteAddr),
    slog.String("user_agent", r.UserAgent()),
    slog.String("result", "found|not_found|error|unavailable"),
    slog.Int64("duration_ms", durationMs),
    slog.String("request_id", httpx.RequestIDFromContext(ctx)),
)
```

**Note:** Không log full `business_code` trong audit log — chỉ log 4 ký tự đầu (đủ để debug, không expose data).

#### 3. Response Headers

```
X-Content-Type-Options: nosniff
Cache-Control: no-store   ← ngăn browser cache response (email/phone không nên cached ở browser)
```

#### 4. Input Sanitization

- Trim whitespace
- Max length 50 ký tự
- Reject non-alphanumeric characters ngoài `-` và `/` (ĐKKD format)
- Không accept `null`, `undefined`, SQL special chars

---

## Phase 3 — Data Access Strategy

### Scale Analysis

| Scale | Table Size | Concurrent Requests | Full Scan Time | Pool Impact | Verdict |
|---|---|---|---|---|---|
| Current (~1700 rows) | ~500KB | <10 req/s | ~3–5ms/req | Negligible | Acceptable |
| Medium (10K rows) | ~3MB | 50 req/s | ~15–20ms/req | ~1 connection busy constantly | Borderline |
| Large (100K rows) | ~30MB | 100 req/s | ~50–100ms/req | Pool saturated | Unacceptable |

**Hiện tại 1700 rows là acceptable nhưng cache vẫn là correct architectural decision** — bởi vì data hiếm khi thay đổi (company profiles update monthly/quarterly) và lookup pattern là highly repetitive (cùng ĐKKD được lookup bởi nhiều user khác nhau trong onboarding).

### Option Analysis

| Option | Effort | Performance | Migration Cost | Long-term | Notes |
|---|---|---|---|---|---|
| A — JSON_EXTRACT scan (current) | 0 | Low at scale | — | Bad | Không có cache = DB hit mỗi request |
| B — Generated Column + Index trên vnstock DB | Medium | Excellent | Medium | Good | **CHỈ áp dụng nếu chúng ta sở hữu vnstock DB schema** |
| C — Dedicated column (migration vnstock schema) | High | Excellent | High | Good | Cần migration, rollback phức tạp |
| D — In-process TTL Cache trong Go service (recommended for now) | Low | Excellent tại scale hiện tại | 0 (no migration) | Good for <50K entries | Correct fit |

**Chọn: Option D (In-process TTL Cache) cho sprint hiện tại + Option B (Generated Column) làm medium-term roadmap nếu sở hữu schema**

### Vnstock DB Ownership Clarification Required

> **Action item:** Cần xác định rõ:
> - Vnstock DB có phải là externally managed (KBS pipeline writes, chúng ta chỉ read) không?
> - Nếu yes → Generated Column không apply. Cache là solution duy nhất.
> - Nếu no → Plan generated column migration.

### In-Process Cache Design (Recommended for This Sprint)

**Location:** `internal/marketreference/app/service.go` — thêm cache layer vào `Service` struct.

**Design:**
```
Service struct:
  repo    ListedCompanyReader
  ping    func(ctx) error
  cache   *businessCodeCache    ← NEW

businessCodeCache:
  mu      sync.RWMutex
  entries map[string]cacheEntry
  maxSize int                   ← 2000 (covers all listed companies 1.2x)
  
cacheEntry:
  result  *ListedCompanyDetail  ← nil = negative cache (not found)
  expiry  time.Time
```

**TTL Strategy:**
- Positive cache (found): 1 giờ — company profiles ít thay đổi
- Negative cache (not found): 10 phút — ngăn repeated DB scan cho invalid codes
- No explicit invalidation needed (TTL is sufficient given update frequency)

**Thundering Herd Protection:** Wrap DB call với `golang.org/x/sync/singleflight` — nếu 100 requests cùng lookup một ĐKKD trong lúc cache miss, chỉ 1 DB query được fire.

**Memory estimate:** 2000 entries × ~2KB/entry (full profile JSON) = ~4MB. Negligible.

**Cache invalidation:** Không cần. TTL 1 giờ đủ. Company profile data được KBS pipeline update định kỳ — không cần real-time invalidation.

---

## Phase 4 — Cache Strategy

### Option Analysis

| Option | Complexity | Cost | Performance | Fits Use Case | Recommendation |
|---|---|---|---|---|---|
| A — No cache (current) | 0 | $0 | Poor at scale | No | ❌ |
| B — In-process Go cache (sync.Map + TTL) | Low | $0 | Excellent (no network hop) | Yes | ✅ **Recommended** |
| C — Redis cache | High | $$ | Good | Overkill | ❌ |
| D — HTTP Cache-Control headers | Low | $0 | Limited (browser only, không help DB) | Partial | Consider as supplement |

**Chọn: Option B — In-process Go cache**

**Rationale:**
- 1700 entries fit easily in memory (~4MB)
- Data changes at most weekly (KBS pipeline refresh)
- No network hop (Redis) needed — latency negligible
- No infrastructure dependency added
- Singleflight prevents thundering herd

**Cache Config:**
| Parameter | Value | Rationale |
|---|---|---|
| Positive TTL | 60 minutes | Company profiles update <= weekly |
| Negative TTL | 10 minutes | Invalid codes should not spam DB |
| Max entries | 2000 | 1700 companies + 20% headroom |
| Invalidation | TTL-only | Update frequency doesn't require active invalidation |
| Singleflight | Yes | Prevents duplicate DB calls on cold start |

**Fallback:** Cache miss → query DB → cache result. No fallback needed since DB is the source of truth.

**HTTP Cache-Control supplement (optional):**
```
Cache-Control: public, max-age=3600, stale-while-revalidate=300
```
Cho phép CDN/browser cache response. Không sensitive (public data). Reduce repeat requests từ cùng user. But chỉ là optimization, không phải requirement.

---

## Phase 5 — UX Strategy Review

### Problem with Current Plan

User đã nhập:
- `company_name`: "Công ty TNHH của tôi"
- `address`: "123 Nguyễn Văn Linh"
- `phone`: "0901234567"

Sau đó nhập `registrationNumber` và lookup returns data. User bấm "Đồng bộ và ghi đè thông tin" → tất cả 3 fields bên trên bị overwrite bởi vnstock data. User mất hết những gì đã nhập.

Button label "ghi đè" giúp user biết behavior nhưng không giảm frustration sau khi data đã bị mất.

### Option Analysis

| Option | UX | Complexity | Risk | Recommendation |
|---|---|---|---|---|
| A — Full overwrite (current plan) | Poor for users who pre-filled form | Low | High — data loss frustration | ❌ |
| B — Field selection checkboxes | Good | Medium | Low | Consider if time allows |
| C — Side-by-side compare modal | Best | High | Low | ❌ Quá phức tạp |
| D — Smart merge: only fill empty fields | Excellent | Low | Minimal | ✅ **Recommended** |

**Chọn: Option D — Smart Merge**

**Definition:** Sync chỉ patch những fields mà giá trị hiện tại trong form đang là empty string hoặc null. Fields đã có content được giữ nguyên.

```
mapSyncPayload(result, currentValues):
  patch = {}
  if currentValues.companyName === '' && result.sync.company_name:
    patch.companyName = result.sync.company_name
  if currentValues.taxCode === '' && result.sync.tax_code:
    patch.taxCode = result.sync.tax_code
  // ... etc
  return patch
```

**Impact trên UX:**
- User chưa điền gì → sync điền hết → perfect
- User đã điền company_name → sync không chạm company_name → safe
- User đã điền vài field, còn để trống email → sync điền email → helpful

**Impact trên button label:**
- Không cần "ghi đè" nữa → đổi thành **"Đồng bộ thông tin"**
- Thêm sub-text: "(Chỉ điền vào các ô trống)"

**Impact trên Preview Card:**
- Hiển thị indicator cho biết field nào sẽ được sync (empty) vs không được sync (đã có content)
- Optional: nhưng adds clarity

**Impact on Acceptance Criteria:**
- AC-02 cần update: "null fields không ghi đè" → mở rộng thành "fields đã có content không bị ghi đè"

---

## Phase 6 — Observability Strategy

### Current Plan Assessment

"Monitor: log `listed-lookup: found` và `listed-lookup: not_found` rates"

**Assessment: Không đủ cho production.** Không có:
- Request latency tracking
- Error rate monitoring
- Abuse detection signal
- Business adoption metrics
- SLA definition

### Required Observability

#### Structured Log Events

Mỗi event là một `slog.InfoContext` call với structured fields. No free-text logs.

**Event: `listed_lookup_requested`** (mỗi request)
```go
slog.InfoContext(ctx, "listed_lookup_requested",
    slog.String("event", "listed_lookup_requested"),
    slog.String("request_id", httpx.RequestIDFromContext(ctx)),
    slog.String("ip", r.RemoteAddr),
    slog.String("user_agent", r.UserAgent()[:min(len(ua), 100)]),
    slog.String("business_code_prefix", businessCode[:min(len(businessCode), 4)]),
    slog.String("result", result),         // "found" | "not_found" | "error" | "unavailable"
    slog.Bool("cache_hit", cacheHit),      // để track cache effectiveness
    slog.Int64("duration_ms", durationMs),
)
```

**Event: `listed_lookup_sync_clicked`** (frontend — thông qua BE tracking nếu cần, hoặc FE analytics)
```
business_code_prefix
result: "synced" | "dismissed"
fields_synced: ["company_name", "tax_code", ...] // fields thực sự được fill
```

Note: Sync event là FE-side event, không cần BE round-trip trừ khi muốn server-side analytics.

#### Metrics (Structured logs → log aggregation)

| Metric | How to derive | Alert threshold |
|---|---|---|
| `lookup_requests_total` | Count `listed_lookup_requested` events | — |
| `lookup_found_rate` | found / total | < 20% sustained → data quality issue |
| `lookup_error_rate` | error / total | > 5% in 5 min → alert |
| `lookup_latency_p95` | duration_ms p95 | > 200ms → investigate |
| `lookup_cache_hit_rate` | cache_hit=true / total | < 50% after warmup → investigate |
| `lookup_abuse_indicator` | requests_per_ip per minute | > 20/min from single IP → alert |

#### Alerting Rules

| Alert | Condition | Severity | Action |
|---|---|---|---|
| Lookup service unavailable | error_rate > 50% in 5 min | High | PD page |
| Lookup latency spike | p95 > 500ms | Medium | Slack |
| Abuse pattern detected | single IP > 50 req/min (Nginx log) | High | Auto-block IP + Slack |
| Found rate anomaly | found_rate < 10% sustained 1h | Low | Slack — possible data quality issue |

#### Dashboard (Log-based, Kibana-compatible)

Sử dụng structured slog output (JSON mode). Kibana queries:

```
event: listed_lookup_requested
| timeseries count by result
| timeseries avg duration_ms
| top 10 ip by count where result: error
```

Không cần Grafana/Prometheus setup mới — reuse existing log pipeline.

#### SLA Definition

| Metric | Target |
|---|---|
| Availability | 99.5% (lower than main API — feature is optional helper) |
| P95 latency (cache hit) | < 50ms |
| P95 latency (cache miss) | < 200ms |
| Rate limit response time | < 5ms (Nginx handles) |

---

## Phase 7 — Recommended Changes Summary

### Changes Required (Blocking for Production)

| # | Change | Priority | Effort | Phase |
|---|---|---|---|---|
| C-1 | In-process TTL cache trong `marketreference/app/service.go` | **Critical** | 0.5 ngày | Phase A |
| C-2 | Nginx rate limit config — mandatory, not optional | **Critical** | 0.25 ngày | Phase A (deliverable) |
| C-3 | Structured audit logging trong handler | **Critical** | 0.25 ngày | Phase A |
| C-4 | Smart merge UX (chỉ fill empty fields) | **High** | 0.5 ngày | Phase B |
| C-5 | Input sanitization beyond trim (reject special chars) | **High** | 0.25 ngày | Phase A |

### Changes Recommended (Not Blocking)

| # | Change | Priority | Effort | Phase |
|---|---|---|---|---|
| R-1 | HTTP Cache-Control header (`public, max-age=3600`) | Medium | 0.1 ngày | Phase A |
| R-2 | Extract parser từ cms-core sang company feature hoặc services/shared | Low | 0.5 ngày | Phase B |
| R-3 | FE: show which fields will be synced vs skipped trong preview card | Low | 0.5 ngày | Phase B |
| R-4 | `Retry-After` header khi 429 (Nginx) | Low | 0 (Nginx config) | Phase D |

### Clarification Required Before Phase A

| # | Question | Owner | Impact |
|---|---|---|---|
| CQ-1 | Vnstock DB ownership: external (KBS managed) hay internal? Có thể add generated column không? | Engineering Lead | Data access strategy |

---

## Phase 8 — Revised Execution Roadmap

### Phase A — Backend Foundation + Security (Day 1–2)

**Deliverables (all required, no optional):**
1. `GetByBusinessCode` trong repository (JSON_EXTRACT)
2. In-process TTL cache với singleflight trong service
3. Handler + route `GET /api/v1/company/listed-lookup`
4. Structured audit logging trong handler (every request)
5. Input sanitization (trim + length + character validation)
6. Nginx rate limit config file (không optional, phải deliverable)
7. Wire `WithListedCompaniesLookup` trong httpserver
8. Unit tests: handler (all cases) + cache behavior + singleflight

**Checkpoint:**
```bash
# Rate limit works
curl -s -o /dev/null -w "%{http_code}" "http://localhost/api/v1/company/listed-lookup?business_code=0101234567"
# Fire 15 requests quickly → some get 429
for i in {1..15}; do curl -s -o /dev/null -w "%{http_code}\n" "http://localhost/api/v1/company/listed-lookup?business_code=TEST"; done

# Cache works — second request has cache_hit=true in logs
curl "http://localhost:8080/api/v1/company/listed-lookup?business_code=0101234567"
curl "http://localhost:8080/api/v1/company/listed-lookup?business_code=0101234567"
# grep logs for cache_hit=true
```

### Phase B — Frontend UX (Day 2–3)

**Deliverables:**
1. `lookupListedCompany(businessCode)` trong `authApi.ts` (no-auth GET)
2. `useListedCompanyLookup.ts` — debounce 500ms, min 8 chars, smart merge payload builder
3. `ListedCompanyPreviewCard.tsx` — smart merge UX, button "Đồng bộ thông tin (chỉ điền ô trống)"
4. `provisionErrors.ts` — thêm COMPANY_ALREADY_EXISTS case
5. `CreateCompanyModal.tsx` — integrate hook + card
6. Unit tests (hook + card + provisionErrors)

**Key implementation note:** `mapSyncPayload(result, currentValues)` — chỉ patch key mà `currentValues[key] === ''`.

### Phase C — Observability + QA (Day 3–4)

**Deliverables:**
1. Verify structured logs đang emit đúng format (grep/jq trên logs)
2. E2E test all 8 scenarios
3. Load test: 100 req/s against endpoint, verify cache_hit rate > 90% after warmup
4. Verify Nginx rate limit bằng script

### Phase D — Dev Deploy (Day 4)

- `VNSTOCK_MARKET_ENABLED=true` + `VNSTOCK_MYSQL_DSN` set
- Nginx rate limit config deployed (not optional)
- Validate logs flowing to log pipeline

### Phase E — Staging QA (Day 5)

- Smoke test với data thực
- Verify `business_code` trong KBS data thực sự là ĐKKD format
- Test rate limit với real Nginx
- Check cache_hit_rate trong logs > 80% sau warmup

### Phase F — Production Rollout (Day 5–6)

- Standard deploy, no migration
- Monitor:
  - `lookup_error_rate` < 1% trong 30 phút đầu
  - `lookup_latency_p95` < 200ms
  - Cache hit rate > 80% sau 10 phút
  - Không có abuse pattern từ Nginx logs

---

## Final Verdict

**⚠️ READY WITH ARCHITECTURE IMPROVEMENTS**

Plan hiện tại đủ chức năng nhưng chưa production-ready. Cần 5 changes trước khi merge sang production branch:

1. **C-1** In-process TTL cache — eliminates DB amplification risk
2. **C-2** Nginx rate limit — mandatory (currently optional = wrong)
3. **C-3** Structured audit logging — required for a compliance SaaS product
4. **C-4** Smart merge UX — replaces full overwrite, simpler and safer
5. **C-5** Input sanitization — beyond trim

**Revised estimate:** +1 ngày dev (3–4 ngày → 4–5 ngày) do cache implementation và observability.

**Không cần rework từ đầu.** Architecture cơ bản vẫn đúng. Đây là improvements lên plan đã có, không phải thay thế.
