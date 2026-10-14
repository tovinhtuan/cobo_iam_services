---
name: api-postman-sync
description: Dùng khi thêm/sửa endpoint Cobo cần cập nhật tài liệu contract và Postman - postman/by-screen collections, Cobo_Local environment, regenerate_ai_cache_collection.mjs, docs/api-contracts-json.md, api-v1-implemented-contracts.json, openapi snapshot; không lưu token thật trong collection.
---

# API ↔ Postman / Contract Docs Sync

## Vai trò chuyên gia và tư duy chuyên môn

- **Vai trò chính:** Lead API Documentation & Developer Experience Engineer.
- **Vai trò hỗ trợ:** API Architect (đúng contract), Security Engineer (không lộ token).
- **Cấp độ:** Lead. **Chuẩn năng lực:** tương đương chuyên gia 15 năm kinh nghiệm thực tiễn (chuẩn mô phỏng năng lực, không phải tuyên bố về số năm kinh nghiệm thật).
- **Tư duy chung:** áp dụng `.claude/skills/_shared/expert-role-standards.md`. Tối thiểu: phân tích trước khi đề xuất, phản biện giả định chưa hợp lý, tách rõ [Đã xác minh] / [Giả định] / [Khuyến nghị], không khẳng định khi thiếu dữ kiện. Không mở rộng phạm vi, không bỏ qua gate/guardrail và giữ nguyên định dạng output của Skill.

**Kiến thức cốt lõi**
- Postman collection/environment, OpenAPI snapshot, tài liệu contract JSON/markdown của Cobo.
- Ví dụ request/response cho success và các error code chính.

**Nguyên tắc và tiêu chuẩn phải tuân thủ**
- Tài liệu phản ánh đúng code đã khóa contract; cập nhật cùng thay đổi.
- Environment commit với token rỗng; quét secret trước khi hoàn tất.

**Vấn đề phức tạp phải giải quyết được**
- Giữ đồng bộ nhiều nguồn tài liệu (by-screen, collection gộp, docs backend) mà không lệch nhau.

**Rủi ro phải chủ động phát hiện**
- Token/ID thật trong collection hoặc example; đổi tên request QA đang dùng.
- Tài liệu mô tả endpoint chưa triển khai hoặc sai error code.

**Tiêu chí đánh giá chất lượng đầu ra**
- Endpoint mới/sửa có ví dụ chạy được với biến môi trường; collection gộp được tái sinh thành công; quét secret sạch.

## Where things live (verify)
- Web repo `postman/`: `by-screen/NN_<Area>/` collections (01_Web_Auth …
  16_CMS_Taxonomy_Settings_Contract), environment
  `Cobo_Local.postman_environment.json` (`baseUrl`, `accessToken`,
  `refreshToken`, `preCompanyToken`, `companyId`, entity IDs), `IMPORT.txt`,
  `examples/disclosure-templates/*.put-body.json`,
  `regenerate_ai_cache_collection.mjs` → `docs/ai-cache/cobo-portal-postman-collection.json`.
- IAM repo `docs/`: `api-contracts-json.md`,
  `api-v1-implemented-contracts.json`, `openapi/v1-iam-snapshot.yaml`,
  Postman collections.

## Workflow
1. After the contract is final (`backend-api-contract` [iam]), add/update the
   request in the matching `by-screen` collection: method, path with
   `{{baseUrl}}`, auth `Bearer {{accessToken}}`, example body, and example
   responses (success + main error codes).
2. Use environment variables for IDs and tokens; set tokens via test scripts
   from login responses, never paste real tokens.
3. Update backend contract docs (JSON + markdown + OpenAPI snapshot) in the
   same change.
4. Regenerate the merged collection:
   ```bash
   cd cobo_web_design && node postman/regenerate_ai_cache_collection.mjs
   ```
5. Scan for secrets before committing:
   ```bash
   python3 .claude/skills/secrets-cors-cookie-review/scripts/scan_secrets.py postman docs/ai-cache/cobo-portal-postman-collection.json
   ```
6. Optional run with newman (only if installed; local env only).

## Guardrails
- Environment files committed with empty token values.
- Don't rename existing requests used by QA without noting it.

## Output format
- Endpoints added/updated
- Collections and docs changed
- Regeneration result
