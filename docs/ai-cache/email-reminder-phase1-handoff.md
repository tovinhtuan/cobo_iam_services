# Email Reminder Content Upgrade — PHASE 1 Handoff (Database Foundation)

**Mode:** Controlled Implementation — PHASE 1
**Date:** 2026-06-19
**Scope boundary:** migration `instructions` column + DTO update + repository read/write path liên quan instructions.
**Explicitly NOT touched:** email template, reminder service logic, recipient name, urgency logic.

---

## 1. Scope Completed

Thêm nền tảng dữ liệu cho field "Hướng dẫn thực hiện" (`instructions`) của workflow step — nguồn cho `implementation_guide` ở các phase sau.

- [x] Migration 0099: thêm cột `instructions` vào `global_workflow_steps` (up + down).
- [x] DTO `GlobalWorkflowStepInput` (disclosure) — thêm field `Instructions` (JSON `instructions,omitempty`).
- [x] DTO `WorkflowStepConfig` (reminder) — thêm field `Instructions`.
- [x] Repository **write** path (CMS): `UpsertGlobalWorkflow` INSERT thêm cột `instructions`.
- [x] Repository **read** path (CMS): `listGlobalWorkflowSteps` SELECT + scan `instructions`.
- [x] Repository **read** path (reminder): `GetStepByID` SELECT + scan `instructions` → `WorkflowStepConfig.Instructions`.

**Round-trip wiring:** `GlobalWorkflowDTO.Steps` (output) và `CmsUpsertGlobalWorkflowRequest.Steps` (input) đều dùng `[]GlobalWorkflowStepInput`. Do đó JSON binding cho `instructions` đi qua HTTP CMS **tự động**, không cần đụng transport layer.

> Lưu ý: field `Instructions` đã có mặt trên `WorkflowStepConfig` và được populate bởi `GetStepByID`, **nhưng chưa được tiêu thụ** ở bất kỳ logic service nào (đúng ranh giới Phase 1). Việc đọc và đưa vào email payload là Phase 3.

---

## 2. Files Modified

| File | Thay đổi |
|------|----------|
| `internal/disclosure/app/contracts.go` | `GlobalWorkflowStepInput`: thêm `Instructions string \`json:"instructions,omitempty"\`` (sau `Stage`) |
| `internal/reminder/app/recipient_resolver.go` | `WorkflowStepConfig`: thêm `Instructions string` (sau `StageName`) |
| `internal/disclosure/infra/mysql/cms_repository.go` | `listGlobalWorkflowSteps`: SELECT + scan `instructions` (sql.NullString → `step.Instructions`); `UpsertGlobalWorkflow`: INSERT thêm cột `instructions` + giá trị `step.Instructions` |
| `internal/reminder/infra/mysql/recipient_query.go` | `GetStepByID`: SELECT + scan `instructions` (sql.NullString → `WorkflowStepConfig.Instructions`); cập nhật doc comment |

### Files Reverted (dọn rác ngoài scope)
Hai file template đã bị sửa ở **giai đoạn exploration trước khi vào Controlled Mode** (không thuộc Phase 1). Đã **revert về nguyên bản** để giữ Phase 1 sạch — template thuộc Phase 2:
- `internal/notification/templates/reminder.deadline_approaching/vi/subject.txt`
- `internal/notification/templates/reminder.deadline_approaching/vi/body.html`

---

## 3. Files Created

| File | Nội dung |
|------|----------|
| `migrations/0099_workflow_step_instructions.up.sql` | `ALTER TABLE global_workflow_steps ADD COLUMN instructions TEXT NULL AFTER stage;` |
| `migrations/0099_workflow_step_instructions.down.sql` | `ALTER TABLE global_workflow_steps DROP COLUMN instructions;` |

---

## 4. Migration

- **Số hiệu:** 0099 (kế tiếp 0098, không trùng).
- **Kiểu cột:** `TEXT NULL` — khớp convention `disclosure_type_versions.implementation_content` (0012); phù hợp text dài tự do.
- **Vị trí:** `AFTER stage`.
- **Tính chất:** additive, nullable, **không backfill**, không đụng `0001_init_core` (file cấm sửa).
- **Idempotency:** không idempotent (MySQL 8.0 không hỗ trợ `ADD COLUMN IF NOT EXISTS`) — đúng convention repo (ghi chú trong file).
- **Reversible:** có down migration (`DROP COLUMN`).
- **Trạng thái apply:** *Chưa apply lên DB nào* (Phase 1 chỉ tạo file; apply thực tế là Phase 6 — Dev deploy).

---

## 5. Build Result

```
go -C cobo_iam_services build ./...
BUILD_EXIT=0   ✅ PASS
```

---

## 6. Test Result

```
go -C cobo_iam_services test ./...
```

