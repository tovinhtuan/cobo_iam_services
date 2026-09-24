# CMS Global Record + Company Processing — implementation plan (v3)

```text
task_type: implementation-plan (pre-coding) — REVISED
date: 2026-09-24
revision: v3 — direct-publish Global CMS Record (no global PendingReview / approve queue)
supersedes: v2 PendingReview + cms.record.submit/approve + global review queue
repos: cobo_iam_services, cobo_web_design
skills: system-design-feature, integration-cross-repo
code_changed: false
migration: đề xuất §8 — chưa thực hiện, chưa đổi DB
commit: not performed
verdict: READY FOR IMPLEMENTATION AFTER API/SCHEMA FREEZE (T1) — remaining PO items only optional edge cases
related:
  - docs/cms-feature-inventory.md
  - docs/fe-route-to-be-endpoint-matrix.md
  - docs/ai-cache/cobo-iam-services-p1-1-disclosure-summary.md
  - docs/ai-cache/tenant-alert-complete-action-plan-2026-09-20/00-pointer.md
```

---

## PO decision summary (invariant)

```text
Platform CMS Admin phát hành Global CMS Record
  ≠
Company reviewer/approver duyệt / publish Company Processing Record
```

Đây là **invariant bắt buộc** của thiết kế. Hai tầng phát hành không được gộp UI, API, permission, hay hàng đợi.

| Tầng | Ai thực hiện | Ý nghĩa |
|------|--------------|---------|
| **Global CMS Record** | Platform CMS Admin | Phát hành nội dung chuẩn toàn platform (`Draft → Published`) |
| **Company Processing Record** | Company reviewer / approver / workflow | Xử lý và công bố hồ sơ **từng** company |

`Global Published` **không** nghĩa mọi company đã `Published` hoặc `Completed`.

### Đã chốt (không mở lại trừ breaking change có chủ đích)

| Chủ đề | Quyết định |
|--------|------------|
| Global lifecycle | `Draft → Published → Archived` — **không** `PendingReview` global |
| Global publish | Admin bấm **Phát hành** = đã duyệt; không bước approve thứ hai |
| Global permissions | `cms.record.publish` — **không** dùng `cms.record.submit` / `cms.record.approve` / `disclosure.approve` |
| Global review queue | **Không xây** |
| Company approval | Giữ workflow riêng; `disclosure.approve` / `workflow.step.confirm` chỉ cho company |
| `cycle_key` | Frequency-native + prefix; timezone `Asia/Ho_Chi_Minh`; ≠ deadline |
| Eligibility | System resolve; Admin **preview** rồi **Materialize** |
| Global content sau Published | **Immutable**; sửa = Archive + tạo record/version mới; không rewrite company đã tạo |
| Company override legal content | **Không** phase 1 |
| Company tham gia muộn | Incremental materialize nếu Global chưa Archived |
| Materialize phase 1 | Manual; chỉ khi `status = Published`; company tạo ở `NotStarted` |
| Event/ad-hoc global | **Ngoài phase 1** |

---

## 0. Executive summary (v3)

**Mô hình đích:**

```text
Global Template (disclosure_types)
  └── Global CMS Record  UNIQUE(template_id, cycle_key)
        ├── Company A Processing Record  UNIQUE(cms_record_id, company_id)
        ├── Company B Processing Record
        └── Company C Processing Record
```

**Vai trò Platform CMS Admin:**

1. Tạo/chỉnh sửa Template  
2. Phát hành / kích hoạt Template  
3. Tạo/chỉnh sửa Global CMS Record (Draft)  
4. **Phát hành** Global CMS Record (`Draft → Published`)  
5. (Tuỳ chọn) Materialize → Company Processing Records  

**Không có** Admin khác duyệt lại Global CMS Record.

**Hiện trạng (bảo toàn):**

- `/api/v1/platform/cms/entries*` = wrapper **company-scoped** `disclosure_records` — **không** phải Global CMS Record.
- Company `SubmitRecord`: `Draft → PendingReview` ≠ công bố pháp lý; chỉ thuộc **company** SM.
- CMS 「Hàng đợi duyệt」hiện trộn/lệch status company — sẽ **đổi nhãn** thành company workflow queue; **không** đưa Global Record vào.

