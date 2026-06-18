# Email Reminder Content Upgrade — PHASE 5 Handoff (Integration Testing)

**Mode:** Controlled Implementation — PHASE 5
**Date:** 2026-06-19
**Scope boundary:** integration tests + contract tests + render tests.
**Explicitly NOT done:** deploy, smoke QA.

---

## 1. Coverage Matrix

| # | Khía cạnh feature | Loại test | Test | Trạng thái |
|---|-------------------|-----------|------|------------|
| C1 | Template parity (declared == used) — 2 template | Contract | `TestContract_VariableParity/{reminder.deadline_approaching, reminder.workflow_step_due}` | ✅ PASS |
| C2 | Không token cấm (`<no value>`/`{{`/localhost/`map[`) | Contract | `TestContract_RenderForbiddenContent/{…}` | ✅ PASS |
| C3 | CTA href absolute https | Contract | `TestContract_CTAAbsolute/{…}` | ✅ PASS |
| C4 | Registry resolve đúng required vars (8 / 9) | Integration | `TestEmbedRegistry_ResolveDeadlineApproaching` (8), `TestEmbedRegistry_ResolveWorkflowStepDue` (9) | ✅ PASS |
| R1 | Render end-to-end (embed registry + real renderer) — deadline, full payload | Render/Integration | `TestIntegration_RenderDeadlineApproaching_FullPayload` | ✅ PASS |
| R2 | Render end-to-end — workflow step, full payload (gồm step_name) | Render/Integration | `TestIntegration_RenderWorkflowStepDue_FullPayload` | ✅ PASS |
| R3 | **Fail-loud** khi thiếu required var (email không gửi) | Integration | `TestIntegration_DeadlineApproaching_MissingRequiredVar_Errors` | ✅ PASS |
| R4 | Pipeline SendReminderEmail (recipient check + render + mock send) — 2 template | Integration | `TestIntegration_SendReminderEmail_BothTemplates_MockSend` | ✅ PASS |
| P1 | `remaining_days`/`urgency_status`/`extractStepID`/`truncate` (helpers) | Unit | `TestCalculateRemainingDays`, `TestDetermineUrgencyStatus`, `TestExtractStepID`, `TestTruncateImplementationGuide` (Phase 3) | ✅ PASS |
| P2 | `prepareDispatch` điền `recipient_name` (pre-populated + resolved) | Unit/Integration | `TestRecipientName_PrePopulatedRecipients_UsesEmail`, `TestRecipientName_ResolvedRecipients_UsesResolvedEmail` (Phase 4) | ✅ PASS |
| P3 | `prepareDispatch` deep-link portal_url (regression) | Integration | `TestFix3_*` (sẵn có) | ✅ PASS |
| P4 | Dispatch backward-compat (no alert config / config disabled / override) | Integration | `TestDispatchDue_*` (sẵn có) | ✅ PASS |
| P5 | Schema round-trip instructions (CMS read/write) | (Phase 1, build-verified) | cms_repository read/write + DTO | ✅ build/compile |

**Giá trị cốt lõi Phase 5:** R1–R4 đóng vòng lặp **payload (prepareDispatch) ↔ real renderer**. Đặc biệt R3 chứng minh ràng buộc fail-loud: thiếu bất kỳ required var → render lỗi → email **không** gửi (thay vì rò rỉ `<no value>`).

---

## 2. Test Result

### 2.1 Feature suites — ✅ ALL PASS
```
Contract (2 template × 3 test):        6/6 subtests PASS
Registry resolve (8/9 vars):           PASS
email (render + 4 integration mới):    PASS
reminder/app (helpers+dispatch+recip): PASS
```

### 2.2 Files Created (test only)
- `internal/reminder/infra/email/reminder_content_integration_test.go` — 4 integration/render test + helpers (`embedSender`, `fullDeadlinePayload`, `fullWorkflowStepPayload`).

Không tạo/sửa code sản phẩm ở Phase 5 (đúng scope: chỉ test).

### 2.3 Full suite — 8 failure pre-existing (KHÔNG đổi qua Phase 1→5)
| Package | Test | Phân loại |
|---------|------|-----------|
| `notification/app` | `TestContract_VariableParity/workflow.approved` | Pre-existing (meta thừa biến) |
| `platform/config` | `TestLoad_UserAvatarEnvOverride` | Pre-existing (Windows path) |
| `httpserver` | 5 integration (`template_category` / session / state) | Pre-existing (harness/validation drift) |
| `companyaccess/transport/http` | `TestCreateSelfServiceCompany_FeatureFlagOff` | Pre-existing (feature flag) |

→ Feature introduce **0 regression** xuyên suốt 5 phase. Tất cả 8 lỗi đã kiểm chứng error message không liên quan (chi tiết Phase 3 §4.4).

---

## 3. Remaining Risks

| # | Risk | Mức độ | Trạng thái |
|---|------|--------|-----------|
| RR1 | `recipient_name` = email (không phải `full_name`) | 🟡 MEDIUM | Đúng spec "hoặc email"; follow-up nâng cấp resolver. Không chặn release. |
| RR2 | `due_date` UTC vs `remaining_days` HCM → lệch 1 ngày ở biên | 🟡 MEDIUM | Ngoài scope; cân nhắc đồng bộ `due_date` sang HCM ở task riêng. |
| RR3 | **Sequencing deploy:** template (P2) yêu cầu required vars; payload (P3/P4) phải lên CÙNG lúc | 🔴 HIGH | Phase 6 deploy gộp: migration + code + template. **Không deploy lẻ.** |
| RR4 | Migration 0099 chưa apply trên DB nào | 🟡 MEDIUM | Apply ở Phase 6 (migration trước, code sau). |
| RR5 | `workflow.approved` pre-existing đỏ làm `notification/app` package FAIL | 🟡 MEDIUM | Ngoài scope; cần user duyệt nếu muốn fix vệ sinh. Không liên quan 2 template feature. |
| RR6 | 8 pre-existing failures che lấp tín hiệu CI | ⚪ LOW | Đã liệt kê đầy đủ; báo cáo Phase 6 phải phân biệt rõ. |

---

## 4. Ready For Phase 6

✅ **READY.** Toàn bộ coverage feature xanh; pipeline render đã được chứng minh end-to-end với payload thực tế.

**Phase 6 (Dev Deploy + Smoke QA) cần làm:**
1. Apply migration **0099** lên DEV DB (`global_workflow_steps.instructions`).
2. Deploy code (service.go, templates, DTO/repo) — **gộp** template + payload + migration.
3. Smoke QA: seed reminder occurrence (deadline + workflow step) → trigger dispatch → kiểm email thực tế (subject `[CẢNH BÁO] …`, đủ 8/9 trường, button "Xem thêm").
4. Evidence: screenshots + log dispatch.
5. **Lưu ý báo cáo:** 8 pre-existing failures sẽ vẫn đỏ — phân biệt rõ với kết quả feature.

**Gợi ý chuẩn bị dữ liệu DEV (cho Platform Admin nhập instructions):** dùng CMS global workflow editor để set `instructions` cho vài step → để smoke QA thấy "Hướng dẫn thực hiện" có nội dung cụ thể (thay vì fallback generic).

---

## 5. Trạng thái

PHASE 5 hoàn tất. 4 integration/render test mới + contract + registry + dispatch suites: **PASS**. Pipeline payload↔renderer đóng vòng, fail-loud verified. Full suite: 8 pre-existing, 0 regression.

⏸️ **DỪNG — chờ `CONFIRM PHASE 6`.**

Không tự động chuyển phase. Không deploy. Không smoke QA.
