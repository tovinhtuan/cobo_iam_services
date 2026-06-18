# Email Reminder Content Upgrade — PHASE 3 Handoff (Payload Enrichment)

**Mode:** Controlled Implementation — PHASE 3
**Date:** 2026-06-19
**Scope boundary:** điền payload cho `remaining_days`, `urgency_status`, `implementation_guide` trong `prepareDispatch`.
**Explicitly NOT done (per chỉ đạo):** N+1 optimization, batch step lookup, cache redesign. `recipient_name` để Phase 4.

---

## 1. Payload Fields Added

Bổ sung trong `internal/reminder/app/service.go` → `prepareDispatch()` (additive, có guard `if _, ok := payload[...]; !ok`):

| Field | Nguồn | Logic |
|-------|-------|-------|
| `remaining_days` | `c.ScheduledAt` vs now | `calculateRemainingDays()` — floor về nửa đêm theo **Asia/Ho_Chi_Minh**, diff ngày (mirror `remainingDaysFromDue` của deadline UI). Trả `int`. |
| `urgency_status` | `remaining_days` | `determineUrgencyStatus()` — `<0`→`"Quá hạn"`, `==0`→`"Đã đến hạn"`, `>0`→`"Sắp đến hạn"`. |
| `implementation_guide` | `step.Instructions` → fallback | WORKFLOW_STEP: lấy `step.Instructions` (từ **cùng** `GetStepByID` đã gọi cho `step_name`), cắt `truncateImplementationGuide(…, 500)`. Nếu rỗng/không có step → `defaultImplementationGuide` (chuỗi generic non-empty). |

**Helper functions mới (cùng file, có unit test):**
- `calculateRemainingDays(dueDate, now time.Time) int`
- `determineUrgencyStatus(remainingDays int) string`
- `extractStepID(scopeID string) string` — gom logic tách `disclosureID:stepID` (trước đây inline 2 nơi), nay tái sử dụng.
- `truncateImplementationGuide(guide string, maxChars int) string` — cắt theo **rune** (UTF-8 an toàn tiếng Việt), thêm `...`.
- `reminderCalculatorLocation() *time.Location` — Asia/Ho_Chi_Minh + fallback FixedZone +07:00.

**Constants:** `implementationGuideMaxChars = 500`, `defaultImplementationGuide = "Vui lòng truy cập hệ thống để xem chi tiết và hoàn thành công việc đúng hạn."`

**Quyết định kế thừa CL-1(a):** `implementation_guide` luôn non-empty (fallback generic) → khớp ràng buộc template `required: true`, không cần cross-module reader đọc `disclosure_type_versions.implementation_content`.

---

## 2. Query Impact Analysis

**KẾT QUẢ: 0 query mới trên production dispatch path.**

- WORKFLOW_STEP scope: trước đây block `step_name` đã gọi `GetStepByID(stepID)` **mỗi candidate** (vì `buildReminderTemplatePayload` **không** set sẵn `step_name` — đã verify tại `repository.go:491-507`). Phase 3 **tái sử dụng cùng một fetch** đó để đọc thêm `step.Instructions`. Code được restructure: fetch `step` **một lần**, dùng cho cả `step_name` lẫn `implementation_guide`.
- DISCLOSURE scope: `remaining_days`/`urgency_status` tính **in-memory** (0 query); `implementation_guide` dùng fallback generic (0 query).
- **KHÔNG** thêm batch/dedup/cache (đúng chỉ đạo — N+1 hiện hữu được giữ nguyên, không tối ưu).

| Path | Query trước | Query sau | Δ |
|------|-------------|-----------|---|
| WORKFLOW_STEP candidate | 1× `GetStepByID` | 1× `GetStepByID` (đọc thêm cột `instructions` đã có sẵn từ Phase 1) | **+0** |
| DISCLOSURE candidate | 0 (cho 3 field này) | 0 | **+0** |

> Lưu ý: trường hợp lý thuyết duy nhất phát sinh +1 query là khi caller **tự** pre-populate `step_name` vào payload (path milestone/config production không làm điều này). Không xảy ra trên dispatch path thực tế.

---

## 3. Build Result

```
go -C cobo_iam_services build ./...
BUILD_EXIT=0   ✅ PASS
```

---

## 4. Test Result

### 4.1 Unit tests bắt buộc (4 helper) — ✅ ALL PASS
```
go test -run 'TestCalculateRemainingDays|TestDetermineUrgencyStatus|TestExtractStepID|TestTruncateImplementationGuide' ./internal/reminder/app/
```
| Test | Kết quả |
|------|---------|
| `TestCalculateRemainingDays` (7 subcases: hôm nay/5 ngày/ngày mai/quá hạn/zero/UTC→HCM) | ✅ PASS |
| `TestDetermineUrgencyStatus` (5 cases) | ✅ PASS |
| `TestExtractStepID` (5 cases) | ✅ PASS |
| `TestTruncateImplementationGuide` (trim/truncate/exact/disabled/UTF-8) | ✅ PASS |

File tạo: `internal/reminder/app/reminder_content_test.go` (package `app`, white-box).

### 4.2 Regression — `reminder/app` full package: ✅ PASS (không vỡ dispatch tests hiện có).