**Cycle này:** chỉ cập nhật plan — không code, không migration, không đổi DB.

---

## 1. Current-state assessment

### 1.1 CMS và `disclosure_records`

| Surface | Behavior | Scope |
|---------|----------|-------|
| `GET/POST/PUT /platform/cms/entries*` | Map `entry_id` ≡ `record_id`; list/find theo `subject.CompanyID` | **Company** |
| `GET .../collections/{type_id}` | Filter by type **trong company** | Company |
| `GET/POST .../reviews*` | Approve/reject trên records company; filter lệch `PendingReview` | Company |
| Portal `/api/v1/disclosures*` | CRUD + submit/confirm | Company |
| Workers / periodic / adhoc | Materialize thẳng company records | Company |

→ Không tồn tại entity nội dung chuẩn theo chu kỳ toàn platform.

### 1.2 Vì sao entries không thể là Global CMS Record

1. Bắt buộc `company_id`.  
2. Không có `cycle_key` / `UNIQUE(template_id, cycle_key)`.  
3. Status = company lifecycle.  
4. Permission `disclosure.*` tenant.  
5. Chỉ bỏ filter `company_id` → leak + không thỏa uniqueness + status vô nghĩa.

### 1.3 Tái sử dụng vs phải tách

| Tái sử dụng | Phải tách / mới |
|-------------|-----------------|
| `disclosure_types` + versions = Global Template | Bảng/API **Global CMS Record** |
| `disclosure_records` + workflow = Company Processing | Không dùng entries làm global |
| Applicability, preferences, entitlement | Input resolve company cho materialize |
| CMS shell + template editor | Tab lịch sử + màn Global detail |
| Outbox/worker patterns | Materialize async (nếu cần scale) |
| Company submit/confirm semantics | Giữ cho company SM only |

### 1.4 Quan hệ as-is

```text
disclosure_types
       │ type_id
       ▼
disclosure_records (company_id, status, workflow…)
       ├── Portal + CMS entries (cùng SoT)
       └── (thiếu) Global CMS Record tầng giữa
```

---

## 2. Target-state architecture

### 2.1 Ba lớp

```text
Global Template
  └── Global CMS Record  UNIQUE(template_id, cycle_key)
        └── Company Processing Records  UNIQUE(cms_record_id, company_id)
```

### 2.2 Global Template

Lifecycle hiện có (giữ contract): `Draft → Published/Active → Archived` (cùng `portal_state` / activate / archive như CMS templates hôm nay).

**Rule:** Template phải **Active/Published** mới được tạo hoặc phát hành Global CMS Record (validate server-side).

### 2.3 Global CMS Record — fields tối thiểu

| Field | Ý nghĩa |
|-------|---------|
| `id` | PK |
| `template_id` | FK template |
| `cycle_key` | Kỳ nghiệp vụ (§5) |
| `title`, `summary`, `content` | Nội dung chuẩn |
| `status` | `Draft` \| `Published` \| `Archived` |
| `template_version_no` | Pin lúc **publish** |
| `created_by`, `created_at`, `updated_by`, `updated_at` | |
| `published_by`, `published_at` | Ai / khi phát hành |
| `archived_by`, `archived_at` | Optional |
| `due_date`, `cycle_start` | Lưu **riêng** — không nhầm với `cycle_key` |

**Constraint:** `UNIQUE(template_id, cycle_key)`.

### 2.4 Company Processing Record

Reuse `disclosure_records` + cột mới `cms_record_id` (nullable legacy):

| Field | Ý nghĩa |
|-------|---------|
| `id` / `record_id` | PK |
| `cms_record_id` | FK global |
| `company_id`, `type_id` | Tenant + denorm template |
| `status` | Company SM |
| assignee / `workflow_instance_id` | |
| `planned_date`, deadlines | |
| `submitted_at`, `published_date`, `completed_at` | |
| evidence / company notes | Không override legal content phase 1 |

