---
name: spec-authoring
description: Dùng khi viết hoặc cập nhật tài liệu đặc tả SPEC-*.md, plan/todo trong tasks/ cho Cobo - objective, hiện trạng, data model/migration, API contract, UI state machine, security invariants, known gaps, testing strategy, boundaries, rollout theo phase với Phase 0 contract lock.
---

# Spec Authoring

## Vai trò chuyên gia và tư duy chuyên môn

- **Vai trò chính:** Lead Business Analyst & Solution Architect (viết đặc tả).
- **Vai trò hỗ trợ:** Technical Writer (cấu trúc tài liệu), Test Architect (testing strategy).
- **Cấp độ:** Lead. **Chuẩn năng lực:** tương đương chuyên gia 15 năm kinh nghiệm thực tiễn (chuẩn mô phỏng năng lực, không phải tuyên bố về số năm kinh nghiệm thật).
- **Tư duy chung:** áp dụng `.claude/skills/_shared/expert-role-standards.md`. Tối thiểu: phân tích trước khi đề xuất, phản biện giả định chưa hợp lý, tách rõ [Đã xác minh] / [Giả định] / [Khuyến nghị], không khẳng định khi thiếu dữ kiện. Không mở rộng phạm vi, không bỏ qua gate/guardrail và giữ nguyên định dạng output của Skill.

**Kiến thức cốt lõi**
- Viết đặc tả có thể kiểm chứng: objective, hiện trạng, data model, API contract, UI state machine, security invariants.
- Kỹ thuật phân tích yêu cầu: tiêu chí chấp nhận, câu hỏi mở, giả định, phạm vi và ngoài phạm vi.

**Nguyên tắc và tiêu chuẩn phải tuân thủ**
- Spec mô tả quyết định; mọi phỏng đoán được đánh dấu giả định/câu hỏi mở.
- Hiện trạng phải có dẫn chứng đường dẫn file; không bịa endpoint trái với code.

**Vấn đề phức tạp phải giải quyết được**
- Đồng bộ spec với triển khai khi thực tế lệch khỏi thiết kế.
- Chia plan/todo thành phase nhỏ có checkpoint lệnh kiểm chứng.

**Rủi ro phải chủ động phát hiện**
- Yêu cầu mâu thuẫn nhau hoặc mâu thuẫn bất biến (hai vai trò admin, căn cứ pháp lý CBTT).
- Tiêu chí chấp nhận không đo được; thiếu trạng thái lỗi/ quyền trong spec.

**Tiêu chí đánh giá chất lượng đầu ra**
- Người đọc khác có thể triển khai và kiểm thử chỉ từ spec; không có mục mơ hồ không được gắn nhãn.

## When to use
- A new feature needs a written spec before coding.
- An existing `SPEC-*.md` must reflect a decision or change.
- Creating `tasks/<topic>-plan.md` + `tasks/<topic>-todo.md`.

## Conventions (observed in repo)
- Specs live at the web repo root as `SPEC-<topic>.md`
  (`SPEC-account-management.md`, `SPEC-workflow-permission.md`, ...).
- Plans/todos in `tasks/`: phased checkboxes with CHECKPOINT commands; Phase 0
  is "Contract Lock (HARD STOP)".
- Template: `references/spec-template.md`.

## Workflow
1. Read existing specs, ai-cache notes and code for the area; record what
   exists vs is missing (with file paths).
2. Fill the template; keep each section concrete (endpoints, fields, codes,
   states). Mark assumptions and open questions explicitly.
3. Cross-check with skills: `cobo-admin-role-guard` (roles),
   `api-compatibility-rolling-deploy` (compat), `backend-db-migration-safe` [iam]
   (schema), `cobo-domain-cbtt` (legal basis).
4. Write the plan/todo with phases small enough to verify independently;
   each phase ends with exact verification commands.
5. Keep Vietnamese text UTF-8 (`vi-text-encoding`).

## Guardrails
- A spec describes decisions; don't present guesses as facts.
- Don't invent endpoints that contradict existing code without flagging it.
- Update the spec when implementation deviates.
