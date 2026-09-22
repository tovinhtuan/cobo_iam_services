# Portal company-resolved deadline display — implementation plan (LOCKED)

```text
task_type: implementation-plan (locked) + implemented 2026-09-22
date: 2026-09-22
updated: 2026-09-22 (absolute-due List/Detail SoT alignment + implementation)
repos: cobo_iam_services, cobo_web_design
skill: integration-cross-repo
verdict: READY FOR IMPLEMENTATION
implementation_result: docs/ai-cache/portal-resolved-deadline-display-implementation-2026-09-22.md
code_changed: true (see implementation result)
migration: none
new_api: none (extend existing Detail DTO only)
blocker_resolved: List/Detail absolute due source-of-truth unified
```

Path: `docs/ai-cache/portal-resolved-deadline-display-implementation-plan-2026-09-22.md`

Related cache:

- `cobo_web_design/docs/ai-cache/portal-company-resolved-deadline-display-2026-09-04/`
- `cobo_web_design/docs/ai-cache/resolved-deadline-rule-implementation-2026-07-31/`
- IAM pointer: `docs/ai-cache/portal-company-resolved-deadline-display-2026-09-04/00-pointer.md`

---

## 1. Executive summary

Portal phải hiển thị deadline đã resolve theo Company. **Method A** giữ nguyên: reuse API + engine; FE không tính ngày; không migration; không endpoint mới.

**Architectural blocker đã xác định (evidence §4):**

| Surface | Absolute due hôm nay | Provenance |
|---------|----------------------|------------|
| Portal List | `resolved_due_at` qua `enrichPortalListResolvedDue` | Persisted cycle **trước** preview (`CYCLE_DUE` / `PLANNED_DATE` / `DEADLINE_SUMMARY_PREVIEW`) |
| Portal Detail | `deadline_summary.deadline_date` + `resolved_deadline_rule.due_date` từ calculator | **Chỉ preview** — `GetTypeDetail` **không** đọc `periodic_cycles` |

→ Cùng Company + Template có thể hiện **hai ngày khác nhau** khi cycle đã materialize (List = due đã chốt, Detail = preview theo config hiện tại). Detail **không** có `resolved_due_at` / `resolved_due_source`.

**Solution đã khóa:** mở rộng **Detail response hiện tại** (additive) với `resolved_due_at` + `resolved_due_source`, reuse cùng precedence + cùng cycle reader/helper như List. Không tạo API `deadline-resolution`. Không ghi ngược preview vào DB.

---

## 2. Decisions locked

### 2.1 Architecture (Method A)

| Quyết định | Giá trị |
|------------|---------|
| API mới | **Không** |
| Deadline engine mới | **Không** |
| Tính deadline ở FE | **Không** |
| Migration | **Không** (additive JSON trên DTO hiện có) |
| Mở rộng Detail DTO | **Có** — `resolved_due_at`, `resolved_due_source` |
| Company ID | Chỉ authenticated subject — không nhận `company_id` từ FE |
| Absolute due SoT List ↔ Detail | **Cùng precedence, cùng resolver path** |

### 2.2 Source-of-truth (hai precedence riêng — không gộp)

**Semantic deadline text:**

```text
resolved_deadline_rule
→ raw deadline_rule (nguyên văn)
→ Chưa xác định
```

**Absolute due date:**

```text
Persisted cycle due (CYCLE_DUE | PLANNED_DATE)
→ Backend preview (DEADLINE_SUMMARY_PREVIEW)
→ Không hiển thị ngày
```

### 2.3 Absolute due — phương án khuyến nghị (LOCKED)

Mở rộng API Detail hiện tại:

```json
{
  "resolved_due_at": "2026-10-30T23:59:59+07:00",
  "resolved_due_source": "CYCLE_DUE"
}
```

Sources hợp lệ: `CYCLE_DUE` | `PLANNED_DATE` | `DEADLINE_SUMMARY_PREVIEW`.

Behavior Detail (giống List):