**Constraint:** `UNIQUE(cms_record_id, company_id)` khi không null.

### 2.5 Content policy

- Phase 1: company **reference** global content (+ optional title cache list).  
- **Không** company override nội dung pháp lý.  
- Published global **immutable**; cần sửa → **Archive** record cũ + tạo Global Record mới (cycle mới hoặc revision policy — revision cùng cycle_key **cấm** bởi UNIQUE; vậy sửa = cycle mới hoặc đổi cycle_key theo PO; mặc định **Archive + tạo mới cùng cycle chỉ sau khi archive giải phóng unique** — xem §8).

**Unique sau archive:** khuyến nghị unique trên `(template_id, cycle_key)` **WHERE status <> 'Archived'** (partial/filtered unique hoặc `cycle_key` + `active_slot`). Chi tiết freeze ở T1/T2.

---

## 3. Lifecycle và hai tầng phát hành

### 3.1 Global Template

```text
Draft → Published/Active → Archived
```

### 3.2 Global CMS Record

```text
Draft → Published → Archived
```

| Status | Ý nghĩa | Được phép |
|--------|---------|-----------|
| `Draft` | Đang soạn | Sửa; **Phát hành** (nếu đủ quyền + template active) |
| `Published` | Admin đã phát hành; nội dung global có hiệu lực | Xem; Materialize; **không** sửa content |
| `Archived` | Ngừng materialize mới; giữ lịch sử | Chỉ xem; không materialize thường |

**Loại bỏ hoàn toàn ở tầng global:**

- `PendingReview`
- `cms.record.submit` / `cms.record.approve`
- Global review queue
- Approve/reject API cho Global Record
- Label UI: 「Gửi duyệt」, 「Chờ duyệt CMS」, 「Duyệt Global Record」

### 3.3 Company Processing Record (canonical đề xuất)

```text
NotStarted → InProgress → PendingReview → Approved → Published → Completed
```

(`PendingReview` ở đây = **company** chờ reviewer — không liên quan global.)

Mapping từ status hiện có: giữ trong T8; Portal submit vẫn `Draft → PendingReview` company.

### 3.4 Quan hệ giữa hai tầng

| Sự kiện | Ảnh hưởng |
|---------|-----------|
| Global **Phát hành** | `Draft → Published`; pin template version; **không** tạo company auto (phase 1) |
| Materialize | Tạo company `NotStarted`; global không đổi |
| Company reject / late / complete | Chỉ company đó |
| Company Published/Completed | Không đổi global |
| Global Archive | Chặn materialize mới; company đang chạy giữ nguyên |

---

## 4. Flow chính (direct publish)

```text
1. Platform CMS Admin tạo/cập nhật Template Draft
2. Admin phát hành / kích hoạt Template
3. Admin tạo Global CMS Record cho một cycle → Draft
4. Admin chỉnh sửa nội dung (Lưu nháp)
5. Admin bấm Phát hành  →  Draft → Published  (+ audit, pin version)
6. Hệ thống / Admin mở preview danh sách company eligible
7. Admin bấm Materialize tới doanh nghiệp
8. Hệ thống tạo Company Processing Records (idempotent)
9. Company workflow xử lý độc lập
10. Company submit / approve / publish / complete riêng
```

### Nút UI (Global Record)

| Nút | Hành động | Permission |
|-----|-----------|------------|
| **Lưu nháp** | Persist Draft | `cms.record.write` |
| **Phát hành** | Draft → Published | `cms.record.publish` |
| **Lưu và phát hành** | Save + publish (atomic UX) | write + publish |
| **Materialize tới doanh nghiệp** | Fan-out | `cms.record.materialize` |

Không dùng: Gửi duyệt / Chờ duyệt CMS / Duyệt Global Record.

---

## 5. `cycle_key`

| Periodicity | Format |
|-------------|--------|
| Daily | `daily:YYYY-MM-DD` |
| Weekly | `weekly:YYYY-Www` |
| Monthly | `monthly:YYYY-MM` |
| Quarterly | `quarterly:YYYY-Qn` |
| Yearly | `yearly:YYYY` |

