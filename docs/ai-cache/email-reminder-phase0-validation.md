# Email Reminder Content Upgrade — PHASE 0 Pre-Implementation Validation

**Mode:** Controlled Implementation — PHASE 0 (no code)
**Date:** 2026-06-19
**Validated by:** Principal Engineer / Architect review of actual source
**Source of truth precedence:** Architecture Validation > Implementation Plan > Current Code

> Mục tiêu PHASE 0: xác nhận 5 điểm trước khi code — (1) recipient_name ownership, (2) workflow step instructions source, (3) implementation_guide fallback strategy, (4) migration impact, (5) backward compatibility impact. Mọi kết luận dưới đây dựa trên đọc code thực tế, không assume.

---

## A. Architecture Findings (Evidence-Based)

### Finding 1 — recipient_name ownership

**Câu hỏi:** Email recipient thuộc User / Membership / Company Membership? Source of truth cho tên là gì?

**Bằng chứng code:**
- Mọi path resolve recipient trong `internal/reminder/infra/mysql/recipient_query.go` trả về cùng một biểu thức:
  `SELECT DISTINCT COALESCE(NULLIF(TRIM(u.email), ''), u.login_id)` — join `memberships m → users u`, scope `m.company_id = ?`.
  - `EmailsByDepartments` (recipient_query.go:34-44)
  - `EmailsByRoles` (recipient_query.go:84-93)
  - `AdminEmailsByCompany` (recipient_query.go:109-121)
  - `AssigneeEmailsByStep` (recipient_query.go:134-146)
- `RecipientResolver` interface trả về **`[]string` (chỉ email)** — không mang theo `user_id` hay tên (`contracts.go:154-157`).
- Schema `users` (migration `0001_init_core.up.sql:3-9`) có **`full_name VARCHAR(255) NOT NULL`**, `email VARCHAR(255) NULL`, `login_id VARCHAR(191) NOT NULL`.

**Kết luận ownership:**
- Recipient = **User** (bảng `users`), được resolve company-scoped qua `memberships`. "Membership" chỉ là cầu nối tenant; định danh người nhận thật là `users.user_id`.
- **Source of truth cho tên = `users.full_name`** (luôn NOT NULL → luôn có giá trị).
- ⚠️ Điểm tinh tế: chuỗi "email" trả về có thể là `users.email` **HOẶC** `users.login_id` (fallback khi email rỗng). Vì vậy reverse-lookup `email → full_name` bằng chính chuỗi email là **không an toàn 100%** (user không có email sẽ map bằng login_id). Đây là input cho quyết định Phase 4.

---

### Finding 2 — workflow step instructions source

**Câu hỏi:** "Hướng dẫn thực hiện" của mỗi step workflow lấy từ đâu?

**Bằng chứng code:**
- Bảng `global_workflow_steps` (migration `0059_global_workflows.up.sql:19-33`, đối chiếu drift-fix `0091_fix_global_workflow_schema.up.sql:33-39`) gồm các cột:
  `step_id, workflow_id, stage, department_id, assignee_role_ids, due_rule, processing_days, display_order, documents_json, created_at`.
  → **KHÔNG có cột `instructions`** (cũng không có field nào mang nghĩa "hướng dẫn thực hiện").
- DTO phía reminder `WorkflowStepConfig` (`recipient_resolver.go:11-16`): chỉ `StepID, StageName, AssigneeRoleIDs, DepartmentID` → **không có Instructions**.
- DTO phía CMS `GlobalWorkflowStepInput` (`contracts.go:990-998`): `StepID, Stage, DepartmentID, AssigneeRoleIds, DueRule, ProcessingDays, DisplayOrder` → **không có Instructions**.
- Read path: `cms_repository.go:159-181` (`listGlobalWorkflowSteps`) SELECT 7 cột, không có instructions.
- Write path: `cms_repository.go:211-225` (`UpsertGlobalWorkflow`) INSERT 8 cột + created_at, không có instructions.
- Reminder read path: `recipient_query.go:179-200` (`GetStepByID`) SELECT `stage, assignee_role_ids, department_id` — đây là query DUY NHẤT mà reminder dùng để đọc step.