**Phase 1 touched packages — tất cả PASS:**
| Package | Kết quả |
|---------|---------|
| `internal/disclosure/infra/mysql` | ✅ ok |
| `internal/disclosure/app` (+ applicability, deadlineengine) | ✅ ok |
| `internal/reminder/app` | ✅ ok |
| `internal/reminder/infra/email` | ✅ ok |
| `internal/reminder/infra/inmemory` | ✅ ok |
| `internal/reminder/infra/observe` | ✅ ok |
| `migrations` | ✅ ok |

**Pre-existing failures — KHÔNG do Phase 1 gây ra, KHÔNG thuộc scope Phase 1:**

| Package | Test | Nguyên nhân | Bằng chứng độc lập |
|---------|------|-------------|--------------------|
| `internal/notification/app` | `TestContract_VariableParity/workflow.approved` | `workflow.approved/meta.yaml` khai báo biến `workflow_instance_id` (required:false) nhưng **không dùng** trong subject/body.html/body.txt → vi phạm parity | Phase 1 **không đụng** template `workflow.approved`, meta của nó, hay contract test. Template duy nhất Phase 1 từng chạm (`reminder.deadline_approaching`) đã được revert về nguyên bản. |
| `internal/platform/config` | `TestLoad_UserAvatarEnvOverride` | Path separator Windows: `storage dir = "\tmp\avatar-root"` (test kỳ vọng `/tmp/...`) | Lỗi môi trường Windows; Phase 1 **không đụng** `internal/platform/config`. |

> Cả hai lỗi pre-existing đều độc lập với feature workflow-step-instructions (migration + DTO + repository). Phase 1 không introduce regression nào.

---

## 7. Risks

| # | Risk | Mức độ | Ghi chú |
|---|------|--------|---------|
| P1-R1 | Migration 0099 chưa apply → code đọc cột `instructions` sẽ lỗi nếu chạy trên DB chưa migrate | 🟡 MEDIUM | Quy tắc deploy: **migration trước, code sau**. Apply thuộc Phase 6. |
| P1-R2 | Pre-existing fail `workflow.approved` parity làm `notification/app` đỏ | 🟡 MEDIUM | Ngoài scope. Cần xử lý ở một task riêng (đề xuất). Không chặn Phase 2 vì Phase 2 chỉ chạy `go test -run TestContract_` trên template do Phase 2 sửa — nhưng lưu ý gói `notification/app` vẫn sẽ đỏ vì test này. Xem mục Ready For Phase 2. |
| P1-R3 | Pre-existing fail config (Windows path) | ⚪ LOW | Môi trường; không ảnh hưởng logic feature. |
| P1-R4 | `instructions` insert chuỗi rỗng `""` thay vì NULL khi không nhập | ⚪ LOW | Chấp nhận được; read trả `""`, không vỡ logic. |

---

## 8. Rollback

Phase 1 hoàn toàn khả nghịch:

1. **Code:** revert 4 file Modified (`contracts.go`, `recipient_resolver.go`, `cms_repository.go`, `recipient_query.go`) về trạng thái trước Phase 1.
2. **Migration files:** xóa `0099_workflow_step_instructions.up.sql` và `.down.sql` (chưa apply nên không cần down trên DB).
3. **Nếu đã apply lên DB:** chạy `0099_workflow_step_instructions.down.sql` (`DROP COLUMN instructions`) — không mất dữ liệu khác.

Không có thay đổi phá vỡ; rollback an toàn, không data loss.

---

## 9. Ready For Phase 2

✅ **READY** — với 2 lưu ý:

1. **Sequencing:** Phase 2 (template) **không deploy lẻ** trước Phase 3 (payload) — xem PHASE 0 R1. Trong mô hình phased, deploy gộp ở Phase 6 → an toàn.
2. **Pre-existing `notification/app` đỏ:** Khi Phase 2 chạy `go test -run TestContract_ ./...`, package `notification/app` vẫn sẽ FAIL do `workflow.approved` parity (pre-existing, không liên quan template Phase 2). Cần thống nhất trước: **(a)** chấp nhận và chỉ verify subtests của 2 template mục tiêu PASS, hoặc **(b)** cho phép sửa nhỏ `workflow.approved/meta.yaml` (gỡ biến `workflow_instance_id` thừa) như một fix vệ sinh đi kèm — **cần user quyết** vì nằm ngoài scope template gốc.

**Open clarifications kế thừa từ PHASE 0 (chưa chốt):**
- **CL-1 (Phase 3):** `implementation_guide` required + fallback non-empty.
- **CL-2 (Phase 4):** cách lấy `recipient_name` không tạo N+1.

---

## 10. Trạng thái

PHASE 1 hoàn tất. Build PASS. Test: Phase 1 packages PASS; chỉ còn 2 lỗi pre-existing độc lập (đã chứng minh không do Phase 1).

⏸️ **DỪNG — chờ `CONFIRM PHASE 2`.**

Không tự động chuyển phase.
