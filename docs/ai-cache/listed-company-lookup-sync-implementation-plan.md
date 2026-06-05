# Implementation Plan: Listed Company Lookup & Sync
**Feature spec:** `docs/feature-listed-company-lookup-sync.md`  
**Reviewed by:** Principal Solution Architect / Principal Product Engineer / Staff Backend Engineer  
**Date:** 2026-06-05  
**Updated:** 2026-06-05 (principal architecture review applied)  
**Status:** ⚠️ READY WITH ARCHITECTURE IMPROVEMENTS  
**Architecture review:** `docs/ai-cache/listed-company-lookup-sync-principal-architecture-review.md`

---

## Executive Summary

Feature yêu cầu người dùng nhập mã đăng ký kinh doanh → lookup công ty niêm yết → sync thông tin vào form tạo doanh nghiệp. Phân tích codebase cho thấy **~80% code cần thiết đã tồn tại** và có thể reuse trực tiếp. Effort thực tế chỉ tập trung vào: repository query method, public endpoint, in-process cache, observability, và UX widget.

Product decisions đã resolved. Principal architecture review đã xác định 5 improvements cần thiết trước production: in-process cache, mandatory rate limiting, structured audit logging, smart merge UX, input sanitization. **Revised estimate: 4–5 ngày dev.**

> **Architecture review:** Xem chi tiết tại `docs/ai-cache/listed-company-lookup-sync-principal-architecture-review.md`

---

## Product Decision Resolution

### Q-BLOCK-1 — ✅ RESOLVED

**Question:** `business_code` trong KBS data có phải là "số đăng ký kinh doanh" hay là mã ngành VSIC?

**Decision:** `business_code` trong `company_profiles.info` đã được xác nhận là **số đăng ký kinh doanh / mã số doanh nghiệp** (dạng `0101234567`) — KHÔNG phải mã ngành VSIC.

**Impact on plan:**
- Giữ nguyên lookup key: `company_profiles.info.business_code`
- API query param: `?business_code={số ĐKKD}`
- Field `registration_number` trong form tạo DN map trực tiếp từ `business_code` của vnstock
- Không cần tìm field thay thế, không thay đổi feature scope

> **Frontend i18n debt (non-blocking):** Label `'listedCompanies.field.businessCode'` hiện là `'Mã ngành kinh doanh'` trong `language.tsx` — cần sửa thành `'Mã ĐKKD / mã số doanh nghiệp'` trong phase frontend. Không block backend.

---

### Q-BLOCK-2 — ✅ RESOLVED (SA Recommendation accepted)

**Question:** UX xử lý thế nào khi sync `tax_id` → `tax_code` gây UNIQUE constraint conflict khi submit?

**Decision:**
- Không bypass unique constraint
- Không tự clear `tax_code` trước khi submit
- Không cho phép tạo doanh nghiệp trùng `tax_code` (backend không thay đổi)
- Không thay đổi `verification_status`
- Giữ nguyên toàn bộ form data user đã sync/nhập — user tự chỉnh sửa
- Hiển thị error message rõ ràng, actionable

**UX error message khi BE trả `COMPANY_ALREADY_EXISTS` sau khi user đã sync:**
```
Mã số thuế này đã được đăng ký bởi một doanh nghiệp khác trên hệ thống.
Vui lòng kiểm tra lại thông tin hoặc liên hệ quản trị viên để được hỗ trợ.
```

**Scope áp dụng:** Message này phải được map ở cả hai flow:
- `POST /api/v1/company/initialize`
- `POST /api/v1/company/create`

**Mapping location:** `features/company/provisionErrors.ts` — thêm case `COMPANY_ALREADY_EXISTS` kèm message trên (hiện file chưa có case này).

---

## Requirement Review (SA Lens)

### Confirmed: `business_code` = số đăng ký kinh doanh / mã số doanh nghiệp

`company_profiles.info.business_code` chứa số ĐKKD / mã số doanh nghiệp. Lookup theo field này là đúng nghiệp vụ. Không có mismatch nào còn tồn tại.

### Known: Database Index Gap — Mitigated by In-Process Cache

`business_code` nằm trong cột JSON (`company_profiles.info`). Query `WHERE JSON_EXTRACT(p.info, '$.business_code') = ?` thực hiện **full table scan** trên `company_profiles`. Không có index trong migrations. Với ~1700 rows, full scan < 5ms — chấp nhận được **khi có cache**. **In-process TTL cache loại bỏ 95%+ DB calls** nên full scan chỉ xảy ra trên cache miss.

**Medium-term (nếu sở hữu vnstock DB schema):** Generated column `business_code_idx` + index. Cần confirm ownership trước khi plan migration.

### Known: `tax_code` UNIQUE Conflict Risk

Migration 0029 đã tạo `UNIQUE KEY uk_companies_tax_code (tax_code)`. Sync workflow làm tăng xác suất conflict vì user có thể sync tax_code của công ty nổi tiếng mà người khác đã đăng ký trước. **Resolved bởi Q-BLOCK-2** — giữ constraint, hiển thị lỗi rõ ràng.

### Open Questions (Non-blocking)

| # | Question | Impact | Decision / Default |
|---|---|---|---|
| ~~Q-BLOCK-1~~ | ~~business_code semantic~~ | ~~Blocker~~ | ✅ **Resolved: là số ĐKKD** |
| ~~Q-BLOCK-2~~ | ~~tax_code conflict UX~~ | ~~High~~ | ✅ **Resolved: error message rõ ràng** |
| Q-3 | vnstock 503: ẩn tính năng hay hiển thị thông báo? | Low | Hiển thị hint nhỏ "Tra cứu tạm thời không khả dụng", form vẫn submit được |
| Q-4 | Debounce delay? Min input length? | Cosmetic | 500ms, min 8 ký tự |
| Q-5 | Confirm dialog khi sync ghi đè data user đã nhập? | Low | Button label "Đồng bộ và ghi đè thông tin" — đủ rõ, không cần dialog thêm |

---

## Current State Analysis

### Reuse Map