- Timezone tính kỳ: **`Asia/Ho_Chi_Minh`**.  
- `cycle_key` = kỳ nghiệp vụ — **không** phải deadline.  
- `due_date`, `cycle_start` lưu cột riêng.  
- Event-based / ad-hoc Global Record: **không thuộc phase 1**.  
- DB: unique theo `(template_id, cycle_key)` (với chiến lược archive — T2).

---

## 6. Materialization / fan-out

### 6.1 Preconditions

```text
Global CMS Record.status = Published
Template Active/Published
User has cms.record.materialize
```

**Từ chối** materialize khi Draft hoặc Archived (Archive backfill = thao tác đặc biệt + quyền + audit — không phase 1 default).

### 6.2 Resolve company eligible

1. Template `applicability_rules` × company profile  
2. Company **active**  
3. Entitlement / subscription hợp lệ  
4. `company_type_preferences.auto_create_enabled` (nếu có)  
5. `applicable_from` / `applicable_to`  
6. Exclusion list hợp lệ  

Output preview: include / skip + reason.

### 6.3 Phase 1 behavior

- **Manual** materialize sau preview.  
- Modes: `dry_run` | `full` | `incremental`.  
- Idempotent `UNIQUE(cms_record_id, company_id)` → `created | exists | skipped | failed`.  
- Company record status khởi tạo: **`NotStarted`**.  
- Run summary + audit per company.  
- Partial failure: giữ bản đã tạo; retry failed.

### 6.4 Company tham gia sau khi Global Published

- Global **chưa Archived** → incremental materialize OK.  
- Tạo `NotStarted`; không reset siblings.  
- Không duplicate `(cms_record_id, company_id)`.  
- Nếu đã qua kỳ hạn: giữ deadline kỳ gốc; đánh dấu late nếu nghiệp vụ yêu cầu (PO chi tiết deadline — implementation follow existing deadline engines).  
- Archived → không auto; backfill đặc biệt có quyền + audit.

### 6.5 Sửa Global sau fan-out

- Không sửa Published.  
- Archive + tạo Global Record mới (theo unique policy).  
- Company records cũ **không** auto-update content/workflow.

### 6.6 Sync vs async

- N nhỏ: sync + report.  
- N lớn: outbox/worker (phase sau). Không bắt buộc phase 1.

---

## 7. Permission / security

### 7.1 Global (Platform CMS Admin role có thể gom, BE check từng action)

| Permission | Ý nghĩa |
|------------|---------|
| `cms.template.read` | Xem Template |
| `cms.template.write` | Tạo/sửa Template |
| `cms.template.activate` | Phát hành/kích hoạt Template |
| `cms.record.read` | Xem Global CMS Record (+ company summary) |
| `cms.record.write` | Tạo/sửa Global **Draft** |
| `cms.record.publish` | **Phát hành** Global (`Draft → Published`) |
| `cms.record.materialize` | Materialize |

**Đã loại:** `cms.record.submit`, `cms.record.approve` (không dùng cho global).

### 7.2 Company only

| Permission | Dùng cho |
|------------|----------|
| `disclosure.view` / `update` / `submit` / `approve` | Company processing |
| `workflow.step.confirm` | Company workflow steps |

**`disclosure.approve` không cho phép** `cms.record.publish`.

### 7.3 Break-glass

`rbac.manage`, `system.settings`: chỉ legacy/break-glass giai đoạn chuyển tiếp — **không** business permission bình thường sau khi policy mới áp dụng.

### 7.4 Rules

- Platform admin không mặc nhiên sửa company workflow/evidence.  
- Company user không đọc/sửa Global CMS Record (403).  
- Materialize: server resolve allow-list; không tin `company_ids` client ngoài set.  
- Mọi check **server-side**.

---

## 8. Database / migration strategy (đề xuất — chưa chạy)

### 8.1 Khuyến nghị: bảng `cms_global_records` riêng

