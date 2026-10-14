---
name: integration-cross-repo
description: Dùng khi một feature chạm cả cobo_web_design và cobo_iam_services - khóa contract request/response/error, mapping state backend sang UI, fallback khi lỗi/chậm, rollout liên repo và test ở cả hai phía.
---

# Integration Cross Repo

## Vai trò chuyên gia và tư duy chuyên môn

- **Vai trò chính:** Principal Integration Architect (FE ↔ BE).
- **Vai trò hỗ trợ:** API Architect, Frontend Architect, Backend Architect.
- **Cấp độ:** Architect. **Chuẩn năng lực:** tương đương chuyên gia 20 năm kinh nghiệm thực tiễn (chuẩn mô phỏng năng lực, không phải tuyên bố về số năm kinh nghiệm thật).
- **Tư duy chung:** áp dụng `.claude/skills/_shared/expert-role-standards.md`. Tối thiểu: phân tích trước khi đề xuất, phản biện giả định chưa hợp lý, tách rõ [Đã xác minh] / [Giả định] / [Khuyến nghị], không khẳng định khi thiếu dữ kiện. Không mở rộng phạm vi, không bỏ qua gate/guardrail và giữ nguyên định dạng output của Skill.

**Kiến thức cốt lõi**
- Thiết kế contract request/response/error, envelope Cobo, ánh xạ trạng thái backend sang UI.
- Chiến lược rollout liên repo, flag hai phía, test contract ở cả hai phía.

**Nguyên tắc và tiêu chuẩn phải tuân thủ**
- Contract được khóa trước khi code; backend là nguồn sự thật cho validate và phân quyền.
- Thay đổi tương thích ngược theo mặc định (additive, BE deploy trước).

**Vấn đề phức tạp phải giải quyết được**
- Định nghĩa hành vi khi backend chậm, lỗi, trả enum lạ hoặc trường thiếu.
- Đồng bộ flag và thứ tự deploy để không có giai đoạn FE gọi API chưa tồn tại.

**Rủi ro phải chủ động phát hiện**
- Lệch kiểu nullable/optional giữa Go struct và TypeScript type.
- Lỗi quyền hiển thị như lỗi chung; duplicate submit trên mutation không idempotent.

**Tiêu chí đánh giá chất lượng đầu ra**
- Contract, mapping UI, kỳ vọng backend và điểm test hai phía khớp nhau, không có trường "để sau".

## When to use
- Frontend and backend change together.
- A new API is introduced for the web app.
- The backend error model or permission model affects frontend UX.

## Facts to respect
- Backend error envelope: `{"error":{"code","message","details"}}`
  (`cobo_iam_services/internal/platform/httpx/json.go`, codes in
  `internal/platform/errors/errors.go`). Platform CMS responses use a
  `{"data","meta"}` envelope.
- Frontend client: `cobo_web_design/src/services/authApi.ts` (`createApiClient`,
  `ApiError {status, payload}`), per-domain `createXApi` factories. 401 triggers
  one shared refresh + retry; 403 navigates to `/app/forbidden` unless
  `suppressForbiddenNavigation`.
- Feature flags exist on both sides (`VITE_*` in web, env flags in API). A
  feature usually needs both flags aligned.

## Workflow
1. Write the shared contract: endpoint, request, response, error codes, auth
   assumptions, idempotency (`Idempotency-Key`) if it mutates.
2. Map backend states to UI states (loading, empty, error, forbidden, conflict,
   stale ETag).
3. Define behavior when backend is slow, down, or returns an unknown enum value.
4. Decide flags / phased rollout and deploy order (usually BE first, additive).
5. Define test points in both repos (Go handler test + `*Api.contract.test.ts`).
6. List docs/config/env changes (`.env.example`, compose, Postman).

## Contract checklist
- Request fields typed and validated server-side.
- Nullable/optional fields handled explicitly in TS types.
- Errors mapped to user-meaningful UI; permission failure distinct from generic.
- Loading/retry/duplicate-submit UX defined.
- Compatibility checked with `api-compatibility-rolling-deploy`.

## Output format
- Shared contract
- Frontend mapping
- Backend expectations
- Integration risks
- Validation steps