| Requirement | Existing Component | Location | Reusable? | Gap |
|---|---|---|---|---|
| Lookup service layer | `marketreference.Service` + `ListedCompanyDetail` DTO | `internal/marketreference/app/` | YES ~90% | Chỉ thiếu method `GetByBusinessCode` |
| Lookup repo layer | `mysql.Repository.GetBySymbol` | `internal/marketreference/infra/mysql/repository.go` | YES ~70% | Thiếu query by JSON field, toàn bộ parse logic đã có |
| Public endpoint routing | `AdminHandler.Register()` + `handleSelfServiceProvision` pattern | `companyaccess/transport/http/` | YES ~80% | Cần thêm 1 route + 1 handler function, không cần middleware |
| Company provision form | `CompanyCreateForm.tsx` + `CompanyCreateFormValues` type | `features/company/CompanyCreateForm.tsx` | YES 100% | Không sửa gì — chỉ điền values từ ngoài vào |
| Form state management | `useCompanyProvision.ts` `setValues` | `features/company/useCompanyProvision.ts` | YES 100% | `setValues` đã exposed, có thể patch values từ ngoài |
| Listed company types (FE) | `ListedCompanyDetail`, `ListedCompanyLegalContact`, etc. | `features/cms-core/listedCompanies/types.ts` | YES 100% | Đã đủ fields cần thiết |
| Response parser (FE) | `parseListedCompanyDetailPayload` | `features/cms-core/services/cmsApi.ts` | YES 100% | Đã parse toàn bộ fields |
| API client pattern (FE) | `createApiClient` + `request<unknown>` | `services/authApi.ts` | YES 100% | Có thể call no-auth endpoint |
| Vnstock DB wiring | `buildListedCompaniesService` + `VnstockMarketEnabled` flag | `internal/httpserver/server.go` | YES 100% | Service đã được inject vào CMS handler, cần inject thêm vào AdminHandler |

**Kết luận reuse: >80% code đã có, effort thực tế = 1 query method + 1 handler + 1 FE hook + 1 FE component**

---

## Architecture Design

### Backend

**Chọn: Thêm endpoint vào AdminHandler (Option A từ spec)**

Lý do:
- `AdminHandler` đã xử lý `/api/v1/company/initialize` và `/api/v1/company/create` — cùng nhóm route không cần auth company context
- Không cần tạo handler mới, chỉ thêm method + inject dependency
- Consistent với routing convention đã có

**Reject:** Thêm vào `platformcms` Handler — sai layer (CMS handler có mandatory auth + CMS permission check)

**Reject:** Route prefix `/api/v1/public/...` — không cần tạo prefix mới, `/api/v1/company/...` đã là pattern cho unauthenticated provision flow

**Endpoint:**
```
GET /api/v1/company/listed-lookup?business_code={code}
```

**Dependency injection:**
```
httpserver.go
  → adminHandler.WithListedCompaniesLookup(listedCompaniesSvc)
  // listedCompaniesSvc đã được build ở buildListedCompaniesService()
  // chỉ cần pass thêm vào adminHandler
```

**Auth pattern:** Không có auth check — giống `/api/v1/company/initialize`. Handler không gọi `h.inspector.InspectAccessToken`.

### Frontend

**Trigger point:** Field `registrationNumber` trong form — khi người dùng ngừng nhập (debounce 500ms), nếu input ≥ 8 ký tự thì gọi API lookup.

**UX Contract (confirmed decisions):**

| Behavior | Rule |
|---|---|
| Trigger | Debounce 500ms sau khi `registrationNumber` thay đổi, min 8 ký tự |
| Stale preview | Reset preview card **ngay lập tức** khi input thay đổi — không chờ debounce |
| Loading indicator | Spinner nhỏ bên phải field `registrationNumber` khi đang fetch |
| Not found | Hint text nhỏ "Không tìm thấy công ty niêm yết phù hợp" — không block form |
| Lookup 503 | Hint text nhỏ "Tra cứu tạm thời không khả dụng" — **không block form submit** |
| Sync action | Chỉ sync khi user bấm button — không auto-fill |
| Sync overwrite | Button label: **"Đồng bộ và ghi đè thông tin"** — không cần confirmation dialog |
| Partial sync | Field null từ DB → giữ nguyên value user đã nhập, không điền empty string |
| Post-sync edit | User vẫn chỉnh sửa được mọi field sau khi sync |
| tax_code conflict | Giữ form data, hiển thị message từ Q-BLOCK-2 |

**UX Flow:**
```
CompanyCreateForm (hiện tại: controlled form — KHÔNG SỬA)
    │
    ▼ integration tại CreateCompanyModal
useListedCompanyLookup hook (NEW)
    ├─ state: idle | loading | found | not_found | error
    ├─ lookupResult: ListedCompanyLookupResult | null
    ├─ trigger: registrationNumber debounce 500ms, min 8 chars
    └─ on input change: reset state về idle ngay lập tức

ListedCompanyPreviewCard component (NEW)
    ├─ Render khi status === 'found'
    ├─ Preview: symbol, exchange, company_type, listing_date
    ├─ Disclaimer text (luôn hiển thị trước button)
    ├─ Button "Đồng bộ và ghi đè thông tin"
    │     → provision.setValues(mapSyncPayload(result))
    │     → chỉ patch field có value (skip null fields)
    └─ Button "Bỏ qua" → dismiss, state về idle

CreateCompanyModal (MODIFY — thêm lookup integration)
    ├─ Thêm useListedCompanyLookup(registrationNumber)
    └─ Render ListedCompanyPreviewCard bên dưới field registrationNumber

provisionErrors.ts (MODIFY — thêm COMPANY_ALREADY_EXISTS case)
    └─ COMPANY_ALREADY_EXISTS → "Mã số thuế này đã được đăng ký..."
```

**Không sửa `CompanyCreateForm.tsx`** — controlled component, sync chỉ gọi `provision.setValues(patch)` từ modal level.

---

## API Design

### Request
```
GET /api/v1/company/listed-lookup?business_code=0101234567
```
- **No auth header required** — public endpoint, áp dụng cho mọi user kể cả chưa đăng nhập
- `business_code`: số ĐKKD / mã số doanh nghiệp
- Trim whitespace trước khi validate và query
- Max length: 50 ký tự
- Trả `found: false` khi công ty tìm thấy nhưng không có `company_profiles` row (no profile)
- Trả 503 khi `VNSTOCK_MARKET_ENABLED=false` hoặc vnstock DB không kết nối được