- Tránh discriminator nguy hiểm trên `disclosure_records`.  
- `disclosure_records.cms_record_id` NULLABLE + unique `(cms_record_id, company_id)`.  
- Bảng `cms_materialization_runs` / `_items` cho audit outcomes.

### 8.2 Unique vs Archive

Options (T2 freeze một):

1. Partial unique: unique chỉ khi `status IN ('Draft','Published')`.  
2. Archive đổi `cycle_key` → `cycle_key || '#archived#' || id` (giữ lịch sử, giải phóng slot).  

Khuyến nghị nghiêng **(1)** nếu MySQL version hỗ trợ functional/partial index; không thì **(2)**.

### 8.3 Backfill

- Legacy company rows: `cms_record_id = NULL`, badge Legacy.  
- **Không** tự bịa Global từ N company rows. Promote thủ công nếu PO cần sau.

### 8.4 Rollback

Down migration drop FK/column/table mới; Portal tiếp tục với NULL.

---

## 9. Template versioning

- Lúc **publish** Global: pin `template_version_no` (= active version lúc đó, hoặc version Admin chọn nếu UI cho phép).  
- Published immutable → content + pin không đổi.  
- Company materialize đọc pin của Global.  
- Template đổi sau đó không rewrite Global đã Published hay company đã tạo.

---

## 10. API plan

### 10.1 Legacy — giữ semantics, không đổi âm thầm

| API | Quyết định |
|-----|------------|
| `/platform/cms/entries*` | Legacy **company**; deprecate dần; không = Global |
| `/platform/cms/reviews*` | Đổi nghĩa UI → **Company Workflow Queue** chỉ `PendingReview` company; không chứa Global |
| Portal `/api/v1/disclosures*` | Giữ; thêm read-only `cms_record_id`, `cycle_key`, global ref |

### 10.2 Global APIs

---

```text
Method + path
POST /api/v1/platform/cms/templates/{template_id}/records

Scope: platform
Mục đích: Tạo Global CMS Record Draft
Permission: cms.record.write
Body: { cycle_key, title, summary, content, due_date?, cycle_start? }
Response: Global DTO (status=Draft)
Idempotency: UNIQUE → 409 CONFLICT
Preconditions: Template Active/Published
Errors: 404 template; 409 duplicate cycle; 400 invalid cycle_key; 403
Audit: cms_global_record.create
```

---

```text
Method + path
GET /api/v1/platform/cms/templates/{template_id}/records

Scope: platform
Permission: cms.record.read
Query: status, cycle_key, q, page, page_size, sort
Response: items + company_record_counts aggregates
Errors: 404 template; 403
```

---

```text
Method + path
GET /api/v1/platform/cms/records/{record_id}
PUT /api/v1/platform/cms/records/{record_id}

Scope: platform
GET permission: cms.record.read
PUT permission: cms.record.write
PUT rules: CHỈ khi status=Draft; Published/Archived → 409 STATE_CONFLICT
Audit: cms_global_record.update
```

---

```text
Method + path
POST /api/v1/platform/cms/records/{record_id}/publish

Scope: platform
Mục đích: Draft → Published (direct publish — không approval decision)
Permission: cms.record.publish
Body: { note? }   // optional; KHÔNG có decision approve/reject
Effects:
  - status=Published
  - published_by, published_at
  - pin template_version_no
  - validate required content + template active + cycle_key
Idempotency: đã Published → 200 no-op HOẶC 409 (chọn một ở T1 freeze; khuyến nghị 200 no-op + same DTO)
Audit: cms_global_record.publish (bắt buộc — hành động quan trọng dù không có bước approve thứ hai)
Errors: 409 nếu Archived hoặc không Draft (trừ idempotent Published); 403 thiếu publish; 400 validation
```

**Không tạo:**

```text
POST .../records/{id}/submit
POST .../records/{id}/review
GET  .../global-reviews
```

---

```text
Method + path
POST /api/v1/platform/cms/records/{record_id}/archive

Scope: platform
Permission: cms.record.publish hoặc cms.record.write+activate-class (freeze T1; khuyến nghị publish hoặc quyền archive riêng sau)
Effects: Published|Draft → Archived; chặn materialize thường
Audit: cms_global_record.archive
```