**Kết luận:**
- Nguồn dữ liệu "instructions" cho step **hiện chưa tồn tại**. Phải tạo mới: cột trên `global_workflow_steps` + field trên các DTO + cập nhật cả write path (CMS) lẫn read path (`GetStepByID` của reminder).
- Owner nghiệp vụ của field này = **Platform Admin** (CMS global workflow), nhất quán với phân biệt Platform Admin vs Company Admin.
- ✅ Quan trọng: reminder đọc step qua `GetStepByID` — query này **đã chạy sẵn** trong `prepareDispatch` (`service.go:380`). Thêm `instructions` chỉ là **mở rộng SELECT thêm 1 cột → 0 query mới**.

---

### Finding 3 — implementation_guide fallback strategy

**Câu hỏi:** Khi step không có instructions, hoặc reminder là DISCLOSURE-scope (không có step), `implementation_guide` lấy từ đâu?

**Bằng chứng code:**
- Fallback ứng viên: `disclosure_type_versions.implementation_content` — **tồn tại** (`0012_disclosure_catalog_versions.up.sql:35` → `implementation_content TEXT NULL`), expose qua DTO disclosure `ImplementationContent` (`disclosure/app/contracts.go:267`).
- Tuy nhiên field này thuộc **module disclosure**, **không** được expose cho module reminder qua interface nào hiện có. Trong reminder, `DispatchCandidate` chỉ có `DisclosureTypeID` (string) — chưa có reader nào lấy implementation_content theo typeID.
- Có sẵn pattern cross-type lookup ở module deadlinealerts: `GetTypeDeadlineConfig(ctx, companyID, typeID)` + cache `map[string]*TemplateDeadlineConfig` (`deadlinealerts/app/service.go:265-287`) — nhưng nó trả về `TemplateDeadlineConfig` (deadline), **không** chứa `implementation_content`.
- DISCLOSURE-scope occurrence không có step nào → không có nguồn instructions từ step.

**Kết luận / điểm cần chốt:**
- Chuỗi fallback hợp lý: `step.instructions` (mới) → `disclosure_type_versions.implementation_content` (cross-module, chưa có reader) → chuỗi generic cố định.
- ⚠️ Ràng buộc contract: nếu meta.yaml khai báo `implementation_guide: required: true`, renderer sẽ **fail nếu rỗng** (`email_renderer.go:30-35`) → email **không gửi**. Do đó fallback **bắt buộc luôn trả về chuỗi non-empty** (generic string) để không bao giờ vỡ render.
- Quyết định cross-module reader (lấy `implementation_content`) là điểm cần làm rõ phạm vi Phase 3 (xem mục C).

---

### Finding 4 — migration impact

**Bằng chứng code:**
- Migration mới nhất: **0098** (`0098_adhoc_multi_reviewer`). Migration kế tiếp = **0099**.
- Thay đổi cần: `ALTER TABLE global_workflow_steps ADD COLUMN instructions ...` (TEXT, NULL default).
- Lưu ý từ `0091` (dòng 11): MySQL 8.0 **không** hỗ trợ `ADD COLUMN IF NOT EXISTS` → migration không idempotent (đúng convention repo này).
- Down migration: `DROP COLUMN instructions` — khả nghịch an toàn (chỉ là cột additive, NULL, không backfill).

**Kết luận:**
- Impact migration: **thấp, additive, no data loss, có down migration**. Không đụng `0001_init_core` (file cấm sửa). Đây là migration mới độc lập.

---

### Finding 5 — backward compatibility impact

**Bằng chứng code:**
- Render guard: `email_renderer.go:30-35` — thiếu HOẶC rỗng bất kỳ required var → trả lỗi → email không gửi.
- Contract tests `template_contract_test.go`:
  - `TestContract_VariableParity` (dòng 109): biến khai báo trong meta.yaml **phải bằng đúng** biến dùng trong template (thừa/thiếu đều fail build).
  - `TestContract_RenderForbiddenContent` (dòng 153): cấm `<no value>`, `{{`, `}}`, `localhost`.
  - `TestContract_CTAAbsolute` (dòng 194): mọi href phải là absolute https.