### Response — Found (200)
```json
{
  "found": true,
  "disclaimer": "Thông tin được lấy từ dữ liệu công ty niêm yết công khai, chỉ mang tính tham khảo. Nền tảng không chịu trách nhiệm pháp lý về tính chính xác của thông tin.",
  "sync": {
    "company_name": "CÔNG TY CỔ PHẦN SỮA VIỆT NAM",
    "tax_code": "0300588569",
    "registration_number": "0101234567",
    "address": "10 Tân Trào, Phường Tân Phú, Quận 7, TP.HCM",
    "phone": "02854155555",
    "contact_email": "ir@vinamilk.com.vn"
  },
  "preview": {
    "symbol": "VNM",
    "exchange": "HOSE",
    "company_type": "Công ty cổ phần",
    "listing_date": "2006-01-19"
  }
}
```

### Response — Not Found (200)
```json
{ "found": false }
```
> **Rationale:** Trả 200 thay vì 404. Feature này là optional helper — 404 sẽ bị frontend treat như error, phức tạp hóa UX unnecessarily.

### Response — Bad Request (400)
```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "business_code is required"
  }
}
```
> Khi `business_code` rỗng sau trim.

### Response — Service Unavailable (503)
```json
{
  "error": {
    "code": "SERVICE_UNAVAILABLE",
    "message": "market reference data is unavailable"
  }
}
```
> Reuse `marketUnavailableError()` đã có trong `platformcms/transport/http/listed_companies_handlers.go`.

### Error Strategy Summary

| Case | HTTP Status | Code | Note |
|---|---|---|---|
| business_code empty | 400 | `INVALID_REQUEST` | Validate before DB call |
| Not found in DB | 200 | — | `found: false` |
| has_profile = false | 200 | — | `found: false` — no meaningful data |
| vnstock DB down | 503 | `SERVICE_UNAVAILABLE` | Frontend graceful degradation |
| vnstock disabled | 503 | `SERVICE_UNAVAILABLE` | `NewDisabledService()` already returns ErrUnavailable |

### Rate Limiting Strategy

**Mandatory: Nginx/gateway layer. KHÔNG optional.**

Nginx rate limit **phải được deploy trước Phase D (Dev Deploy)** — đây là deliverable của Phase A, không phải "gợi ý".

```nginx
# Required config — PHẢI có trong Nginx trước production
limit_req_zone $binary_remote_addr zone=listed_lookup:10m rate=10r/m;

location = /api/v1/company/listed-lookup {
    limit_req zone=listed_lookup burst=3 nodelay;
    limit_req_status 429;
    add_header Retry-After 60 always;
    proxy_pass http://backend;
}
```

**Rationale:** 10 req/phút/IP đủ cho legitimate use (debounce 500ms, user không gõ > 10 ĐKKD khác nhau mỗi phút). Burst=3 handles normal debounce. Scraper bị block sau 3 requests đầu.

### Response Headers Required

```
X-Content-Type-Options: nosniff
Cache-Control: public, max-age=3600, stale-while-revalidate=300
```

Cache-Control cho phép browser/CDN cache response — public data, không sensitive.

---

## Data Mapping Validation

### Field-by-Field Verification

| Form Field | Source Path | DB Column | Verified Exists? | Concerns |
|---|---|---|---|---|
| `company_name` | `equity_list.company_name` | `company_name VARCHAR NOT NULL` | ✅ Yes | None. Value always present. |
| `tax_code` | `company_profiles.info → tax_id` | JSON field | ✅ Yes | **UNIQUE constraint** trên `companies.tax_code` — có thể gây lỗi khi submit nếu đã tồn tại |
| `registration_number` | `company_profiles.info → business_code` | JSON field | ✅ Yes | `business_code` đã xác nhận là số ĐKKD / mã số doanh nghiệp (Q-BLOCK-1 resolved) |
| `address` | `company_profiles.info → address` | JSON field | ✅ Yes | Nullable, safe |
| `phone` | `company_profiles.info → phone` | JSON field | ✅ Yes | Nullable, safe |
| `contact_email` | `company_profiles.info → email` | JSON field | ✅ Yes | Nullable, public company contact email |

### Preview-Only Fields

| Field | Source | Verified? | Notes |
|---|---|---|---|
| `symbol` | `equity_list.symbol` | ✅ Yes | Always present, PK of equity_list |
| `exchange` | `equity_list.exchange` → fallback `info.exchange` | ✅ Yes | COALESCE logic đã có trong repo |
| `company_type` | `company_profiles.info → company_type` | ✅ Yes | Nullable |
| `listing_date` | `company_profiles.info → listing_date` | ✅ Yes | Nullable, parsed as time |

### Sync Field omission logic

Khi một field trong `sync` object là empty/null từ DB: **không điền field đó vào form** (để trống, user tự nhập). Không điền empty string vào field đã có value.

---

## Security Review

### Public Endpoint Risks

**Enumeration / Scraping:**  
- Kẻ tấn công có thể dùng endpoint này để enumerate danh sách mã ĐKKD hợp lệ.
- **Mitigation:** (1) Data này đã public (vnstock/public market data), không có lợi ích gì khi enumerate. (2) Nginx rate limit như đề xuất trên.

**DOS:**  
- Mỗi request kéo theo `JSON_EXTRACT` full scan trên `company_profiles`.
- **Mitigation:** (1) Table nhỏ (~1700 rows), full scan < 5ms. (2) Nginx rate limit. (3) Vnstock DB là read-only pool riêng, không ảnh hưởng main DB.

**Abuse — dùng làm company info API:**  
- Endpoint trả email + phone của công ty niêm yết.
- **Assessment: Acceptable.** Đây là thông tin công khai, đã được vnstock/KBS publish. Không phải dữ liệu cá nhân.

### Multi-tenant Impact

**Không có bypass nào:**
- Endpoint này chỉ đọc từ vnstock DB (read-only pool riêng).
- Không có query vào `companies` table của main DB.
- Không ảnh hưởng đến `verification_status` — doanh nghiệp vẫn tạo với `unverified`.
- Không shortcut bất kỳ RBAC check nào trong provision flow.

### Data Privacy Assessment

