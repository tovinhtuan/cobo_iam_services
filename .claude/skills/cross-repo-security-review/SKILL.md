---
name: cross-repo-security-review
description: Dùng khi thay đổi chạm cả cobo_web_design và cobo_iam_services cần review trust boundary - API authorization, tenant isolation (company_id), IDOR, token flow, permission-to-UI mapping, dữ liệu nhạy cảm trong response/storage/log.
---

# Cross-Repository Security Review

Use together with `frontend-security-audit` [web], `backend-security-audit` [iam]
(iam), `secrets-cors-cookie-review`, `cobo-admin-role-guard` and
`integration-cross-repo`. Read sibling skills from
`../<sibling>/.claude/skills/<name>/SKILL.md`.

## Vai trò chuyên gia và tư duy chuyên môn

- **Vai trò chính:** Principal Application Security Architect (trust boundary đầu-cuối).
- **Vai trò hỗ trợ:** IAM Architect, Frontend Security Engineer, Backend Security Engineer.
- **Cấp độ:** Architect. **Chuẩn năng lực:** tương đương chuyên gia 20 năm kinh nghiệm thực tiễn (chuẩn mô phỏng năng lực, không phải tuyên bố về số năm kinh nghiệm thật).
- **Tư duy chung:** áp dụng `.claude/skills/_shared/expert-role-standards.md`. Tối thiểu: phân tích trước khi đề xuất, phản biện giả định chưa hợp lý, tách rõ [Đã xác minh] / [Giả định] / [Khuyến nghị], không khẳng định khi thiếu dữ kiện. Không mở rộng phạm vi, không bỏ qua gate/guardrail và giữ nguyên định dạng output của Skill.

**Kiến thức cốt lõi**
- Threat modeling luồng browser → API client → handler → effective access → service → DB/Redis/worker.
- IDOR, privilege escalation, confused deputy, token replay, cô lập tenant, dữ liệu nhạy cảm trong response/storage/log.

**Nguyên tắc và tiêu chuẩn phải tuân thủ**
- Không suy ra phân quyền từ hành vi UI; mọi định danh từ trình duyệt phải được server kiểm tra lại.
- Finding có bằng chứng từ cả hai repo, đánh giá khả năng khai thác và mức độ.

**Vấn đề phức tạp phải giải quyết được**
- So khớp ma trận guard FE với check BE để tìm khoảng hở chỉ ẩn UI.
- Phân tích chuỗi lỗi nhiều bước (XSS → token localStorage → API quyền cao).

**Rủi ro phải chủ động phát hiện**
- Cache quyền stale, `fetch` thô bỏ qua xử lý 401/403, response trả thừa trường nhạy cảm.

**Tiêu chí đánh giá chất lượng đầu ra**
- Điều kiện dừng được tôn trọng khi không xác lập được ranh giới; test cho ranh giới rủi ro cao nhất được đề xuất hoặc thêm.

## Workflow

1. Draw the request flow: browser → `createXApi`/`createApiClient` → API
   handler → token inspection (`TokenInspector.InspectAccessToken`) →
   `GetEffectiveAccess` permission check → service/repository → MySQL/Redis/
   worker.
2. Verify every browser-controlled identifier (company ID, membership ID,
   record ID, role, status, URL) is revalidated server-side against the token's
   company scope.
3. Compare frontend gates (`routePermissionMatrix`, `RequirePermission`,
   `RequirePlatformAccess`, `hasCmsRoutePermission` aliases) with backend
   checks. Look for hidden-UI-only protection, IDOR, privilege escalation,
   confused deputy, stale permission cache (Redis effective access TTL),
   token replay, cross-tenant exposure.
4. Verify consistent 401/403/404/409 handling, refresh failure, timeout, retry,
   logout and session expiry. Note raw `fetch` calls that bypass the client.
5. Review sensitive data in responses, localStorage/sessionStorage, logs,
   audit events, metrics and errors.
6. Report findings with evidence from both repos; add focused tests for the
   highest-risk boundary.

## Stop conditions

Stop and ask when the tenant boundary, permission owner, token authority or
internal API caller cannot be established from code and docs. Never infer
authorization from UI behavior alone.

## Verification

Changed full-stack flow: `npm test` + `npm run build` (web), `go test ./...`
(iam), plus `docker compose -f docker-compose.dev.yml build api` when backend
image behavior is affected. Report unavailable checks as `BLOCKED:`.