---

```text
Method + path
GET /api/v1/platform/cms/records/{record_id}/company-records
POST /api/v1/platform/cms/records/{record_id}/materialize

GET: cms.record.read — list children
POST: cms.record.materialize
Body: { mode: "dry_run"|"full"|"incremental", company_ids?: [] }
Preconditions: status=Published
Idempotency: UNIQUE(cms_record_id, company_id)
Audit: cms_materialization.run + per-company outcomes
Errors: 409 nếu Draft/Archived; 403; partial results 200 với failed[]
Tenant: chỉ company trong eligible set
```

### 10.3 Company APIs (Portal)

```text
GET/PATCH /api/v1/disclosures/{id}
POST /api/v1/disclosures/{id}/submit
POST /api/v1/disclosures/{id}/confirm
```

Additive read-only: `cms_record_id`, `cycle_key`, global content reference/version.  
Không cho sửa Global qua Company API.

---

## 11. UI / UX

### 11.1 Template detail — tab 「Lịch sử bản ghi CMS」

Cột: Cycle | Title | Global status | Template version | Created by/at | Published at | #companies | #in progress | #completed | Actions (View / Edit Draft / Materialize|View companies).

### 11.2 Global CMS Record detail

- Nội dung; template/cycle; status Draft/Published/Archived.  
- Người tạo; **người phát hành**; thời điểm phát hành.  
- Audit timeline.  
- Company table + materialization preview + run results.  

Nút theo status:

| Status | Sửa | Phát hành | Materialize |
|--------|-----|-----------|-------------|
| Draft | Có (write) | Có (publish) | Không |
| Published | Không | — | Có (materialize) |
| Archived | Không | Không | Không (thường) |

### 11.3 「Hàng đợi duyệt」CMS

- **Không** Global CMS Review Queue.  
- Đổi nhãn → **「Hàng đợi xử lý doanh nghiệp」** / Company Workflow Queue.  
- Chỉ Company Processing Records `PendingReview`.  
- Không đưa Global Record vào queue này.

### 11.4 Tab Bản ghi legacy

1. Feature flag ẩn nav.  
2. Redirect → Template history hoặc Global detail.  
3. Không đổi semantics `/cms/content/entries`.  
4. Cleanup sau test/redirect ổn định.

---

## 12. Audit và safety (publish vẫn critical)

**Audit bắt buộc:** creator; editors; **publisher**; timestamps; template version; cycle_key; materialization run + company outcomes.

**Gates trước publish:**

- Template Active/Published  
- `cycle_key` hợp lệ theo periodicity  
- Chưa tồn tại active duplicate `(template_id, cycle_key)`  
- Required content đủ  
- User có `cms.record.publish`  

**Gates trước materialize:**

- Global `Published`  
- User có `cms.record.materialize`  
- Chỉ company eligible  

---

## 13. Test plan

### Global lifecycle
- Draft sửa được; Published/Archived PUT → conflict.  
- Publish: Draft → Published thẳng; **không** PendingReview; **không** gọi approval API.  
- Publish ghi audit + pin version.  
- Archived không materialize (default).  
- Duplicate cycle → 409.

### Permission
- `write` không `publish`: tạo/sửa Draft OK; publish 403.  
- `publish`: publish OK.  
- `disclosure.approve` ↛ publish Global.  
- Company user ↛ Global read/write.  
- Sau policy mới: `rbac.manage` không đủ như business publish.

### Materialization
- Chỉ Published; Draft → 409.  
- N companies; retry no dup; incremental chỉ company mới.  
- Company reject/publish/complete không đổi global.

### Regression
- Company workflow vẫn cần company approver.  
- Portal disclosures contract.  
- Legacy entries vẫn company-scoped.  
- Template activate ≠ Global publish (hai action/state).

### FE
- Labels: Lưu nháp / Phát hành / Materialize — không 「Gửi duyệt」.  
- Queue chỉ company PendingReview.  
- Flag ẩn Bản ghi.

---