- `email` và `phone` từ `company_profiles.info` là thông tin liên hệ của doanh nghiệp (IR email, switchboard) — không phải dữ liệu cá nhân theo GDPR/PDPD.
- Disclaimer đã clear về trách nhiệm pháp lý.

---

## UX Review

### Vấn đề trong spec UX

**Stale preview:** Nếu người dùng đã thấy preview → xóa bớt chữ trong `registrationNumber` → gõ thêm chữ khác → preview cũ vẫn hiển thị trong khi đang debounce. Cần xử lý: khi input thay đổi, dismiss preview ngay lập tức và show loading indicator sau debounce.

**Sync overwrite behavior:** Spec nói "người dùng vẫn có thể chỉnh sửa sau sync". Nhưng nếu họ đã tự nhập `company_name` trước khi lookup → bấm sync → tên tự nhập bị overwrite. Cần UX rõ ràng: button label là "Đồng bộ (ghi đè)" hoặc show confirmation "Thao tác này sẽ ghi đè thông tin bạn đã nhập".

**Partial sync khi field null:** Nếu `phone` trong DB là null, không điền gì — field phone giữ nguyên value người dùng đã nhập trước đó. Behavior này cần document rõ trong code comment.

**Loading state:** Debounce 500ms + fetch time → user thấy form im lặng. Cần spinner/indicator nhỏ bên cạnh field `registrationNumber` khi đang loading.

### Đề xuất UX tốt hơn

```
[Số đăng ký kinh doanh field]
  Input → user nhập
  Khi đang debounce: không có gì
  Khi đang fetch: spinner nhỏ bên phải field
  Khi found: preview card xuất hiện bên dưới field (không phải modal)
  Khi not_found: hint text nhỏ "Không tìm thấy công ty niêm yết phù hợp"
  Khi error: hint text nhỏ "Tra cứu tạm thời không khả dụng"

Preview Card layout:
  ┌─────────────────────────────────────────────┐
  │ 🏢 VNM — CÔNG TY CỔ PHẦN SỮA VIỆT NAM     │
  │    HOSE · Công ty cổ phần · Niêm yết 2006  │
  │                                             │
  │ ⚠️ Thông tin chỉ mang tính tham khảo.      │
  │    Nền tảng không chịu trách nhiệm pháp lý. │
  │                                             │
  │  [Đồng bộ thông tin]  [Bỏ qua]             │
  └─────────────────────────────────────────────┘
```

---

## Test Strategy

### Backend Unit Tests

**File:** `companyaccess/transport/http/admin_handler_listed_lookup_test.go`

| Test case | Input | Expected |
|---|---|---|
| business_code found with full profile | `?business_code=0101234567` | 200 `found:true`, sync + preview populated |
| business_code found, leading/trailing whitespace | `?business_code=+0101234567+` (URL-encoded spaces) | 200 `found:true` — trim trước khi query |
| business_code not found | `?business_code=9999999999` | 200 `found:false` |
| business_code empty string | `?business_code=` | 400 INVALID_REQUEST |
| business_code whitespace only | `?business_code=+++` | 400 INVALID_REQUEST |
| vnstock unavailable | service returns ErrUnavailable | 503 SERVICE_UNAVAILABLE |
| vnstock disabled | `NewDisabledService()` wired | 503 SERVICE_UNAVAILABLE |
| company found in equity_list but no profile | equity row exists, no company_profiles row | 200 `found:false` |
| sync.tax_code present when tax_id non-null | profile has tax_id | sync object includes tax_code field |
| sync omits null fields | profile has null phone | sync object omits phone key |

**File:** `marketreference/infra/mysql/repository_test.go` (thêm test case)

| Test case | Verify |
|---|---|
| GetByBusinessCode — exact match | Returns correct ListedCompanyDetail |
| GetByBusinessCode — value with whitespace | Trim applied, matches correctly |
| GetByBusinessCode — not found | Returns ErrNotFound |
| GetByBusinessCode — equity exists, no profile | Returns ErrNotFound (JOIN vs LEFT JOIN) |

### Frontend Unit Tests

**File:** `features/company/useListedCompanyLookup.test.ts` (NEW)

| Test | Verify |
|---|---|
| Short input (< 8 chars): không gọi API | 0 fetch calls |
| Debounce: không gọi API khi chưa đủ 500ms | Mock timer, assert 0 calls before delay |
| Trigger từ registrationNumber field | Lookup fires khi registrationNumber ≥ 8 chars |
| Found: state → `found`, lookupResult populated | `status === 'found'`, result.sync có fields |
| Not found: state → `not_found` | `status === 'not_found'`, result null |
| 503 error: state → `error` | `status === 'error'` |
| Input thay đổi: preview reset ngay lập tức | state về `idle` trước khi debounce fires |
| Input cleared (empty): state → `idle` | 0 fetch calls, preview dismissed |

**File:** `features/company/ListedCompanyPreviewCard.test.tsx` (NEW)

| Test | Verify |
|---|---|
| Render: disclaimer luôn hiển thị | Disclaimer text present trước buttons |
| Render: button label "Đồng bộ và ghi đè thông tin" | Button text exact match |
| Render: preview fields (symbol, exchange, company_type) | Fields present |
| Click "Đồng bộ": gọi onSync với mapped payload | onSync called với correct CompanyCreateFormValues patch |
| Click "Đồng bộ": null fields KHÔNG được patch | Patch object omits keys where source is null |
| Click "Bỏ qua": gọi onDismiss | onDismiss called |
| Sync không auto-submit | onSubmit NOT called after sync |

**File:** `features/company/provisionErrors.test.ts` (MODIFY — thêm test case)

| Test | Verify |
|---|---|
| COMPANY_ALREADY_EXISTS maps to tax_code conflict message | Error string contains expected message |

**File:** `features/company/CompanyCreateForm.test.tsx` (đã có — chạy lại, KHÔNG SỬA)

Existing tests phải pass nguyên vẹn sau khi integrate lookup.

### E2E Test Scenarios

