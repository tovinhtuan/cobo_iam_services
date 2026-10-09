# C2: Completion report (2026-10-09)

```text
Bug: Mọi route /api/v1/platform/cms/.../workflow/* (workflowconfig) chỉ kiểm tra access token.
     Bất kỳ user nào của bất kỳ tenant nào cũng publish/activate được workflow toàn cục và tạo
     được assignee role toàn cục. WORKFLOW_VERSIONING_ENABLED=true trên server dev.
Reproduction: TestAuthz_PublishActivate. Trên code cũ: tenant admin/member activate được (200,
     activated_by = user tenant); publish lọt qua authz (02-repro.md). T2/T3 có repro tương tự:
     read routes 200, tenant admin tạo được assignee role (201).
Root cause: handler workflowconfig chỉ gọi InspectAccessToken (actor()) và không được wire
     authorizer; service không kiểm tra quyền.
Fix (phương án B, bậc quyền template CMS):
  - internal/workflowconfig/transport/http/authz.go (mới): subject từ token; AccessResolver;
    requireTemplateRead/Write/Activate (platform.cms.view trước, rồi quyền template/legacy);
    authorizer nil → 503
  - version_handler.go: gate 6 route đọc (read), publish (write), activate (activate/checker)
  - catalog_handler.go: GET assignee-roles giữ chỉ cần token; POST cần write
  - internal/httpserver/server.go: truyền authSvc vào 2 chỗ wiring
  - tests: authz_test.go (personas cms/legacy/tenant, nil/lỗi authorizer, actor, quét mọi route)
Verification: 03-verify.md. Không có test fail mới so với baseline; -race PASS; Docker build PASS;
     kiểm tra ngược (gỡ gate → test FAIL); smoke local BLOCKED.
Review: be-security + admin-role: không còn đường cho tenant chạm route ghi; CMS operator đã
     seed (0009/0058/0063/0071) không mất quyền. Đã bổ sung test theo BES-01/BES-02 (LOW).
Blast radius / data repair: không đổi schema/contract của case thành công. Cần audit read-only
     global_workflow_versions và catalog assignee role, tìm bản ghi do user không có
     platform.cms.view tạo (chờ user duyệt).
Follow-ups:
  1. [Ops, cần duyệt] Tắt WORKFLOW_VERSIONING_ENABLED trên server cho tới khi deploy fix;
     POST assignee-roles không phụ thuộc flag, nên cần chặn ở nginx.
  2. [Ops, cần duyệt] Audit read-only như trên.
  3. [FE] DONE trong cobo_web_design (docs/ai-cache/bug-workflow-release-403-2026-10-09/), chưa commit; phần còn mở: ẩn nút theo quyền + alias. Mô tả gốc: Publish/activate bị 403 thì điều hướng sang /app/forbidden và promise bị bỏ lửng:
     nên dùng suppressForbiddenNavigation và hiện lỗi tại chỗ; ẩn nút theo quyền.
     Alias permissionGuards ở FE rộng hơn BE.
  4. [Docs/skill] cobo-admin-role-guard ghi "platform mutation cần rbac.manage|system.settings";
     C2 dùng bậc template theo thiết kế, nên cập nhật skill để reviewer sau không báo nhầm.
  5. [Ops] Role nào được cấp tay platform.cms.view sau 0071 có thể thiếu cms.template.write/activate.
```