- Payload build hiện tại: `buildReminderTemplatePayload` (`repository.go:491+`) tạo payload gốc; `prepareDispatch` (`service.go:318-428`) bổ sung thêm (additive, có guard `if _, ok := payload[...]; !ok`).
- DTO/repository change (thêm Instructions) là additive (`omitempty`), tương thích ngược **với điều kiện migration chạy trước** code đọc cột mới.

**Kết luận — rủi ro tương thích ngược quan trọng nhất:**
- ⚠️ **Sequencing hazard:** Nếu Phase 2 (template khai báo + dùng biến mới, required) được **triển khai chạy thực tế** TRƯỚC Phase 3 (payload điền biến), thì mọi email reminder sẽ **fail render → không gửi**. Phase 2 **không phải đơn vị deploy độc lập**.
  - Trong mô hình phased hiện tại không deploy đến tận Phase 6 → tại Phase 6 template + payload + migration lên cùng lúc → **an toàn**. Nhưng phải ghi nhận rõ: **không được deploy riêng Phase 2**.
- Các thay đổi còn lại (DTO, repository, migration) đều additive, tương thích ngược nếu giữ đúng thứ tự: **migration trước, code sau**.

---

## B. Risks

| # | Risk | Mức độ | Bản chất |
|---|------|--------|----------|
| R1 | Phase 2 (template) deploy lẻ trước Phase 3 (payload) → render fail → email không gửi | 🔴 HIGH | Sequencing — chỉ an toàn nếu lên cùng Phase 6 |
| R2 | `implementation_guide` khai báo required nhưng fallback rỗng → render fail | 🔴 HIGH | Phải đảm bảo fallback luôn non-empty |
| R3 | recipient_name: reverse map email→full_name không an toàn khi user dùng login_id thay email | 🟡 MEDIUM | Quyết định Phase 4 (ownership đã rõ là User) |
| R4 | Cross-module lookup `implementation_content` (disclosure) từ reminder chưa có interface | 🟡 MEDIUM | Phải chọn: tạo reader mới / dùng pattern deadlinealerts / chỉ dùng generic fallback |
| R5 | N+1 sẵn có ở `GetStepByID` per-candidate (`service.go:380`) | ⚪ PRE-EXISTING | **Đã tồn tại trước feature**; thêm `instructions` = 0 query mới. Phân loại OUT OF SCOPE theo chỉ đạo. |
| R6 | Nếu Phase 4 thêm per-email name lookup trong loop → N+1 MỚI | 🟡 MEDIUM | Mâu thuẫn với "no batch optimization" — cần hướng giải quyết không-batch (xem C) |

---

## C. Clarifications Needed (cần user quyết trước Phase 3 / Phase 4)

> Các điểm này **không chặn Phase 1** (DB foundation). Phase 1 hoàn toàn rõ ràng và có thể chạy ngay khi được CONFIRM.

**CL-1 (ảnh hưởng Phase 3) — `implementation_guide` required hay optional + nguồn fallback:**
- Có 2 hướng:
  - (a) Khai báo `required: true`, và Phase 3 đảm bảo **luôn** điền chuỗi non-empty (step.instructions → generic constant). Đơn giản, an toàn render, KHÔNG cần cross-module reader.
  - (b) Khai báo `required: true` + thêm reader đọc `disclosure_type_versions.implementation_content` làm fallback giữa step và generic. Chất lượng nội dung tốt hơn nhưng phát sinh cross-module work (R4).
- **Đề xuất:** (a) cho đúng scope feature; coi reader implementation_content là enhancement riêng. Cần user xác nhận.

**CL-2 (ảnh hưởng Phase 4) — recipient_name nguồn & cách lấy không vi phạm "no batch":**
- Ownership đã xác định: **User / `users.full_name`** (Finding 1). Câu hỏi còn lại là *cách* lấy mà không tạo N+1 mới và không làm batch optimization:
  - (a) Đổi resolver để trả `email + full_name` cùng lúc (mở rộng SELECT sẵn có ở `recipient_query.go`, đổi kiểu trả về `[]string` → struct). Không thêm query, nhưng đổi signature interface.
  - (b) Fallback dùng email làm `recipient_name` (đúng như UI cho phép "tên user (nếu có) hoặc email"). 0 thay đổi resolver, 0 query mới, an toàn nhất.
