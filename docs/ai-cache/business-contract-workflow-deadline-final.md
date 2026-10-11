# Business Contract (Final) — Workflow, Periodic, Ad-hoc & Deadline Alerts

**Phiên bản:** 1.2  
**Ngày hiệu lực:** 2026-05-27 (v1.1); **v1.2: 2026-10-11**  
**Cập nhật PO:** 2026-05-27 — §2.1 quyết định triển khai P0 (locked); **2026-10-11 — §2.2 Amendment v1.2 (PO đồng ý toàn bộ khuyến nghị trong `bug-worker-alert-fixes-2026-10-10/05-po-ba-decision-recommendations.md`)**  
**Trạng thái:** Canonical — thay thế mô tả mâu thuẫn trong contract cũ (periodic “workflow disabled”, Done = Published)  
**Phạm vi repo:** `cobo_iam_services` (BE), `cobo_web_design` (FE)  
**Đối tượng:** Dev, BA, PO, QA

**Triển khai:** [workflow-deadline-e2e-implementation-plan.md](./workflow-deadline-e2e-implementation-plan.md)  
**Kế hoạch kỹ thuật chi tiết:** [deadline-alert-workflow-overview-implementation-plan.md](./deadline-alert-workflow-overview-implementation-plan.md)

---

## 1. Mục đích & phạm vi

Hợp đồng này định nghĩa **business logic bắt buộc** cho:

1. **Tạo cảnh báo / disclosure record** theo 3 loại template (`periodic`, `custom`, `irregular`).
2. **Khởi tạo workflow instance** kèm **snapshot** các bước xử lý.
3. **Hiển thị & vận hành** trên tab **Cảnh báo về thời hạn** (deadline alerts).
4. **Xác nhận hoàn thành** cảnh báo (DC-B) sau khi record đạt trạng thái terminal.

**Ngoài phạm vi (P2+):** CMS maker-checker workflow override UI đầy đủ; `POST /deadline-alerts/complete`; email thông báo periodic auto-create.

---

## 2. Quyết định PO (canonical)

| ID | Quyết định | Mô tả ngắn |
|----|------------|------------|
| **OQ-WF-01** | **WF-A** | Materialize periodic/custom **luôn** tạo `workflow_instance` + `snapshot_json` ≥ 1 bước |
| **OQ-WF-02** | **WF2-A** | Có `workflow_instance_id` mà snapshot rỗng → **lỗi tích hợp** (retry), không UX “degraded chuẩn” |
| **OQ-WF-03** | **WF3-A + WF3-C** | `current_step` = bước đầu snapshot (`display_order` min); UI không `completed` giả |
| **OQ-WF-04** | **WF4-A** | `WORKFLOW_SNAPSHOT_ENABLED=true` mọi môi trường nghiệm thu |
| **OQ-WF-05** | **WF5-A / WF5-B** | periodic/custom = effective workflow; irregular = effective + `step_overrides` |
| **OQ-DA-01** | **DC-B** | Record terminal → `PENDING_CONFIRM`; `DONE` chỉ sau `POST .../confirm` |
| **OQ-DA-03** | **HC edge** | Degraded UI **chỉ** khi **không** có `workflow_instance_id` (materialize lỗi / record cũ) |
| **AL-8** | **AL-B** | List deadline vẫn hiện record chưa workflow nếu có ngày hạn theo dõi |

Tài liệu PO 2026-05-26 **thay thế** các mục xung đột trong `deadline-alert-detail-po-decisions-summary.md` (2025-05-25).

### 2.1 Quyết định triển khai P0 (locked — 2026-05-27)

| Chủ đề | Quyết định | Ghi chú dev |
|--------|------------|-------------|
| **Preference RBAC** | Permission mới `disclosure.auto_create.manage` cho **PATCH**; **GET** = `disclosure.view` | Seed: gán cho `admin_doanh_nghiep`; **không** dùng `rbac.manage` |
| **Exactly-once materialize** | **Optimistic claim** trên `periodic_cycles` trước khi tạo record | Không transaction dài qua disclosure + workflow service |
| **`cycle_start`** | **Cột `cycle_start DATE`** ghi lúc seed; materialize đọc làm T0 | Không derive từ `cycle_label` tại materialize |
| **Partial workflow fail (periodic)** | **Fail materialize** — cycle **không** gắn `record_id` nếu thiếu `workflow_instance_id` + snapshot hợp lệ (WF-A) | Orphan record hiếm = edge OQ-DA-03; P1 metric/cleanup |
| **Dữ liệu cũ không workflow** | **Không backfill P0** | UX degraded + `pending_init`; backfill P1 theo batch BA |
| **Effective workflow rỗng** | **Chặn** — ad-hoc **4xx**; worker skip cycle + `error_count` (không tạo record) | Không record “thành công” không workflow khi WF-A bật |