| Scenario | Steps | Expected |
|---|---|---|
| Initialize flow — lookup found → sync → submit | Nhập ĐKKD hợp lệ → preview card xuất hiện → bấm "Đồng bộ và ghi đè" → submit | Company created với synced data |
| Initialize flow — lookup found → dismiss → submit | Nhập ĐKKD → preview card → bấm "Bỏ qua" → nhập tay → submit | Company created với manually typed data |
| Initialize flow — lookup not found | Nhập ĐKKD không khớp | Hint "Không tìm thấy", form submit bình thường |
| Create Nth company — lookup found → sync → submit | User đã có company → open modal → lookup → sync → submit | Company created, session switched |
| vnstock 503 — form vẫn submit | Lookup disabled → nhập ĐKKD → hint nhỏ → submit form | Form submit thành công, không bị block |
| User edits after sync | Sync → chỉnh sửa company_name → submit | Saved với value đã chỉnh sửa (không phải synced value) |
| tax_code conflict after sync | Sync → submit → tax_code đã tồn tại ở company khác | Form giữ data, hiển thị "Mã số thuế này đã được đăng ký..." |
| Input change dismisses stale preview | Lookup found → xóa 1 ký tự trong registrationNumber | Preview card biến mất ngay lập tức |

---

## Deployment Strategy

### Phase A — Backend Foundation + Security (Day 1–2)

**Scope (tất cả required, không có optional):**
1. Thêm `GetByBusinessCode(ctx, businessCode string)` vào `ListedCompanyReader` interface (`marketreference/app/types.go`)
2. Implement `GetByBusinessCode` trong `marketreference/infra/mysql/repository.go` — JSON_EXTRACT query + trim + character validation
3. **NEW: In-process TTL cache** trong `marketreference/app/service.go` — `businessCodeCache` struct, TTL 60 phút (positive) / 10 phút (negative), max 2000 entries, singleflight
4. Thêm `GetByBusinessCode` service method (với cache) vào `marketreference/app/service.go`
5. Thêm field `listedLookup *marketapp.Service` + method `WithListedCompaniesLookup` vào `AdminHandler`
6. Tạo `admin_handler_listed_lookup.go` — handler với structured audit logging, input sanitization, response headers
7. Wire: `adminHandler.WithListedCompaniesLookup(listedCompaniesSvc)` trong `httpserver/server.go`
8. **NEW: Nginx rate limit config file** — 10r/m/IP, burst=3, 429 + Retry-After header
9. Unit tests: handler (all cases) + cache behavior + singleflight + input validation

**Checkpoint:**
```bash
# Basic lookup
curl "http://localhost:8080/api/v1/company/listed-lookup?business_code=0101234567"
# → 200 found:true hoặc found:false

# Empty → 400
curl "http://localhost:8080/api/v1/company/listed-lookup?business_code="

# Cache works — check logs for cache_hit=true on second call
curl "http://localhost:8080/api/v1/company/listed-lookup?business_code=0101234567"
# grep logs: cache_hit=true on second request

# Rate limit (after Nginx deploy)
for i in {1..10}; do curl -s -o /dev/null -w "%{http_code}\n" "http://localhost/api/v1/company/listed-lookup?business_code=TEST"; done
# → some 429s after first 3
```

**No migration required.**

### Phase B — Frontend UX (Day 2–3)

**Scope:**
1. Thêm `lookupListedCompany(businessCode: string)` vào `authApi.ts` — no-auth GET, reuse `createApiClient`
2. Tạo `useListedCompanyLookup.ts` — debounce 500ms, min 8 chars, state machine (idle/loading/found/not_found/error), reset preview on input change
3. **CHANGED: Smart merge logic** — `mapSyncPayload(result, currentValues)` chỉ patch field mà `currentValues[field] === ''`
4. Tạo `ListedCompanyPreviewCard.tsx` — preview fields, disclaimer, button **"Đồng bộ thông tin"** (không phải "ghi đè"), sub-text "(Chỉ điền vào các ô còn trống)"
5. Update `provisionErrors.ts` — thêm case `COMPANY_ALREADY_EXISTS` → message từ Q-BLOCK-2
6. Integrate vào `CreateCompanyModal.tsx` — thêm useListedCompanyLookup, render preview card bên dưới field registrationNumber
7. Unit tests: hook + card + provisionErrors + smart merge logic

**Checkpoint:** Dev server — user đã nhập company_name → nhập ĐKKD → preview card xuất hiện → bấm "Đồng bộ thông tin" → company_name không bị overwrite → các field trống được điền → submit → company created.

### Phase C — Observability + Integration QA (Day 3–4)

1. Verify structured log events đang emit đúng format: `grep "listed_lookup_requested" logs | jq .`
2. Verify `cache_hit` field toggling: hit on 2nd request cho cùng ĐKKD
3. E2E test toàn bộ 10 scenarios trong Test Strategy (bao gồm smart merge scenarios)
4. Load test: verify cache_hit_rate > 90% sau warmup với 50 requests cùng ĐKKD
5. Test rate limit với script (Nginx phải đã được deploy)
6. Test input sanitization: special chars, oversized input
7. Monitoring checklist (AC-11, AC-12, AC-13, AC-14)

### Phase D — Dev Deploy

- `VNSTOCK_MARKET_ENABLED=true` + `VNSTOCK_MYSQL_DSN` phải được set (đã có trong dev env)
- **Nginx rate limit config phải được deploy** — không optional, prerequisite của Phase E
- Verify monitoring checklist trước khi mark complete

### Phase E — Staging QA

- Smoke test với data thực từ KBS — verify `business_code` format là ĐKKD thực sự
- Test tax_code conflict với company đã có trên staging
- Monitor `lookup_error_rate` < 1% trong 30 phút đầu
- Verify Nginx 429 firing correctly với script

### Phase F — Production Rollout

- Standard deploy, không có schema migration
- Monitor first 30 phút:
  - `lookup_error_rate` < 1%
  - `lookup_latency_p95` < 200ms
  - `lookup_cache_hit_rate` > 80% sau 10 phút warmup
  - Không có abuse pattern từ Nginx logs (không có IP > 50 req/min)

---

## Risks & Mitigation