1. Company từ authenticated subject.
2. Xác định current logical cycle = `ResolveLogicalSlot(freq, now, Asia/Ho_Chi_Minh)`.
3. Đọc persisted due (company + type_id + cycle_label) qua reader hiện có (`ListPortalListCycleDues` với 1 type, hoặc helper shared).
4. Nếu có → set `resolved_due_at` + source `CYCLE_DUE`/`PLANNED_DATE`.
5. Nếu không → preview calculator hiện tại → `DEADLINE_SUMMARY_PREVIEW`.
6. Nếu không resolve → không hiện ngày; semantic vẫn theo precedence riêng.
7. **Không** ghi ngược preview vào DB; **không** rewrite cycle khi chỉ đọc.

Ưu tiên: **tách/reuse** logic từ `enrichPortalListResolvedDue` / `list_resolved_due.go` — không duplicate.

### 2.4 Phương án fallback (KHÔNG chọn cho Phase 1)

Nếu không đưa persisted due vào Detail:

- Semantic List/Detail giống nhau vẫn bắt buộc.
- Absolute due có thể lệch; UI bắt buộc gắn nhãn List=cycle / Detail=preview.
- AC **không** yêu cầu hai ngày luôn giống.
- Ghi nhận là rủi ro được chấp nhận.

**Recommendation mặc định: không chấp nhận fallback.** Ưu tiên mở rộng Detail contract. → Verdict Phase 1 = implement phương án khuyến nghị.

### 2.5 Data roles / Catalog / CMS

Không đổi so với lock trước: `deadline_rule` raw; Catalog authoring-only; không `deadline_rule_id`; CMS labels raw vs engine; không Company preview thật (Phase 2).

---

## 3. Current architecture

```text
Template version
  ├── deadline_rule
  ├── deadline_config
  └── applicability_rules

Company runtime
  ├── CompanyApplicabilityProfile
  ├── CompanyDeadlineContext (+ CompanyTypePreference anchors)
  └── periodic_cycles (+ disclosure_records.planned_date)

List absolute due
  ListPortalListCycleDues(company, typeIDs[])
    key match: type_id + "|" + ResolveLogicalSlot(...)
    planned_date > cycle.due_date
  else CalculateDeadlineSummary → DEADLINE_SUMMARY_PREVIEW
  → DisclosureTypeSummaryDTO.resolved_due_at / resolved_due_source

Detail absolute due (TODAY — gap)
  CalculateDeadlineSummary only
  → deadline_summary.deadline_date
  → attachResolvedDueDate → resolved_deadline_rule.due_date
  ✗ no cycle read, ✗ no resolved_due_source on DisclosureTypeDTO
```

---

## 4. Evidence from source code (verify bắt buộc)

### 4.1 Answers to analysis questions

| # | Câu hỏi | Evidence / kết luận |
|---|---------|---------------------|
| 1 | `GetTypeDetail` có đọc persisted cycle due? | **Không.** `service.go` ~651–734: profile → ResolveDeadlineRule → `CalculateDeadlineSummary` → `attachResolvedDueDate`. Không gọi `ListPortalListCycleDues`. |
| 2 | `ListTypes` đọc persisted cycle ở đâu? | `enrichPortalListResolvedDue` (`list_resolved_due.go`): `repo.(portalListCycleDueReader).ListPortalListCycleDues` rồi match slot. |
| 3 | Repo method reuse cho Detail? | **Có.** `ListPortalListCycleDues(ctx, companyID, typeIDs)` — MySQL `infra/mysql/repository.go` ~2374+; inmemory stub. Detail gọi với `[]string{typeID}`. |
| 4 | Cycle key? | `company_id` + `type_id` + `cycle_label`, trong đó `cycle_label = ResolveLogicalSlot(freq, now, loc)` (`effective_schedule.go`). Prefers `planned_date` else `pc.due_date`. |
| 5 | Khi nào List `resolved_due_at` ≠ Detail `deadline_summary.deadline_date`? | (a) Cycle đã materialize với due cũ, Template/config/N đổi → List = persisted, Detail = preview mới. (b) `planned_date` khác calculator. (c) Detail gắn due lên `resolved_deadline_rule.due_date` từ summary trong khi List đã lấy cycle. **Không được coi hai field tương đương.** |
| 6 | Shared helper List+Detail? | **Được và nên.** Tách resolve absolute due (cycle lookup + preview fallback + FormatResolvedDueAtHCMEOD) từ `list_resolved_due.go`; List batch, Detail single. |
| 7 | Mở rộng Detail phá compat? | **Không** nếu additive `omitempty` trên `DisclosureTypeDTO`. Client cũ bỏ qua field. FE `normalizeDisclosureTypeDetailResponse` đã đi qua `normalizeDisclosureTypeSummary` (đã map `resolved_due_at` / `resolved_due_source`) — rolling deploy an toàn khi BE chưa có field (null). |
| 8 | Batch vs single? | Detail: **một** query cycle (1 company × 1 type) + preview nếu miss. List giữ batch — không N+1. |