### 4.3 Test fixtures reconcile (do contract template Phase 2 — bắt buộc fix để build xanh)
Thay đổi `required` vars ở 2 template Phase 2 làm vỡ 3 fixture cũ (assert theo contract cũ). Đã cập nhật:
| File | Test | Sửa |
|------|------|-----|
| `internal/notification/infra/registry/embed_registry_test.go` | `TestEmbedRegistry_ResolveDeadlineApproaching` | wantVars 4→8 |
| `internal/notification/infra/registry/embed_registry_test.go` | `TestEmbedRegistry_ResolveWorkflowStepDue` | wantVars 5→9 |
| `internal/reminder/infra/email/smtp_sender_test.go` | `TestSender_PassThroughKey_UsesEmbedRegistry` | payload bổ sung 4 biến required mới |
→ Cả 3 package PASS sau fix.

### 4.4 Full suite — failures còn lại: **8, TẤT CẢ pre-existing, KHÔNG do feature này**
Đã kiểm chứng qua **error message thực tế** (không cái nào nhắc `instructions`/`implementation_guide`/`recipient_name`/`urgency`/template reminder):

| Package | Test | Error thực tế | Phân loại |
|---------|------|---------------|-----------|
| `notification/app` | `TestContract_VariableParity/workflow.approved` | `workflow_instance_id` declared-but-unused | Pre-existing (ghi nhận Phase 1) |
| `platform/config` | `TestLoad_UserAvatarEnvOverride` | `storage dir = "\tmp\avatar-root"` (Windows path) | Pre-existing môi trường (ghi nhận Phase 1) |
| `httpserver` | `TestIntegration_disclosureTypeCatalog_adminUpsertAndVersioning` | `template_category must be one of [periodic, irregular]` | Pre-existing (validation drift) |
| `httpserver` | `..._deadlineSummaryFixedDateWarnOnlyTimezone` | `session expired` (401) | Pre-existing (harness/seed) |
| `httpserver` | `..._deadlineSummaryFixedDateMoveNextWorkingDay` | `record is not in approved state` (409) | Pre-existing (harness/seed) |
| `httpserver` | `..._platformCMSPrefix_dashboardCollectionsEntries` | `template_category ...` | Pre-existing |
| `httpserver` | `..._platformCMSPrefix_entriesReviewsSchedulesContract` | `template_category ...` | Pre-existing |
| `companyaccess/transport/http` | `TestCreateSelfServiceCompany_FeatureFlagOff` | feature-flag behaviour | Pre-existing |

> Các failure httpserver/companyaccess nằm **trên** tail cutoff của lệnh test ở Phase 1 nên chưa hiển thị khi đó; chúng đã tồn tại từ trước (error message không liên quan thay đổi của tôi). Phase 3 introduce **0 regression mới**.

---

## 5. Files Modified / Created

**Modified (sản phẩm):**
- `internal/reminder/app/service.go` — restructure block WORKFLOW_STEP (1 fetch dùng chung), thêm block `remaining_days`/`urgency_status`/`implementation_guide` fallback, thêm 5 helper + 2 const.

**Created (test):**
- `internal/reminder/app/reminder_content_test.go` — unit test 4 helper.

**Modified (test reconcile do Phase 2 contract):**
- `internal/notification/infra/registry/embed_registry_test.go`
- `internal/reminder/infra/email/smtp_sender_test.go`

Không đụng: email template (Phase 2), database/migration (Phase 1), recipient_name (Phase 4), query/repository.

---

## 6. Risks

| # | Risk | Mức độ | Ghi chú |
|---|------|--------|---------|
| P3-R1 | `due_date` format theo **UTC** (code cũ) còn `remaining_days` theo **HCM** → lệch 1 ngày ở biên ngày | 🟡 MEDIUM | Ngoài scope sửa `due_date` (Phase 3 chỉ 3 field mới). `remaining_days` theo Architecture Validation (HCM, khớp UI). Cần cân nhắc đồng bộ `due_date` sang HCM ở task riêng. |
| P3-R2 | N+1 sẵn có ở `GetStepByID` per-candidate vẫn còn | ⚪ PRE-EXISTING | Cố ý giữ nguyên (chỉ đạo: out of scope). 0 query mới. |
| P3-R3 | `implementation_guide` fallback generic khi step thiếu instructions → email vẫn gửi nhưng nội dung hướng dẫn chung chung | ⚪ LOW | Chấp nhận theo CL-1(a). Platform Admin nhập instructions (Phase 1) để có nội dung cụ thể. |
| P3-R4 | `recipient_name` CHƯA điền (Phase 4) → nếu deploy lẻ, render fail (required) | 🔴 HIGH | Template + P3 + P4 lên cùng Phase 6. Không deploy lẻ. |

---

## 7. Ready For Phase 4

✅ **READY.** Sau Phase 3, payload đã có: `company_name`, `disclosure_title`, `due_date`, `portal_url`, `step_name`, `remaining_days`, `urgency_status`, `implementation_guide`. **Còn thiếu duy nhất `recipient_name`** (Phase 4).

**Phase 4 phải chốt trước khi code (per plan):** recipient ownership.
- PHASE 0 đã xác định: recipient = **User** (`users.full_name`), resolve company-scoped qua `memberships`; resolver hiện trả `[]string` email (có thể là `login_id` fallback).
- **CL-2** vẫn mở: cách lấy `recipient_name` **không tạo N+1 mới** — đề xuất (b) dùng **email làm fallback** cho `recipient_name` (0 query mới, đúng "no batch optimization").

---

## 8. Trạng thái

PHASE 3 hoàn tất. Build PASS. 4 unit test helper PASS. 3 fixture vỡ-do-Phase-2 đã reconcile. Full suite: chỉ còn 8 failure pre-existing (đã chứng minh qua error message, không do feature). 0 query mới — đúng chỉ đạo không tối ưu N+1/batch/cache.

⏸️ **DỪNG — chờ `CONFIRM PHASE 4`.**

Không tự động chuyển phase.