| Risk | Severity | Likelihood | Mitigation |
|---|---|---|---|
| ~~`business_code` là mã ngành VSIC~~ | ~~Critical~~ | — | ✅ **Resolved (Q-BLOCK-1)** — confirmed là số ĐKKD |
| `tax_code` UNIQUE conflict sau sync | Medium | Medium | ✅ **Resolved (Q-BLOCK-2)** — error message rõ, giữ form data |
| Full table scan khi cache cold/miss | Low | Certain | **Mitigated:** In-process TTL cache loại bỏ 95%+ DB calls. Full scan chỉ xảy ra trên cache miss |
| DOS via DB amplification | Medium | Medium | **Mitigated:** In-process cache + Nginx rate limit (mandatory) |
| vnstock DB unavailable | Medium | Low | Graceful degradation: 503 + hint nhỏ, form vẫn submit |
| Smart merge skips filled fields unexpectedly | Low | Low | UX rõ ràng: sub-text "(Chỉ điền vào các ô còn trống)". Unit test cover edge cases |
| Mojibake company name từ KBS | Low | Low | `fixMojibake()` đã có trong parser — reuse |
| Scraping abuse | Low | Low | Mandatory Nginx rate limit 10r/m/IP + audit log |
| singleflight dependency not in go.mod | Low | Low | Verify `golang.org/x/sync` trong go.mod trước Phase A — thường đã có |
| Vnstock DB schema externally managed | Unknown | Unknown | Clarify ownership — nếu external thì generated column roadmap không apply |

---

## Component Dependency Graph

```
[Backend]
httpserver/server.go
  └─ adminHandler.WithListedCompaniesLookup(listedCompaniesSvc)   ← đã có listedCompaniesSvc
      └─ admin_handler_listed_lookup.go (NEW file)
          └─ marketreference/app/service.go.GetByBusinessCode (NEW method)
              └─ marketreference/infra/mysql/repository.go.GetByBusinessCode (NEW method)
                  └─ company_profiles table, JSON_EXTRACT query (NO migration)

[Frontend]
CreateCompanyModal.tsx (MODIFY — thêm lookup integration)
  ├─ useListedCompanyLookup.ts (NEW hook)
  │   └─ authApi.ts.lookupListedCompany (NEW method — no-auth GET)
  └─ ListedCompanyPreviewCard.tsx (NEW component)
      └─ ListedCompanyDetail type (REUSE từ features/cms-core/listedCompanies/types.ts)
      └─ parseListedCompanyDetailPayload (REUSE từ features/cms-core/services/cmsApi.ts)
```

---

## Files to Change

### Backend (cobo_iam_services)

| File | Change Type | Description |
|---|---|---|
| `internal/marketreference/app/types.go` | MODIFY | Thêm `GetByBusinessCode` vào `ListedCompanyReader` interface |
| `internal/marketreference/app/service.go` | MODIFY | Thêm `businessCodeCache` struct + singleflight + method `GetByBusinessCode` (cache-first) |
| `internal/marketreference/infra/mysql/repository.go` | MODIFY | Implement `GetByBusinessCode` — JSON_EXTRACT + trim + char validation |
| `internal/companyaccess/transport/http/admin_handler.go` | MODIFY | Thêm field `listedLookup *marketapp.Service` + method `WithListedCompaniesLookup` |
| `internal/companyaccess/transport/http/admin_handler_listed_lookup.go` | **NEW** | Handler: input sanitize, audit log, cache-aware call, response mapping, response headers |
| `internal/httpserver/server.go` | MODIFY | `adminHandler.WithListedCompaniesLookup(listedCompaniesSvc)` |
| `deploy/nginx/listed-lookup-rate-limit.conf` (hoặc equivalent) | **NEW** | Nginx rate limit config — mandatory deliverable |

### Backend Tests

| File | Change Type | Key cases |
|---|---|---|
| `internal/companyaccess/transport/http/admin_handler_listed_lookup_test.go` | **NEW** | All handler cases + audit log emission + input sanitization |
| `internal/marketreference/app/service_test.go` | MODIFY | Cache hit/miss, TTL expiry, negative cache, singleflight |
| `internal/marketreference/infra/mysql/repository_test.go` | MODIFY | GetByBusinessCode exact match, trim, not found, no profile |

### Frontend (cobo_web_design)

| File | Change Type | Description |
|---|---|---|
| `src/services/authApi.ts` | MODIFY | Thêm `lookupListedCompany(businessCode)` — no-auth GET |
| `src/features/company/useListedCompanyLookup.ts` | **NEW** | Hook: debounce, state machine, **smart merge payload builder** |
| `src/features/company/ListedCompanyPreviewCard.tsx` | **NEW** | Button "Đồng bộ thông tin", sub-text "(Chỉ điền vào các ô còn trống)", disclaimer |
| `src/features/company/CreateCompanyModal.tsx` | MODIFY | Integrate hook + render preview card |
| `src/features/company/provisionErrors.ts` | MODIFY | COMPANY_ALREADY_EXISTS → message Q-BLOCK-2 |

### Frontend Tests

| File | Change Type | Key cases |
|---|---|---|
| `src/features/company/useListedCompanyLookup.test.ts` | **NEW** | Debounce, state machine, smart merge: skip filled fields |
| `src/features/company/ListedCompanyPreviewCard.test.tsx` | **NEW** | Button label, disclaimer, smart merge payload, no auto-submit |
| `src/features/company/provisionErrors.test.ts` | MODIFY | COMPANY_ALREADY_EXISTS message |

**No DB migrations. No new env vars (reuses `VNSTOCK_MARKET_ENABLED` + `VNSTOCK_MYSQL_DSN`).**  
**New Go dependency: `golang.org/x/sync/singleflight` (already transitively available in most Go projects — verify in go.mod).**

---

## Open Questions Summary

| # | Question | Blocking? | Status |
|---|---|---|---|
| Q-BLOCK-1 | `business_code` là ĐKKD hay mã ngành VSIC? | ~~YES~~ | ✅ Resolved: là số ĐKKD |
| Q-BLOCK-2 | UX khi tax_code conflict sau sync? | ~~YES~~ | ✅ Resolved: error message rõ, giữ form data |
| Q-3 | vnstock 503: ẩn hay hiển thị thông báo? | No | Decided: hint nhỏ, form vẫn hoạt động |
| Q-4 | Debounce ms? Min input length? | No | Decided: 500ms, min 8 ký tự |
| Q-5 | Confirm dialog khi sync ghi đè? | No | Decided: không cần dialog, button label đủ rõ |