### 4.2 Field presence today

| Field | List (`DisclosureTypeSummaryDTO`) | Detail (`DisclosureTypeDTO`) |
|-------|:---------------------------------:|:----------------------------:|
| `resolved_deadline_rule` | Có | Có |
| `resolved_due_at` | Có | **Thiếu** |
| `resolved_due_source` | Có | **Thiếu** |
| `deadline_summary` | Không (đầy đủ) | Có |
| `deadline_rule` | Có | Có |

---

## 5. Runtime data flow (target)

### List (giữ)

```text
ListTypes(subject.CompanyID)
  → enrichPortalListResolvedDue (batch cycles + preview)
  → resolved_deadline_rule + resolved_due_at + resolved_due_source
```

### Detail (sau fix)

```text
GetTypeDetail(subject.CompanyID, type_id)
  → resolved_deadline_rule (semantic — không đổi)
  → deadline_summary (calculator — giữ cho status/remaining/debug)
  → resolveAbsoluteDueForType (shared):
       cycle(company, type, ResolveLogicalSlot) ?
         → resolved_due_at + CYCLE_DUE|PLANNED_DATE
       : preview from summary.deadline_date ?
         → resolved_due_at + DEADLINE_SUMMARY_PREVIEW
       : omit due
  → FE Detail: hạn chót từ resolved_due_at; nguồn từ resolved_due_source
```

**UI absolute due:** luôn ưu tiên `resolved_due_at` khi có. Không dùng `deadline_summary.deadline_date` để **ghi đè** persisted cycle. Summary vẫn có thể tồn song song cho metadata khác; ngày hiển thị Portal lấy từ `resolved_due_at`.

Semantic vẫn từ `resolved_deadline_rule` (không đổi bởi absolute resolver).

---

## 6. Raw vs resolved data contract

| Field | List | Detail | Source | Meaning |
|-------|:----:|:------:|--------|---------|
| `resolved_deadline_rule` | Có | Có | Profile + applicability | Semantic rule |
| `resolved_due_at` | Có | **Cần bổ sung** | Cycle rồi preview | Absolute due (RFC3339 HCM EOD) |
| `resolved_due_source` | Có | **Cần bổ sung** | Cycle/preview | Provenance |
| `deadline_summary` | Không bắt buộc | Có | Calculator | Calculation details / status |
| `deadline_summary.deadline_date` | — | Có | Preview path input | **Không** tương đương `resolved_due_at` khi có cycle |
| `deadline_rule` | Có | Có | Template raw | Legacy fallback nguyên văn |

Ghi rõ:

- `resolved_due_at` **không** luôn cùng source với `deadline_summary.deadline_date`.
- Persisted cycle due **thắng** preview.
- Không dùng summary date để ghi đè persisted cycle due.
- Sau Phase 1: List và Detail cùng Company + cùng slot → cùng `resolved_due_at` + cùng `resolved_due_source` (trừ race cực hẹp giữa hai request).

---

## 7. Source-of-truth precedence

Xem §2.2. Bổ sung policy cycle:

- Cycle đã materialize: giữ due khi Template đổi (không rewrite khi chỉ đọc Portal).
- Cycle mới: materialize theo config mới.
- Preview không persist.

---

## 8. Portal UX specification

### Portal List

```text
Trong vòng 30 ngày làm việc
Hạn chót: 30/10/2026
```

- Chính: `resolved_deadline_rule`.
- Phụ: `resolved_due_at`.
- Source badge: **optional**; recommendation **không** trên card (chỉ Detail).

### Portal Detail

