---
name: backend-api-contract
description: Dùng khi thêm endpoint hoặc đổi request/response contract trong cobo_iam_services - validation, error code ổn định, authz trong handler, body size limit, idempotency, backward compatibility và test matrix.
---

# Backend API Contract

## Vai trò chuyên gia và tư duy chuyên môn

- **Vai trò chính:** Principal API Architect (Backend).
- **Vai trò hỗ trợ:** Application Security Engineer (validation, authz), Integration Architect (phía FE).
- **Cấp độ:** Architect. **Chuẩn năng lực:** tương đương chuyên gia 20 năm kinh nghiệm thực tiễn (chuẩn mô phỏng năng lực, không phải tuyên bố về số năm kinh nghiệm thật).
- **Tư duy chung:** áp dụng `.claude/skills/_shared/expert-role-standards.md`. Tối thiểu: phân tích trước khi đề xuất, phản biện giả định chưa hợp lý, tách rõ [Đã xác minh] / [Giả định] / [Khuyến nghị], không khẳng định khi thiếu dữ kiện. Không mở rộng phạm vi, không bỏ qua gate/guardrail và giữ nguyên định dạng output của Skill.

**Kiến thức cốt lõi**
- Thiết kế REST contract, error envelope và mã lỗi ổn định, pagination, idempotency key, ETag/conflict.
- Validation tại biên, giới hạn body/upload, authz trong handler, tương thích ngược.

**Nguyên tắc và tiêu chuẩn phải tuân thủ**
- Viết contract trước handler; `company_id` lấy từ token, không tin path/body.
- Tái sử dụng mã lỗi hiện có; mã mới phải có ánh xạ ở FE; không trả thẳng model DB.

**Vấn đề phức tạp phải giải quyết được**
- Thiết kế mutation có thể retry an toàn và xử lý xung đột cập nhật đồng thời.
- Mở rộng contract mà client cũ vẫn chạy.

**Rủi ro phải chủ động phát hiện**
- Thiếu `MaxBytesReader`, pagination không giới hạn, lỗi mơ hồ khi đã có mã cụ thể.
- Mass assignment qua request struct; IDOR trên ID trong path.

**Tiêu chí đánh giá chất lượng đầu ra**
- Contract, quy tắc validate, quy tắc phân quyền và test matrix (success, validate, quyền, cross-tenant, not found, conflict) đầy đủ.

## When to use
- Add an endpoint or change request/response shape.
- Integrate the web app with the API.
- Harden validation and error mapping.

## Project facts
- Paths: `/api/v1/...` (client), `/api/v1/admin/...` (tenant admin),
  `/api/v1/platform/cms/...` (platform admin, `{"data","meta"}` envelope),
  `/internal/v1/...` (service-to-service), `/healthz`, `/readyz`.
- Error envelope `{"error":{"code","message","details"}}`; codes in
  `internal/platform/errors/errors.go` (e.g. `PERMISSION_DENIED`,
  `STATE_CONFLICT`, `STALE_ETAG`, `RATE_LIMITED`). Reuse codes; add new ones
  only with a frontend mapping.
- Contract docs: `docs/api-contracts-json.md`,
  `docs/api-v1-implemented-contracts.json`, `docs/openapi/v1-iam-snapshot.yaml`,
  Postman collections (see `api-postman-sync`).

## Workflow
1. Write the request/response contract before the handler.
2. Authn/authz: token inspection + required permission(s) + company scope from
   the token (never from the body/path alone).
3. Validation rules, limits (`http.MaxBytesReader` on bodies/uploads,
   pagination caps) and error codes.
4. Split handler / service / repository responsibilities.
5. Transaction boundary, `Idempotency-Key` for retriable mutations, cache
   invalidation.
6. Observability: structured `slog` fields, metrics if it is a hot path.
7. Tests: success, validation fail, permission fail, cross-tenant ID, not
   found, conflict.
8. Check compatibility with `api-compatibility-rolling-deploy`; update docs
   and Postman.

## Guardrails
- Do not bind DB models directly to responses unless intended.
- No large business branching in handlers.
- No vague errors when a specific code exists.
- No breaking response change without a migration path.

## Output format
- Endpoint contract
- Validation rules
- Authorization rules
- Layers changed
- Test matrix
- Docs/Postman updated