**Không còn câu hỏi nào pending. Tất cả đã resolved.**

---

## Recommended Architecture

1. **Backend:** Một file mới `admin_handler_listed_lookup.go` trong `companyaccess/transport/http/` — không phân tán logic.
2. **Service injection:** Pass `*marketapp.Service` vào `AdminHandler` qua `WithListedCompaniesLookup()` — consistent với `WithTokenIssuer`, `WithIdempotency` pattern đã có.
3. **Frontend:** Hook `useListedCompanyLookup` tách biệt hoàn toàn khỏi `useCompanyProvision` — sync chỉ là gọi `provision.setValues(patch)` từ modal, không couple hai hook với nhau.
4. **Response format:** `{ found: bool, sync?: {...}, preview?: {...}, disclaimer?: string }` — flat, không nest sâu.

---

---

## Security Architecture

### Threat Model Summary

| Threat | Mitigation | Status |
|---|---|---|
| Enumeration of valid ĐKKD | Nginx rate limit 10r/m/IP | Mandatory — Phase A deliverable |
| Scraping email/phone | Nginx rate limit + audit log | Mandatory — Phase A deliverable |
| DOS via DB amplification | In-process cache + Nginx limit | Both required |
| Data exfiltration pattern | Structured audit log (IP + result + timestamp) | Mandatory — Phase A deliverable |

### Audit Logging Contract

Mọi request đến `GET /api/v1/company/listed-lookup` phải emit:

```go
slog.InfoContext(ctx, "listed_lookup_requested",
    slog.String("event", "listed_lookup_requested"),
    slog.String("request_id", httpx.RequestIDFromContext(ctx)),
    slog.String("ip", r.RemoteAddr),
    slog.String("user_agent", r.UserAgent()),
    slog.String("business_code_prefix", businessCode[:min(len(businessCode), 4)]),
    slog.String("result", "found|not_found|error|unavailable"),
    slog.Bool("cache_hit", cacheHit),
    slog.Int64("duration_ms", durationMs),
)
```

**Note:** Log `business_code_prefix` (4 ký tự đầu) — đủ để debug, không expose full ĐKKD.

### Input Sanitization Rules

| Rule | Validation |
|---|---|
| Trim whitespace | Trim trước khi validate |
| Max length | 50 ký tự |
| Min length after trim | 1 ký tự (rỗng → 400) |
| Character set | Alphanumeric + `-` + `/` chỉ. Reject SQL special chars |
| Null byte | Reject |

---

## Lookup Data Access Strategy

### Architecture Decision

**Chosen: In-process TTL cache + JSON_EXTRACT (no migration required)**

| Layer | Component | Description |
|---|---|---|
| Cache | `businessCodeCache` trong `marketreference/app/service.go` | sync.Map + TTL + singleflight |
| DB Query | `JSON_EXTRACT(p.info, '$.business_code')` | Full scan — acceptable with cache |
| Singleflight | `golang.org/x/sync/singleflight` | Prevent thundering herd on cold start |

### Cache Spec

| Parameter | Value | Rationale |
|---|---|---|
| Positive TTL | 60 phút | Company profiles hiếm khi thay đổi |
| Negative TTL | 10 phút | Ngăn spam cho invalid ĐKKD |
| Max entries | 2000 | 1700 companies + headroom |
| Invalidation | TTL-only | Update frequency không đòi hỏi active invalidation |
| Memory estimate | ~4MB | 2000 × ~2KB/entry |
| Dependency | `sync.RWMutex` — standard library, no new dep | — |

### Medium-term Roadmap (Post-Sprint)

- Clarify vnstock DB ownership (KBS managed vs. internal)
- If internal: plan generated column migration `ALTER TABLE company_profiles ADD COLUMN business_code_idx VARCHAR(50) GENERATED ALWAYS AS (JSON_UNQUOTE(JSON_EXTRACT(info, '$.business_code'))) STORED, ADD INDEX idx_cp_business_code (business_code_idx)`
- This eliminates the need for full scan entirely even on cache miss

---

## Cache Strategy

**Decision: In-process Go cache (Option B from architecture review)**

Rationale: 1700 entries fit in ~4MB. Data changes monthly. No network hop needed. Singleflight prevents thundering herd. No new infrastructure dependency.

**NOT Redis** — overkill at current scale, adds infra dependency.

**HTTP Cache-Control header** (supplementary):
```
Cache-Control: public, max-age=3600, stale-while-revalidate=300
```
Optional optimization — allows browser/CDN to cache. Does not reduce DB load directly.

---

## Sync UX Strategy

### Decision: Smart Merge (Option D from architecture review)

**Replaces:** "Đồng bộ và ghi đè thông tin" (full overwrite)  
**New behavior:** Chỉ patch fields hiện tại đang **empty** (`''`). Fields đã có content không bị touch.

### Smart Merge Logic

```
mapSyncPayload(syncResult, currentValues) → patch:
  patch = {}
  for each field in [companyName, taxCode, registrationNumber, address, phone, contactEmail]:
    if currentValues[field] === '' AND syncResult[field] IS NOT NULL:
      patch[field] = syncResult[field]
  return patch
```

### Button Label Update

| Before | After |
|---|---|
| "Đồng bộ và ghi đè thông tin" | "Đồng bộ thông tin" |
| — | Sub-text: "(Chỉ điền vào các ô còn trống)" |

### UX States (Updated)

| State | Display |
|---|---|
| `idle` | Không có gì |
| `loading` | Spinner bên phải field registrationNumber |
| `found` | Preview card bên dưới field. Smart merge patch preview (optional) |
| `not_found` | Hint nhỏ: "Không tìm thấy công ty niêm yết phù hợp" |
| `error` (503) | Hint nhỏ: "Tra cứu tạm thời không khả dụng" — **không block form** |

---

## Observability Strategy

### Structured Log Events

| Event | When | Required Fields |
|---|---|---|
| `listed_lookup_requested` | Every request | request_id, ip, user_agent, business_code_prefix, result, cache_hit, duration_ms |

### Key Metrics (derived from logs)