## 14. Ordered tasks (v3)

| ID | Goal | Files/packages | API/schema | Deps | Acceptance | Tests | Risk / rollback |
|----|------|----------------|------------|------|------------|-------|-----------------|
| **T0** | Chốt direct-publish + perms (done in this doc) | ADR pointer | — | — | PO summary invariant | — | — |
| **T1** | Freeze OpenAPI/DTO: publish not submit/review; cycle_key regex; unique-vs-archive | docs/contracts | Contract | T0 | Spec reviewed | contract stubs | Drift FE/BE |
| **T2** | DDL `cms_global_records`, runs, `cms_record_id` nullable | migrations | Schema | T1 | up/down DEV | migration | Unique archive |
| **T3** | Create/update/**publish**/archive/list/get APIs | platformcms / cmsrecords | New routes | T2 | Direct publish works | API suite | Authz |
| **T4** | Template history tab | FE templates + cmsApi | Consumes list | T3 | History UI | component | |
| **T5** | Global detail + company summary | FE page | get + company-records | T3–T4 | Detail UX | e2e | |
| **T6** | Materialize preview/run/idempotency | app + optional worker | materialize | T2–T3 | dry_run/full/incremental | materialize suite | Partial fail |
| **T7** | Link `disclosure_records.cms_record_id`; Portal additive fields | disclosure | Additive | T6 | NULL legacy OK | integration | |
| **T8** | Tách global state vs company SM docs + mapping | disclosure + FE labels | — | T7 | No status mix | status tests | |
| **T9** | Seed `cms.record.*`; audit; regression; rename company queue | authz + FE publishing | Perms | T3–T8 | Matrix PASS | permission + regression | Alias quá rộng |
| **T10** | Hide/redirect legacy Bản ghi | CmsLayout | — | T4–T5 | Flag OK | layout | Bookmarks |
| **T11** | Optional global search | FE + API | Search | T3 | Admin search | | |
| **T12** | Backfill tools, rollout flags, cleanup entries list | scripts/docs | — | T9–T10 | Release checklist | smoke | Xóa sớm |

---

## 15. Rollout

1. T2–T3 behind `cms_global_records_enabled`.  
2. T4–T5 Platform CMS only.  
3. T6 materialize manual trên DEV → staging.  
4. T9 queue rename + perms.  
5. T10 ẩn Bản ghi khi thay thế đủ.  
6. T12 cleanup cuối.

---

## 16. Open questions còn lại (edge — không đụng invariant)

1. Unique slot sau Archive: partial unique vs rename cycle_key?  
2. Quyền **Archive**: gộp `cms.record.publish` hay permission riêng?  
3. Publish idempotent: 200 no-op vs 409?  
4. `Lưu và phát hành` = một transaction BE hay hai call FE?  
5. Periodic worker tương lai: auto-tạo Global Draft hay chỉ gợi ý Admin?  
6. Deadline “late” khi incremental sau due — chi tiết engine nào stamp?

---

## 17. Kết luận

- **Mẫu** = Global Template.  
- **Global CMS Record** = đúng một nội dung chuẩn / `(template_id, cycle_key)`; Admin CMS **publish trực tiếp** `Draft → Published`.  
- **Company Processing Record** = bản ghi con; workflow/duyệt/publish **riêng**.  
- CMS quản lý global content + materialize; Portal quản lý xử lý company.  
- `/cms/entries` company-scoped **không** phải nguồn Global cho đến khi có model mới.  
- Không Global review queue; không `PendingReview` global; publish dùng `cms.record.publish`.  
- Tab Bản ghi tổng chỉ ẩn sau history + detail + materialize đủ.

**Plan sẵn sàng cho implementation sau khi T1 freeze API/schema.**  
**Tuyệt đối chưa sửa code / migration / DB trong lần cập nhật tài liệu này.**

---

## Docs consulted

- `docs/ai-cache/README.md`  
- Plan v2 cùng path (rewritten → v3)  
- `docs/cms-feature-inventory.md`  
- Evidence prior: `platformcms` entries/reviews; `SubmitRecord` company semantics
