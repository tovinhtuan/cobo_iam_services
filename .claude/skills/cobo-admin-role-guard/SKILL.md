---
name: cobo-admin-role-guard
description: Dùng khi chạm bất kỳ màn hình, route, endpoint hoặc permission nào của admin trong Cobo - phân biệt Platform Admin (/cms, /api/v1/platform/cms, platform.cms.view) với Company Admin (/app/admin, /api/v1/admin, scope company_id từ token); chặn leo quyền, cross-tenant, IDOR, lộ verification_status.
---

# Cobo Admin Role Guard

Confusing the two admin roles is a **critical** bug in this product.

## Vai trò chuyên gia và tư duy chuyên môn

- **Vai trò chính:** Principal IAM / Authorization Architect cho SaaS đa tenant.
- **Vai trò hỗ trợ:** Application Security Architect (IDOR, mass assignment), Frontend Architect (route guard).
- **Cấp độ:** Architect. **Chuẩn năng lực:** tương đương chuyên gia 20 năm kinh nghiệm thực tiễn (chuẩn mô phỏng năng lực, không phải tuyên bố về số năm kinh nghiệm thật).
- **Tư duy chung:** áp dụng `.claude/skills/_shared/expert-role-standards.md`. Tối thiểu: phân tích trước khi đề xuất, phản biện giả định chưa hợp lý, tách rõ [Đã xác minh] / [Giả định] / [Khuyến nghị], không khẳng định khi thiếu dữ kiện. Không mở rộng phạm vi, không bỏ qua gate/guardrail và giữ nguyên định dạng output của Skill.

**Kiến thức cốt lõi**
- Mô hình RBAC đa tenant của Cobo: Platform Admin (`platform.cms.view`) và Company Admin (scope `company_id` từ token).
- Tenant boundary → permission → data scope → responsibility; effective access cache.

**Nguyên tắc và tiêu chuẩn phải tuân thủ**
- Nhầm lẫn hai vai trò admin là lỗi Critical; backend là nguồn sự thật, guard FE chỉ là UX.
- Least privilege: không cấp quyền mà actor không có; tenant admin không bao giờ chạm `platform.*`, `status`, `verification_status`.

**Vấn đề phức tạp phải giải quyết được**
- Người dùng nhiều membership, chuyển công ty, alias quyền ở FE mở rộng ngoài ý muốn.
- Phát hiện IDOR trên mọi ID trong path/query/body và mass assignment trong request struct.

**Rủi ro phải chủ động phát hiện**
- Endpoint đặt sai prefix/package, handler thiếu check quyền, `company_id` lấy từ body.
- Cache quyền stale sau khi đổi role; thiếu audit log cho thao tác đặc quyền.

**Tiêu chí đánh giá chất lượng đầu ra**
- Mọi endpoint/route bị chạm có kết luận theo checklist; test bắt buộc (cross-tenant, mass assignment, 403) có hoặc được nêu là thiếu.

## The two roles

| | Platform Admin (Admin Web) | Company Admin (Admin Doanh nghiệp) |
|---|---|---|
| Gate | `platform.cms.view` (strict); mutations also `rbac.manage` or `system.settings` | tenant permissions (`admin.membership.*`, `admin.role.permission.*`, `rbac.manage` within company) |
| Frontend | `src/features/cms-core/`, `/cms/*`, `RequirePlatformAccess`, `hasCmsRoutePermission` | `src/features/admin-core/`, `/app/admin/*`, `RequirePermission` + `routePermissionMatrix`, `useGuard` |
| Backend | `/api/v1/platform/cms/...` (`internal/platformcms`) | `/api/v1/admin/...` (`internal/companyaccess`) |
| Scope | Any company on the platform | Only `company_id` from the token (`sub.CompanyID`) |
| May change | company `status`, `verification_status`, platform users, CMS content | own company profile fields, memberships, roles, departments, titles |
| Must not | - | list other companies, change `status`/`verification_status`, touch cross-company resources |

Post-login: `platform.cms.view` → `/cms`; otherwise `/app/...`.
One user may have several memberships; the active `company_id` +
`membership_id` come from the token; switching uses select/switch company.

## Checklist for every change

Backend
- [ ] Endpoint lives under the correct prefix and package for its role.
- [ ] Handler inspects the token and checks the exact permission(s).
- [ ] Tenant admin endpoints derive `company_id` from the token; any
      `company_id`/`membership_id`/record ID in path, query or body is
      verified to belong to that company (IDOR).
- [ ] Tenant admin request structs cannot set `status`,
      `verification_status`, platform roles or another company's IDs
      (mass assignment).
- [ ] Role/permission assignment cannot grant permissions the actor lacks
      (escalation), and cannot grant `platform.*` from tenant admin.
- [ ] Effective-access cache invalidated (or staleness accepted and
      documented) after role/permission changes.
- [ ] Audit log entry for privileged mutations.

Frontend
- [ ] `/cms` screens only reachable via `RequirePlatformAccess`; `/app/admin`
      via `RequirePermission`.
- [ ] Permission aliases in `permissionGuards.ts` do not widen access
      unintentionally.
- [ ] UI hides fields the role cannot edit, **and** the backend rejects them.
- [ ] Company switch clears company-scoped state and caches.

## Required tests
- Tenant admin of company A calls with company B's IDs → 403/404.
- Tenant admin tries to set `verification_status`/`status` → rejected.
- User without `platform.cms.view` hits `/api/v1/platform/cms/...` → 403.
- Platform admin without `rbac.manage`/`system.settings` attempts CMS mutation → 403.
- Frontend: user lacking permission is redirected to `/app/forbidden`; platform
  admin lands on `/cms`.

## Output format
- Role(s) affected
- Endpoints/routes checked
- Findings (severity, file:line)
- Tests added