| Metric | Derive from | Alert Threshold |
|---|---|---|
| `lookup_error_rate` | result=error / total | > 5% sustained 5 min → Slack alert |
| `lookup_latency_p95` | duration_ms p95 | > 200ms → investigate |
| `lookup_cache_hit_rate` | cache_hit=true / total | < 50% after warmup → investigate |
| `lookup_abuse_signal` | requests_per_ip / min (Nginx log) | > 50/min single IP → alert + review |
| `lookup_found_rate` | result=found / total | < 10% sustained 1h → data quality check |

### SLA

| Metric | Target |
|---|---|
| Availability | 99.5% (optional feature, lower than main API) |
| P95 latency (cache hit) | < 50ms |
| P95 latency (cache miss) | < 200ms |

### Monitoring Checklist (Phase D — Dev Deploy)

- [ ] `listed_lookup_requested` events appearing in log stream
- [ ] `cache_hit` field present and toggling correctly
- [ ] Nginx access logs showing rate limit 429s for spam attempts
- [ ] No unhandled panics from new handler

---

## Acceptance Criteria (Full Set)

**AC-01** — Khi nhập ĐKKD hợp lệ của công ty niêm yết có đầy đủ profile:
- Preview card xuất hiện với tên công ty, mã CK, sàn, loại hình DN
- Disclaimer text hiển thị trước button sync
- Button "Đồng bộ và ghi đè thông tin" và "Bỏ qua" render rõ ràng

**AC-02** — Khi bấm "Đồng bộ thông tin":
- **Smart merge:** Chỉ các trường hiện tại đang empty (`''`) và có giá trị từ DB mới được điền
- Trường đã có content của user **không bị touch** — kể cả khi DB có giá trị khác
- Trường null/empty từ DB không ảnh hưởng form dù field hiện tại có trống hay không
- Người dùng chỉnh sửa được mọi field sau sync
- Nút sync không tự submit form

**AC-03** — Khi nhập ĐKKD không có trong DB niêm yết:
- Không hiện preview card
- Hiển thị hint text nhỏ "Không tìm thấy công ty niêm yết phù hợp"
- Form hoạt động bình thường

**AC-04** — Khi bấm "Bỏ qua":
- Preview card biến mất
- Form không bị điền gì
- Người dùng tự nhập và submit bình thường

**AC-05** — Khi vnstock DB không khả dụng (503):
- Hiển thị hint text nhỏ "Tra cứu tạm thời không khả dụng"
- Form submit vẫn hoạt động bình thường — lookup failure **không block submit**

**AC-06** — Disclaimer text hiển thị trong preview card trước khi user bấm sync.

**AC-07** — Sau khi tạo doanh nghiệp thành công, không có dữ liệu nào lưu liên kết giữa doanh nghiệp mới và công ty niêm yết.

**AC-08** — Tax code conflict (Q-BLOCK-2):
- Khi submit bị `COMPANY_ALREADY_EXISTS` do tax_code trùng sau khi sync:
  - Hệ thống không tạo doanh nghiệp mới
  - Form giữ nguyên toàn bộ data user đã sync/nhập
  - Hiển thị: *"Mã số thuế này đã được đăng ký bởi một doanh nghiệp khác trên hệ thống. Vui lòng kiểm tra lại thông tin hoặc liên hệ quản trị viên để được hỗ trợ."*
  - User chỉnh sửa và submit lại được
  - Áp dụng cho cả `/company/initialize` và `/company/create`

**AC-09** — business_code semantic (Q-BLOCK-1):
- API lookup dùng `company_profiles.info.business_code` — đã xác nhận là số ĐKKD / mã số doanh nghiệp
- Query param: `?business_code={số ĐKKD}`
- Input được trim whitespace trước khi validate và query

**AC-10** — Input change dismisses preview:
- Khi người dùng thay đổi nội dung field `registrationNumber` sau khi đã có preview, preview biến mất ngay lập tức (không chờ debounce)

**AC-11** — Audit logging:
- Mọi request đến `/api/v1/company/listed-lookup` phải emit structured log event `listed_lookup_requested`
- Log chứa: request_id, ip, result (found/not_found/error/unavailable), cache_hit, duration_ms
- Log KHÔNG chứa full business_code — chỉ 4 ký tự đầu

**AC-12** — In-process cache:
- Request thứ hai cho cùng business_code (trong TTL 60 phút) phải trả `cache_hit: true` trong log
- Cache miss khi TTL expired phải trigger DB query mới

**AC-13** — Rate limit:
- Nginx rate limit 10r/m/IP phải active trước khi deploy lên staging
- Request vượt limit nhận HTTP 429 với header `Retry-After: 60`

**AC-14** — Input sanitization:
- business_code > 50 ký tự → 400 INVALID_REQUEST
- business_code chứa ký tự ngoài alphanumeric, `-`, `/` → 400 INVALID_REQUEST

---

## Final Verdict

**⚠️ READY WITH ARCHITECTURE IMPROVEMENTS**

Plan cơ bản đúng. Architecture review xác định 5 improvements cần thiết để đạt production-grade. Tất cả đã được incorporate vào plan này.

**Revised Estimate:** 4–5 ngày dev  
- Backend + Security (Phase A): 2 ngày (tăng 0.5 ngày cho cache + audit log + Nginx config)
- Frontend UX (Phase B): 2 ngày (giữ nguyên)
- Observability + QA (Phase C): 1 ngày (tăng 0.5 ngày cho load test + observability verify)

**5 Changes từ architecture review (tất cả required):**
1. **C-1** In-process TTL cache + singleflight — eliminates DB amplification
2. **C-2** Nginx rate limit — mandatory deliverable Phase A (không phải ghi chú)
3. **C-3** Structured audit logging — mọi request
4. **C-4** Smart merge UX — chỉ fill empty fields, không overwrite
5. **C-5** Input sanitization — character set + max length

**Implementation order:**
1. Phase A — Backend + Security (không phụ thuộc FE)
2. Phase B — Frontend (có thể parallel với Phase A sau khi API contract lock)
3. Phase C — Observability + QA (sau khi cả hai xong)
4. Phase D → F — Deploy pipeline

**Clarification needed (non-blocking):** Vnstock DB ownership — xác định trước khi plan generated column roadmap (medium-term, không block sprint này).

**Xem chi tiết architecture analysis:** `docs/ai-cache/listed-company-lookup-sync-principal-architecture-review.md`