```text
Thời hạn áp dụng
Trong vòng 30 ngày làm việc kể từ ngày kết thúc quý

Căn cứ
Công ty có công ty con

Hạn chót kỳ
30/10/2026

Nguồn
Theo chu kỳ đã tạo
```

| `resolved_due_source` | Copy |
|-----------------------|------|
| `CYCLE_DUE` | Theo chu kỳ đã tạo |
| `PLANNED_DATE` | Theo ngày kế hoạch |
| `DEADLINE_SUMMARY_PREVIEW` | Ngày dự kiến theo cấu hình |
| null / omit | Không hiển thị dòng Nguồn |

Khi preview: UI **phải** nói rõ ngày dự kiến, không phải ngày đã chốt. Không chỉ dùng màu.

Fallback semantic: raw nguyên văn → “Chưa xác định được thời hạn áp dụng”. Không parse `T+N`.

### Irregular / custom

List/Detail skip absolute periodic due (giữ behavior hiện tại). Semantic: raw event-relative nếu không có resolved DTO.

---

## 9. CMS UX specification

Hai khu vực (Phase 1 = labels only):

1. **Deadline rule hiển thị Portal** — `deadline_rule` raw / fallback.
2. **Cấu hình engine** — mode, days, day type, structure, period (`deadline_config` / `applicability_rules`).

Không preview Company thật; không `deadline_rule_id`; Catalog authoring only.

---

## 10. Backend impact

| Việc | Quyết định |
|------|------------|
| Thêm field trên `DisclosureTypeDTO` | **Cần** — `ResolvedDueAt`, `ResolvedDueSource` |
| Shared absolute-due helper | **Cần** — extract/reuse từ `list_resolved_due.go` |
| `GetTypeDetail` gọi helper | **Cần** — cycle trước preview |
| Đổi calculator / engine | **Không nên** |
| API mới / migration | **Không** |
| `deadline_summary` | Giữ; không dùng để override cycle due trên UI |
| `attachResolvedDueDate` | Có thể giữ cho semantic DTO; **Portal hạn chót** lấy `resolved_due_at` sau khi SoT thống nhất (tránh gắn preview lên due_date khi đã có cycle — implement phải không để FE ưu tiên nhầm summary) |

Implement note: khi có cycle due, set `resolved_due_at` từ cycle; optional sync `resolved_deadline_rule.due_date` về cùng ngày persisted để không lệch nội bộ DTO — hoặc FE chỉ đọc `resolved_due_at` cho hàng “Hạn chót”. Plan khóa: **FE Detail dùng `resolved_due_at` làm absolute display authority** khi field có mặt.

---

## 11. Frontend impact

| Việc | Mô tả |
|------|--------|
| Types / normalizer | Detail đã map qua summary path — verify wire; null-safe rolling deploy |
| Detail UI | Hạn chót + Nguồn từ `resolved_due_at` / `resolved_due_source`; disclaimer preview |
| List UI | Secondary due; bỏ `T+N`; raw fallback nguyên văn |
| CMS labels | Tách raw vs engine |
| i18n / a11y | Source copy VI/EN |

---

## 12. Performance / cache / security

| Chủ đề | Yêu cầu |
|--------|---------|
| List | Giữ batch `ListPortalListCycleDues` — không N+1 |
| Detail | 1 cycle query (single type) + calculator đã có — chấp nhận |
| Cache | Key gồm `company_id` (+ type + slot nếu cache); cấm cache chỉ `type_id` |
| AuthZ | Subject company only |
| Leak | Không trả due Company khác |
| Persist | Preview không ghi DB |

---

## 13. Step-by-step implementation plan

### Step 1 — Verify current List/Detail due behavior

Trace `ListTypes` / `GetTypeDetail`; cycle reader; preview calculator; document mismatch (§4). **Done khi** evidence đủ (doc này).

### Step 2 — Chốt shared absolute due resolver

Reuse/extract helper: input `(companyID, typeItem/config, now)` → `(resolvedDueAt, source)`; cycle trước preview; không duplicate. **Done khi** List gọi batch wrapper, Detail gọi single — cùng precedence.

### Step 3 — Extend Detail contract

Additive `resolved_due_at`, `resolved_due_source` trên `DisclosureTypeDTO` / JSON; backward compatible; không API mới; không migration.

