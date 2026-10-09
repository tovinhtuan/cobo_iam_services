# C4 + H2: Completion report (2026-10-09)

```text
Bug: Thao tác sang company khác company của token (và tạo/mời/liệt kê user "không có company")
     qua route tenant /api/v1/admin/* được quyết định bằng rbac.manage, vốn là quyền tenant
     (mọi chủ công ty tự đăng ký đều có). H2 (liệt kê membership của company khác) cùng gốc.
Reproduction: TestScope_* (admin_service_company_scope_test.go) và TestTenantRoutes_*
     (admin_handler_company_scope_test.go). Trước khi sửa: persona non-operator với company khác
     token không bị từ chối ở 7 hàm service; persona operator đi qua route tenant vẫn thao tác
     được company khác (02-repro.md).
Root cause: (1) tín hiệu "platform admin" sai (rbac.manage); (2) route tenant tin company_id từ
     client; (3) authorizer không so company của resource với company của token.
Fix (backend only, internal/companyaccess):
  - admin_service_company_scope.go (mới): isPlatformCompanyOperator (platform.cms.view và
    (rbac.manage | system.settings), không dùng overlay break-glass), resolveTargetCompany (fail
    closed khi token không có company), assertRoleHasNoPlatformPermissions
  - admin_service.go, admin_service_invite_scope.go: CreateUser, CreateMembership,
    AssignUserToCompany, ListCompanyMemberships (H2), InviteUser, ListInviteRoles,
    ResendUserInvitation, authorizeMembershipInvite, authorizePlatformCompanyAdmin, AssignRole
  - transport/http/admin_handler.go: tenantRouteCompany cho createUser, createMembership,
    listMemberships (company khác token → 403 COMPANY_SCOPE_MISMATCH; rỗng → company của token)
  - docs/api-contracts-json.md cập nhật hợp đồng
  - Test cũ đổi persona sang platform operator (thêm platform.cms.view) ở 8 test
Verification: 03-verify.md. Không có test fail mới so với HEAD 7207a95; -race sạch; Docker build PASS;
     kiểm tra ngược helper (17 nhóm test FAIL khi định nghĩa operator sai). Smoke DEV: chưa chạy
     (chưa deploy).
Review: be-security, admin-role, api-compat. 1 HIGH (ROLE-01, AssignRole không chặn role mang quyền
     platform) đã sửa; BES-01 đã sửa; còn lại LOW/INFO xem 03-verify.md.
Blast radius / data repair:
  - Role có rbac.manage nhưng thiếu platform.cms.view (22 role trên DEV) mất khả năng thao tác
    cross-company/no-company qua route tenant. Hành vi mong muốn. CMS operator
    (cms_operator c_001/c_002, admin_web c_001) có đủ quyền nên không bị ảnh hưởng.
  - Dữ liệu DEV (02-data-audit chạy read-only trong session, kết quả trong báo cáo chat): 0 membership
    tạo chéo company; 9 user không có membership do tài khoản seed tenant admin tạo qua route tenant
    (2 user đang active, có session). Chưa vô hiệu hoá: cần user duyệt riêng.
Follow-ups:
  1. [C5] Mutation theo membership_id (UpdateMembership, DeleteMembership, RemoveRole, direct
     permission, title, department, primary-role, org-assignments) chưa kiểm membership thuộc company
     của token; authorizeScopedMembershipMutation trả nil cho company scope. Gốc chung: authorizer
     bỏ qua Resource company.
  2. [BES-02] CreateMembership gắn bất kỳ user_id tồn tại vào company của mình; nên thay bằng luồng mời.
  3. [FE API-01] Coi COMPANY_SCOPE_MISMATCH là ngữ cảnh cũ (tab chưa làm mới sau khi đổi company);
     hiển thị thông báo tiếng Việt thay vì message tiếng Anh.
  4. [ROLE-02] Hợp nhất hai định nghĩa "platform operator" (isPlatformCMSOperator chỉ cần
     platform.cms.view và đang dùng cho các nhánh bỏ qua denylist/validate role).
  5. [Docs] docs/openapi/v1-iam-snapshot.yaml, docs/api-v1-implemented-contracts.json,
     docs/WebPortal.postman_collection.json:744, cobo_web_design/src/features/cms-core/routeSpecs.ts:218
     vẫn mô tả hành vi cũ.
  6. [Ops, cần duyệt] 9 user không có membership trên DEV; xem xét vô hiệu hoá.
  7. [Deploy] Chưa deploy. Rollback backend sẽ mở lại lỗ hổng.
```