### 2.2 Amendment v1.2 (2026-10-11): thay thế các mục mâu thuẫn bên dưới

**Định nghĩa chung.** Đây là nguồn sự thật duy nhất cho tab Cảnh báo, dashboard, "Việc của tôi" và reminder.

| Khái niệm | Định nghĩa |
|---|---|
| `ObligationOpen(record)` | Periodic / custom / thủ công: `status = Draft AND submitted_at IS NULL`.<br>Irregular (ad-hoc): proposal `approved` AND record chưa ở trạng thái `Approved / Published / Completed / Done` (tức từ lúc duyệt tới khi workflow công bố hoàn tất), bất kể record đã auto-submit lúc duyệt. |
| `CompletedAt(record)` | Lấy theo thứ tự ưu tiên:<br>1. `confirmed_at` (xác nhận đã công bố, kể cả ngoài hệ thống);<br>2. periodic/thủ công: `submitted_at`;<br>3. ad-hoc: thời điểm record đạt `Approved / Published / Completed`.<br>So sánh theo ngày `Asia/Ho_Chi_Minh` với hạn. |
| Hạn (due) | Tính theo **đúng loại ngày template khai báo**: `applicability_rules.deadline_day_type`, nếu trống thì `deadline_config.deadline_duration_type`.<br>Ngày cụ thể (`FIXED_DATE`) là ngày bắt buộc; chỉ dời khi template chọn `MOVE_TO_NEXT_WORKING_DAY`.<br>**Không tự dời** hạn ngày lịch rơi vào ngày nghỉ.<br>Template có `deadline_days > 0` phải khai báo loại ngày khi publish; nếu thiếu thì fallback là ngày lịch. |
| "Hôm nay" | Ngày theo `Asia/Ho_Chi_Minh` (không lấy ngày UTC). |

**Thay đổi so với v1.1**

