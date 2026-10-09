# C3: Completion report (2026-10-09)

```text
Bug: POST /api/v1/platform/cms/admin/ops/adhoc-migrate-legacy-approvals duyệt proposal
     pending_admin_approval của MỌI tenant, mạo danh process controller từng tenant. Gate chỉ là
     authorize(action="rbac.manage"), thực tế fallback về system.settings và chỉ xét trong
     company của người gọi.
Reproduction: TestLegacyMigrationRoute_NotExposed. Trên code cũ: 200 {"processed":1}, finalize
     company-B bằng token company-001 (02-repro.md).
     TestAdminApprove_EmptyControllerOrMembership_Returns403 (ROLE-01: empty == empty được duyệt).
Root cause: route platform cross-tenant được gate bằng check trong phạm vi tenant (action code
     dùng tên permission, authorizer bỏ qua Resource, không kiểm platform.cms.view), cộng với việc
     cố ý mạo danh process controller (01-root-cause-solution.md).
Fix (A′, chỉ backend):
  - internal/adhoc/transport/http/handler.go: gỡ route + handler migrateLegacyApprovals
  - internal/adhoc/app/contracts.go: gỡ FinalizeLegacyApproval, ListPendingLegacyApprovals,
    PendingApprovalRow, Repository.ListPendingAdminApproval; cập nhật doc @deprecated
  - internal/adhoc/app/service.go: gỡ 2 hàm legacy; AdminApprove từ chối khi
    process_controller_id rỗng hoặc membership rỗng (ROLE-01)
  - internal/adhoc/infra/mysql/repository.go: gỡ query không scope tenant ListPendingAdminApproval
  - tests: tenant_scope_test.go (mới), handler_test.go, service_test.go
Verification: xem 03-verify.md. Không có test fail mới so với baseline HEAD; Docker build PASS;
     smoke local BLOCKED (stack không chạy).
Review: be-security, admin-role, api-compat: không có CRITICAL/HIGH/MEDIUM về bảo mật.
     Đã xử lý BES-01 (LOW) và ROLE-01 (LOW).
Blast radius / data repair: không đổi schema; FE/Postman không dùng route. Chưa xác minh dữ liệu
     (cần query read-only trên DB server, chờ user duyệt). Xem T6.
Follow-ups:
  1. [Ops, cần duyệt] Chặn route ở nginx làm lưới an toàn khi rollback (API-03); thu hồi
     system.settings khỏi role demo role_org_admin_001.
  2. [Ops, cần duyệt] Query read-only: đếm pending_admin_approval theo company; tìm
     adjustment_note='Auto-approved by migration 0098 (D9)'; role có system.settings; bảng
     action_policy_matrix có tồn tại không.
  3. [FE] Nút "Duyệt (legacy)" cho process controller (adHocAlertsApi.adminApprove chưa có UI);
     canWithdraw nên xét thêm quyền ad_hoc_alert.propose (ROLE-02).
  4. [Product] Proposal mồ côi (controller và creator đều không còn active).
  5. [Docs] DONE (banner + ghi chú trong 2 doc v3 của cobo_web_design, chưa commit). Mô tả gốc: Đánh dấu route đã gỡ trong cobo_web_design/docs/ai-cache/adhoc-multi-reviewer-*-v3-*
     (§6.7/A1, §12.5) (API-02, ROLE-03).
  6. [Hardening] DONE: thêm json:"-" cho 9 field Subject trong adhoc/app/contracts.go, kèm test
     TestRequestStructs_DoNotDecodeSubjectFromJSON (bỏ tag thì 5 struct decode được Subject → FAIL).
     Còn mở: MaxBytesReader; trả 400 khi decode body admin-approve lỗi (BES-04).
  7. [Authz] companyaccess/app/admin_service.go:983,1008,1026 dùng authorize("rbac.manage"),
     thực tế đòi system.settings; authorizer bỏ qua Resource (gốc của C4/C5/H2).
  8. [Test] DONE PERF-10: raceFakeRepo.rendezvous chỉ đóng gate ở lần gọi thứ 2. Trước sửa
     fail 4/15 lần, sau sửa 0/30 lần (-race, TestConcurrentCancelAndApprove_OneWinsOneGets409Conflict).
  9. [Repo] go.mod: x/text chuyển thành direct (ngoài scope), để commit riêng.
```