- **Đề xuất:** Bắt đầu (b) để không vỡ scope; nâng lên (a) nếu user yêu cầu tên thật. Cần user xác nhận tại đầu Phase 4 (đúng như plan đã quy định "confirm ownership trước khi code").

---

## D. Recommended Adjustments (so với plan gốc)

1. **Gỡ batching/N+1/cache khỏi scope thực thi** — Plan gốc (PHASE-0-ARCHITECTURE-VALIDATION.md mục 10.2) từng đánh dấu "batch step lookup" là MUST FIX. Đối chiếu chỉ đạo Controlled Mode: N+1 hiện hữu là **pre-existing**, và thêm `instructions` vào SELECT sẵn có **không** tạo query mới → **chấp nhận phân loại OUT OF SCOPE**. Đây là điều chỉnh thống nhất chính thức giữa 2 tài liệu.
2. **Đảm bảo `implementation_guide` fallback non-empty** — chốt theo CL-1(a) để loại bỏ R2.
3. **Khóa thứ tự deploy** — ghi rõ Phase 2 không deploy lẻ; template + payload + migration lên cùng Phase 6 (loại bỏ R1).
4. **recipient_name khởi đầu bằng email-fallback** — theo CL-2(b), tránh N+1 mới (R6) và giữ đúng scope.
5. **Phase 1 scope chốt cứng:** migration 0099 (`instructions` trên `global_workflow_steps`) + thêm field `Instructions` vào `GlobalWorkflowStepInput` và `WorkflowStepConfig` + cập nhật read/write path (`cms_repository.go` list/upsert, `recipient_query.go` GetStepByID). KHÔNG đụng template/service/recipient/urgency.

---

## E. Final Verdict

**Status: NEEDS CLARIFICATION** (2 quyết định mở: CL-1, CL-2)

**Lý do:**
- 5 điểm validation đều đã có bằng chứng code rõ ràng. Trong đó:
  - Finding 1 (ownership), 2 (instructions source), 4 (migration), 5 (backward-compat sequencing) → **đã kết luận chắc chắn**.
  - Finding 3 (implementation_guide fallback) → còn quyết định nghiệp vụ/scope (CL-1).
  - Cách lấy recipient_name không-batch → còn quyết định (CL-2).
- Hai clarification này **không chặn Phase 1**. Phase 1 (Database Foundation) đã **đầy đủ thông tin, không phụ thuộc CL-1/CL-2** và có thể tiến hành ngay khi được xác nhận.

**Phase-gate readiness:**
| Phase | Trạng thái sẵn sàng |
|-------|---------------------|
| Phase 1 — DB Foundation | ✅ READY (unblocked) |
| Phase 2 — Template | ✅ READY (lưu ý: không deploy lẻ — R1) |
| Phase 3 — Payload enrichment | ⚠️ Cần chốt CL-1 trước khi bắt đầu |
| Phase 4 — recipient_name | ⚠️ Cần chốt CL-2 trước khi bắt đầu (đúng như plan đã yêu cầu) |
| Phase 5 — Integration test | ✅ phụ thuộc 1-4 |
| Phase 6 — Dev deploy + smoke QA | ✅ phụ thuộc 1-5 |

---

## F. Đề xuất hành động kế tiếp

PHASE 0 hoàn tất. **DỪNG, chờ xác nhận.**

- Nếu đồng ý các Recommended Adjustments (đặc biệt CL-1(a) và CL-2(b)) → trả lời `CONFIRM PHASE 1` để bắt đầu Database Foundation. CL-1/CL-2 có thể chốt ngay hoặc để tới đầu Phase 3/Phase 4.
- Nếu muốn điều chỉnh hướng CL-1 / CL-2 → phản hồi trực tiếp, tài liệu này sẽ được cập nhật trước khi vào Phase 1.

**Không có dòng code nào được viết trong PHASE 0.**