| Mục | v1.1 | v1.2 |
|---|---|---|
| §5.1 / AC-4 | Terminal → `PENDING_CONFIRM`; confirm → `DONE` | **Bỏ `PENDING_CONFIRM`.** Record hết `ObligationOpen` thì rời tab Cảnh báo (xem ở lịch sử). **Confirm** dùng khi nghĩa vụ đã thực hiện **ngoài hệ thống** trong lúc record còn mở → `DONE`. Confirm lần hai giữ người xác nhận đầu tiên (trả 200). |
| §5.2 | List theo record có due | List = `ObligationOpen` + có due + đã tới `OpenAt` (periodic). Ad-hoc hiện **ngay khi được duyệt**. Một record không tính được hạn không được làm lỗi cả danh sách. |
| §3.2 | 2 vòng focal → admin | Một vòng, nhiều reviewer, **cần tất cả đồng ý** (v3).<br>Ngày chốt (`final_t0_date`, `final_deadline_date`, ghi chú): **giá trị được nhập sau cùng thắng**; ô để trống **không** xoá giá trị người trước đã nhập; modal duyệt hiển thị giá trị hiện hành.<br>Hạn hiển thị dùng `final_t0_date` khi có.<br>Khi đã đủ phiếu thì **không** cho Reject/Cancel (409); finalize lỗi giữa chừng phải bù trừ.<br>Submit proposal kiểm tra lại reviewer (active, có quyền) và quyền tạo hồ sơ của người đề xuất. |
| §7 Reminder | Milestone → occurrence → email | Nhắc theo **bước hiện tại của workflow** (không phụ thuộc task), đúng người (assignee → trưởng phòng → nhân viên phòng → admin).<br>Gửi trong khung **07:00–21:00 VN mọi ngày**.<br>Grace 2 ngày sau hạn bước; bản nhắc trễ quá 48h thì hết hạn (không gửi).<br>Khi trễ chỉ gửi **một** thông báo "Đã quá hạn N ngày" cho mỗi bước.<br>Bản bị skip không chặn hàng đợi.<br>Bước workflow nhắc theo offset của template; lịch gửi của company chỉ áp cho nhắc hạn nộp.<br>Thiếu cấu hình kênh thì dùng mặc định (email + in-app). |
| Company status | — | `inactive`: dừng nhắc, dừng tạo kỳ và cảnh báo.<br>`suspended`: vẫn tạo kỳ và vẫn nhắc (nghĩa vụ pháp lý không tạm ngưng). |
| Workflow submit thủ công | Theo cấu hình | T0 = ngày submit (HCM), `t0_policy=system_date`. Tổng thời gian xử lý vượt hạn thì cảnh báo, không tự nén. |
| Dashboard KPI | `06-approved-kpi-formulas.md` (2026-08-03) | KPI trạng thái mở (#1, #3, #5) là **ảnh chụp hiện tại, không theo range**; #2 dùng cửa sổ 7 ngày; range chỉ áp cho KPI hoàn thành (#4, #6) theo `CompletedAt`. Màu tỉ lệ đúng hạn: ≥95% success, 85–95% warning, <85% critical, mẫu 0 thì neutral. |
| Ngày nghỉ | File cấu hình | File chỉ chứa ngày chắc chắn; ngày phụ thuộc thông báo hằng năm nhập qua CMS. BA cập nhật lịch năm sau vào **tháng 10** hằng năm. |

**Acceptance criteria bổ sung**

| # | Tiêu chí |
|---|---|
| AC-11 | Ad-hoc được duyệt xuất hiện ngay trên tab Cảnh báo với hạn đúng; biến mất khi workflow hoàn tất |
| AC-12 | Bước 2..N của workflow periodic/custom được nhắc khi tới lượt; bước đã hoàn tất không bị nhắc |
| AC-13 | "Việc của tôi" hiển thị task của record periodic (Draft) |
| AC-14 | Cùng template/kỳ: preview CMS, worker và tab Cảnh báo ra **cùng** ngày hạn |
| AC-15 | Không có email nhắc gửi ngoài 07:00–21:00 VN; không gửi lặp cùng occurrence |

Thay thế: §3.2 (state machine), §5.1, §5.2, AC-4, §7 ở dưới chỉ còn hiệu lực ở phần **không** mâu thuẫn với §2.2.

---

## 3. Ba loại template & nguồn record

| `template_category` | Tên nghiệp vụ | Cách tạo record | Workflow snapshot |
|---------------------|---------------|-----------------|-------------------|
| `periodic` | Định kỳ | Worker: seed `periodic_cycles` → materialize | WF5-A (effective) |
| `custom` | Tần suất tùy chỉnh | Cùng worker (frequency monthly/quarterly/yearly) | WF5-A |
| `irregular` | Bất thường | Ad-hoc proposal → focal → process controller approve | WF5-B (effective + overrides) |

### 3.1 Periodic / custom (WF-A)

- **Trigger:** Worker tick (`PERIODIC_SEEDING_ENABLED=true`).
- **Phase 1 — Seed:** Với mỗi `(company, type)` active + `auto_create_enabled` → upsert `periodic_cycles` (idempotent theo `cycle_label`); persist **`cycle_start`** (cột DB).
- **Phase 2 — Materialize:** Optimistic **claim** cycle → `CreateAndSubmitRecord` (record + workflow + snapshot) → gán `record_id` **chỉ khi** WF-A thành công (§2.1).
- **T0:** `t0_date` = **`cycle_start`** từ DB, không phải `now()` tại materialize.
- **Lỗi workflow / snapshot rỗng:** materialize thất bại; cycle retry; không coi là happy path.
- **Opt-out:** `PATCH .../disclosure-types/{type_id}/preferences` (`auto_create_enabled`, permission §6.3).

### 3.2 Irregular (ad-hoc)

- Chỉ `template_category === 'irregular'`.
- State machine: `ad_hoc_draft` → `pending_focal_approval` → `pending_admin_approval` → `approved` | `rejected` | `cancelled`.
- **Admin approve** (process controller): tạo record + submit + workflow (idempotent qua `ReserveAdminApproval`).
- Overrides: chỉ `step_id` + `processing_days` trong `proposed_workflow_json`; snapshot đầy đủ = effective + patch.
- **Effective workflow rỗng:** từ chối approve (**4xx**); không tạo record (§2.1).

### 3.3 Edge — record không workflow (OQ-DA-03 HC)

- Record tồn tại, **không** `workflow_instance_id` (materialize partial fail, dữ liệu cũ).
- Tab deadline: **vẫn list** nếu có due date (AL-B).
- Chi tiết: banner degraded + **một** bước ảo «Chờ khởi tạo quy trình» (`pending_init`).

---

## 4. Workflow instance & snapshot

### 4.1 Khi nào tạo

| Luồng | Bắt buộc workflow? |
|-------|---------------------|
| Periodic/custom materialize | **Có** (WF-A) |
| Ad-hoc admin approve | **Có** khi `WORKFLOW_ADHOC_ENABLED` + snapshot enabled |
| Manual disclosure form (không qua ad-hoc) | Theo cấu hình hiện tại (ngoài phạm vi contract này) |

### 4.2 Snapshot (`snapshot_json`)

Mảng `StepSnapshot` (xem `internal/workflow/app/contracts.go`):

| Field | Bắt buộc | Ghi chú |
|-------|----------|---------|
| `step_id` | Có | Khóa merge override |
| `step_code` | Khuyến nghị | Dùng cho task / UI |
| `stage`, `department` | Có | Hiển thị overview |
| `due_rule` | Có | VD `T+N` |
| `display_order` | Có | Thứ tự bước |
| `processing_days` | Có | Có thể bị override ad-hoc |

**Nguồn:** `GET effective-workflow` logic nội bộ (`GetEffectiveWorkflow`) → map snapshot → `workflow_source` (`global_template` | `company_override`).

### 4.3 Trạng thái instance lúc tạo (WF3-A)

- `status` = `in_progress`
- `current_step_code` = bước có `display_order` nhỏ nhất
- Task đầu: cùng `step_code`, `status` = `pending`
- **Không** hardcode `review` nếu snapshot bước đầu khác

### 4.4 WF2-A — snapshot rỗng có instance id

- BE: `ValidateSnapshot` fail khi flag bật và snapshot rỗng.
- FE: error block + retry load instance; **không** coi là `workflowDegraded` chuẩn.

### 4.5 WF4-A — feature flag

- `WORKFLOW_SNAPSHOT_ENABLED=true` trên dev/staging/prod nghiệm thu.
- Tắt flag → không đạt acceptance contract.

---

## 5. Deadline alerts (tab Cảnh báo)

### 5.1 Trạng thái hiển thị (DC-B)

| Status API | Điều kiện |
|------------|-----------|
| `UPCOMING` / `DUE_SOON` / `OVERDUE` | Record chưa terminal, có due date |
| `PENDING_CONFIRM` | Record terminal (`published` / `completed` / tương đương) **chưa** confirm |
| `DONE` | Có row `deadline_alert_confirmations` sau `POST /company/deadline-alerts/{record_id}/confirm` |

**Không** map `Published` → `DONE` trực tiếp trên FE (bỏ fallback cũ).

### 5.2 List (AL-B)

- `LEFT JOIN workflow_instances` — **không** bắt buộc có instance để vào list.
- Vẫn loại record không có due date (logic `resolveDueDateAndStatus`).

### 5.3 Confirm

- Permission: `deadline.manage` / `deadline.confirm`
- `POST /api/v1/company/deadline-alerts/{record_id}/confirm`

### 5.4 Chi tiết cảnh báo — UX

| Tình huống | `workflowDegraded` | Steps UI |
|------------|-------------------|----------|
| Không `workflow_instance_id` | `true` | 1 card `pending_init` |
| Có instance, snapshot OK | `false` | N cards từ snapshot |
| Có instance, snapshot rỗng | `false` (WF2-A) | Error + retry, không bước ảo |

---

## 6. API contract (runtime)

### 6.1 Portal — deadline detail

| # | Method | Path | Permission |
|---|--------|------|------------|
| 1 | GET | `/api/v1/disclosures/{record_id}` | `disclosure.view` |
| 2 | GET | `/api/v1/company/deadline-alerts` | `deadline.view` |
| 3 | GET | `/api/v1/workflows/instances/{id}` | `workflow.read` |
| 4 | GET | `/api/v1/workflows/instances/{id}/tasks` | `workflow.read` |
| 5 | POST | `/api/v1/company/deadline-alerts/{record_id}/confirm` | `deadline.manage` |

### 6.2 Ad-hoc

| Method | Path |
|--------|------|
| POST | `/api/v1/company/ad-hoc-proposals` |
| POST | `/api/v1/company/ad-hoc-proposals/{id}/submit` |
| POST | `/api/v1/company/ad-hoc-proposals/{id}/admin-approve` |

### 6.3 Periodic preferences

| Method | Path | Permission |
|--------|------|------------|
| GET | `/api/v1/company/disclosure-types/{type_id}/preferences` | `disclosure.view` |
| PATCH | `/api/v1/company/disclosure-types/{type_id}/preferences` | `disclosure.auto_create.manage` |

Seed P0: gán `disclosure.auto_create.manage` cho role `admin_doanh_nghiep` (§2.1).

---

## 7. Reminder (tách biệt)

- Module `reminder`: milestone → occurrences → email (`WORKFLOW_REMINDERS_ENABLED`).
- Phụ thuộc workflow instance / timeline khi `WORKFLOW_TIMELINE_ENABLED`.
- **Không** thay thế deadline confirm (DC-B).

---

## 8. Acceptance criteria (nghiệm thu)

| # | Tiêu chí |
|---|----------|
| AC-1 | Sau worker tick, periodic record có `workflow_instance_id` + `len(snapshot) ≥ 1` |
| AC-2 | Ad-hoc approve: snapshot = effective steps; override chỉ `processing_days` |
| AC-3 | `GET workflow instance` trả đủ `snapshot[]` từ DB |
| AC-4 | Terminal record → list `PENDING_CONFIRM`; confirm → `DONE` |
| AC-5 | FE: không `workflowDegraded` khi có instance + steps; WF2-A error khi snapshot rỗng |
| AC-6 | FE: `!hasWorkflow` → 1 bước `pending_init` |
| AC-7 | Materialize không tạo duplicate record (optimistic claim — §2.1) |
| AC-8 | `WORKFLOW_SNAPSHOT_ENABLED=true` trên env nghiệm thu |
| AC-9 | `t0_date` workflow periodic = `cycle_start` seed, không phải thời điểm worker tick |
| AC-10 | Template effective workflow rỗng → không materialize / không approve (4xx) |

---

## 9. Tài liệu tham chiếu & supersede

| Tài liệu | Quan hệ |
|----------|---------|
| [business-contract-periodic-auto-creation.md](../../cobo_web_design/docs/ai-cache/skill-outputs/business-contract-periodic-auto-creation.md) | **Superseded** — periodic phần workflow/disabled |
| [business-contract-adhoc-alert-create.md](../../cobo_web_design/docs/ai-cache/skill-outputs/business-contract-adhoc-alert-create.md) | Bổ sung — ad-hoc SM; snapshot theo contract này |
| [periodic-vs-adhoc-sequence-quick-review.md](./periodic-vs-adhoc-sequence-quick-review.md) | Sequence runtime |
| [deadline-alert-workflow-overview-implementation-plan.md](./deadline-alert-workflow-overview-implementation-plan.md) | Ticket BE/FE chi tiết |

---

## 10. Checklist PO / QA (P0)

- [ ] WF-A worker bật workflow + snapshot
- [ ] DC-B confirm trên tab deadline
- [ ] WF2-A không hiển thị overview “degraded” khi chỉ thiếu snapshot
- [ ] OQ-DA-03 HC: banner + `pending_init` khi không instance
- [ ] AL-B: record không workflow vẫn list nếu có hạn
- [ ] Preference API: GET `disclosure.view`, PATCH `disclosure.auto_create.manage` + FE load GET
- [ ] `cycle_start` persisted; materialize claim + WF-A success gate
- [ ] Empty effective workflow blocked (4xx ad-hoc; worker skip)