### Step 4 — Update Detail backend

`GetTypeDetail`: gọi resolver; subject company; không rewrite cycle; không đổi semantic `resolved_deadline_rule` logic.

### Step 5 — Update frontend types/normalizer

Confirm Detail maps fields; legacy response thiếu field vẫn render (null-safe).

### Step 6 — Update Detail UI

Semantic + due + source + preview disclaimer.

### Step 7 — Update List UI

Render `resolved_due_at`; bỏ `T+N`; raw nguyên văn.

### Step 8 — CMS labels

Raw vs engine; không Company preview Phase 1.

### Step 9 — Tests

List/Detail same company; cycle wins; preview fallback; source labels; legacy; cross-company; config change không rewrite cycle cũ.

### Step 10 — DEV smoke

Hai Company; có/không cycle; List↔Detail đối chiếu; no mismatch.

Sau cycle có sửa code: Docker/build verify theo ai-cache README (implement phase).

---

## 14. File impact matrix

### Backend

| File | Phân loại | Ghi chú |
|------|-----------|---------|
| `internal/disclosure/app/service.go` | **Cần sửa** | GetTypeDetail gọi absolute resolver |
| `internal/disclosure/app/list_resolved_due.go` | **Cần sửa** | Extract/share helper; List giữ batch |
| `internal/disclosure/app/resolved_deadline_rule.go` | Tái sử dụng / có thể | Không đổi semantic resolve; optional sync due_date |
| `internal/disclosure/app/contracts.go` | **Cần sửa** | Additive fields trên `DisclosureTypeDTO` |
| `infra/mysql/repository.go` `ListPortalListCycleDues` | Tái sử dụng | |
| `infra/inmemory/repository.go` | Có thể cần sửa | Stub trả cycle cho Detail tests |
| Detail HTTP handler | Không nên sửa | Serialize DTO |
| `deadline_calculator.go` / `deadlineengine_adapter.go` | Không nên sửa | |
| `applicability/rules.go` / `types.go` | Tái sử dụng | |
| tests `list_resolved_due_test.go` | **Cần sửa** | Shared helper + Detail parity |
| tests GetTypeDetail / cycle due | **Cần sửa** | Persisted wins trên Detail |

### Frontend

| File | Phân loại | Ghi chú |
|------|-----------|---------|
| `src/types.ts` | Tái sử dụng / có thể | Fields đã có trên `DisclosureType` |
| `src/services/normalizers.ts` | Tái sử dụng / verify | Detail qua summary normalizer |
| `src/pages/portal/DisclosureTypeDetail.tsx` | **Cần sửa** | Wire due/source |
| `.../DisclosureDeadlineSection.tsx` | **Cần sửa** | Hạn chót + Nguồn + disclaimer |
| `.../resolvedDeadlineRuleDisplay.ts` | **Cần sửa** | Copy/layout; absolute từ `resolved_due_at` |
| `src/services/deadlineDisplayHelpers.ts` | **Cần sửa** | List due; bỏ T+N |
| `src/pages/portal/DisclosureTypeList.tsx` | **Cần sửa** | Secondary due |
| CMS TemplateEditor / ApplicabilitySection / mappers | Có thể cần sửa | Labels only |
| DeadlineRuleCatalogScreen | Không nên sửa | |
| i18n + tests | **Cần sửa** | |

---

## 15. Test plan

### Backend

- Detail có persisted cycle due → `resolved_due_source` CYCLE_DUE/PLANNED_DATE; due khớp List.
- Detail không cycle → preview + `DEADLINE_SUMMARY_PREVIEW`.
- List/Detail cùng company cùng source + cùng `resolved_due_at`.
- Company isolation.
- Config đổi không rewrite cycle cũ; cycle mới dùng config mới.
- Irregular skip absolute due.
- Thiếu profile/config: semantic/due fallback đúng; không panic.

### Frontend

- Detail render persisted vs preview + source copy.
- List secondary due.
- Legacy thiếu field mới vẫn OK.
- Raw fallback nguyên văn; không `T+N`.
- Loading/error/empty; VI/EN; a11y.

### Integration / E2E

1. Tạo Template.  
2. Company A/B profile khác.  
3. List/Detail từng Company — semantic khác.  
4. Materialize cycle A.  
5. List/Detail A **cùng** persisted due + source.  
6. B không cycle → cả hai preview.  
7. Đổi config → cycle A giữ due; cycle mới theo config mới.

---

## 16. DEV smoke plan

| Case | Expect |
|------|--------|
| A có subsidiaries + có cycle | List/Detail cùng due; source CYCLE_DUE/PLANNED_DATE; Detail “Theo chu kỳ đã tạo” |
| B simple / subordinate, chưa cycle | List/Detail cùng preview due; source PREVIEW; “Ngày dự kiến…” |
| A vs B | Semantic/days khác; không lẫn company |
| Legacy raw | Hiện nguyên văn khi không resolve |
| Irregular | Không ép DueAt periodic |

---

## 17. Acceptance criteria (locked)

1. List và Detail cùng Company hiển thị **cùng semantic** deadline.  
2. Có persisted cycle due → List và Detail dùng **cùng ngày** đó.  
3. Không persisted → cả hai dùng BE preview theo policy.  
4. `resolved_due_source` phản ánh đúng nguồn.  
5. Preview gắn nhãn ngày dự kiến (không phải đã chốt).  
6. Không persisted → FE không suy đoán ngày.  
7. Không parse `T+N`.  
8. Company A không nhận due Company B.  
9. Cycle cũ không bị rewrite khi Template đổi (read path).  
10. Cycle mới dùng config mới.  
11. Raw `deadline_rule` chỉ fallback.  
12. API cũ hoạt động rolling deploy (field additive).  
13. Portal List không N+1.  
14. Không API mới / không migration nếu mở rộng DTO đủ.  
15. (giữ) CMS phân biệt raw vs engine; irregular không áp sai periodic; cache không lẫn company.

---

## 18. Out of scope (Future Phase)

- API `GET .../deadline-resolution`.  
- `deadline_rule_id` / catalog snapshot publish.  
- CMS preview Company thật.  
- Rewrite persisted due cycle cũ.  
- Đổi deadline engine.  
- Tính deadline ở FE.  
- Đồng bộ hàng loạt raw text cũ.  
- **Fallback chấp nhận List≠Detail absolute due** (không chọn Phase 1).

---

## 19. Open questions

| # | Câu hỏi | Recommendation mặc định |
|---|---------|-------------------------|
| 1 | Source badge trên List? | **Không** — chỉ Detail |
| 2 | Irregular semantic resolution riêng? | **Không** Phase 1 |
| 3 | CMS preview Company thật? | **Không** — Phase 2 |
| 4 | Chấp nhận List/Detail khác ngày nếu chưa đưa cycle vào Detail? | **Không chấp nhận** — bắt buộc mở rộng Detail contract trong API hiện tại |

Không còn blocker kiến trúc về absolute-due SoT.

---

## 20. Risks and mitigations

| Risk | Mitigation |
|------|------------|
| Duplicate List/Detail resolve logic | Extract shared helper (§13 Step 2) |
| FE vẫn ưu tiên `deadline_summary.deadline_date` | Detail UI authority = `resolved_due_at` khi có |
| Race List vs Detail giữa hai request | Accept narrow window; same code path |
| Rolling deploy BE chưa có field | FE null-safe; không crash |
| Gắn nhầm preview lên `resolved_deadline_rule.due_date` khi đã có cycle | Set/sync absolute từ cycle trước; test parity |
| Scope creep API mới | Out of scope; Method A only |

---

## Verdict

```text
READY FOR IMPLEMENTATION
```

Đã chốt: Detail dùng **cùng absolute-due source-of-truth** với List (persisted cycle → preview → không ngày), bằng cách mở rộng DTO Detail hiện tại + reuse cycle reader/helper — không API mới, không migration, không tính ở FE.

```text
Không tạo API deadline-resolution mới.
Không tạo migration nếu contract hiện tại đủ.
Không tính deadline ở frontend.
List và Detail phải dùng cùng absolute-due source-of-truth.
```

**Lượt này:** chỉ cập nhật tài liệu plan — chưa sửa source, chưa migration, chưa deploy, chưa commit.
