# ROLE-01: Completion report

```text
Bug: ROLE-01 (CRITICAL). Rollback ma trận RBAC và apply approval rbac_matrix ghi đè role global, role tenant_default protected (cms_operator, admin_web, admin_doanh_nghiep) và gỡ quyền cms/platform; đồng bộ quyền trực tiếp thu hồi cả quyền ngoài phạm vi.
Reproduction: internal/companyaccess/app/rbac_rollback_scope_test.go (9 test FAIL trước khi sửa, 03-repro.md), rbac_restore_plan_test.go, transport/http/admin_handler_rbac_rollback_test.go.
Root cause (evidence): RestoreRBACMatrixFromSnapshot và restoreRBACMatrixInTx reconcile mọi role do ListRoles trả về (gồm global và protected) và so want đã lọc phạm vi doanh nghiệp với current chưa lọc; không có guard nào của đường sửa trực tiếp. Dữ liệu DEV (02-data-audit.md): rollback ở c_001 sẽ gỡ platform.cms.view khỏi cms_operator ×2 và admin_web; rollback ở tenant tự đăng ký đổi quyền self_reg_company_owner của 5 company.
Fix (files):
  app/rbac_restore_plan.go (mới): ComputeRBACMatrixRestorePlan, ComputeRBACDirectRestorePlan, BuildRBACRestorePlan, grant policy, critical theo tier
  app/rbac_rollback_approval.go (mới): rbacRestoreImpact, routeRBACRollbackToApproval (change_type rbac.matrix.rollback)
  app/config_versioning.go (rollback: impact -> rbac.manage khi có thay đổi -> 202 nếu critical), app/config_approval.go (SubmitConfigApproval rbac.permission.remove kiểm role/phạm vi)
  configversion/types.go (hằng mới)
  infra/mysql/admin_repository_rbac_restore.go (mới: SQL có điều kiện company_id/tenant_custom/is_protected; quyền trực tiếp có điều kiện company), admin_repository_versioning.go, admin_repository_approval.go
  infra/inmemory (role có company; restore dùng plan)
  transport/http/admin_handler_versioning.go (202 dạng phẳng)
  docs: api-contracts-json.md, qa-test-matrix.csv
Verification: 04-verify.md (test vs baseline không fail mới, vet, race, Docker build, revert-check 10 guard, SELECT chỉ đọc trên DEV).
Blast radius / data repair: Không cần sửa dữ liệu DEV (trạng thái hiện tại đúng). Không migration. FE không gọi route rollback. Route rollback có thể trả 202; SubmitConfigApproval rbac.permission.remove trả 403/404/400 cho role protected/ngoài company/ngoài phạm vi.
Follow-ups:
  1. ĐÃ LÀM (2026-10-09, user duyệt): deploy DEV + smoke có ghi, 4 câu SQL mới chạy thật, 38/38 PASS, hash dữ liệu ngoài phạm vi không đổi (release-2026-10-09/09-role01-post-deploy.md). Còn lại: nhánh approval apply chưa chạy trên DEV vì c_001 không có người duyệt system.settings.
  2. Tính plan lần 2 bên trong transaction của repo (TOCTOU, BES-03/ROLE-15, hẹp); truyền plan đã kiểm vào repo hoặc kiểm lại critical trong tx.
  3. Snapshot sau apply approval đang lưu bản đề xuất thay vì trạng thái thực (ROLE-14); thêm danh sách thay đổi vào summary approval cho người duyệt; CloneRole/CreateCustomRole chưa chụp phiên bản.
  4. Approval do binary cũ tạo cho rbac.permission.remove trên role protected/global: apply bản mới là no-op im lặng (API-03). DEV hiện không có dòng pending như vậy (chỉ 1 rbac.direct_permission.remove). Khi deploy prod: huỷ/từ chối các dòng pending loại này.
  5. FE: nhãn `rbac.matrix.rollback` trong ApprovalsPanel (hiện hiển thị mã thô).
  6. AddRolePermission/RemoveRolePermission ở repo chưa tự kiểm company/role_type (chỉ guard ở service): hardening chiều sâu.
  7. Công ty không ai giữ system.settings không duyệt được rollback critical (liên quan ROLE-04).
  8. BES-05 body size: handler không dùng MaxBytesReader (chung toàn companyaccess).
```

## Phát hiện của vòng review và cách xử lý
| Reviewer | Phát hiện | Xử lý |
|---|---|---|
| be-security BES-01, admin-role ROLE-13, api-compat API-04 | Đồng bộ quyền trực tiếp không giới hạn phạm vi (có thể thu hồi `platform.cms.view`, `ad_hoc_alert.*` trực tiếp; DEV có 1 grant `platform.cms.view` và 6 grant `process_control`) | **Đã sửa**: chỉ quản lý mã trong `GrantablePermissions`, chỉ membership của company, không lặp grant đã có |
| admin-role ROLE-11, be-security BES-02 | Restore thêm quyền vào role tùy chỉnh không theo grant policy | **Đã sửa**: chỉ thêm quyền `grantable` cho role tùy chỉnh |
| be-security BES-02 | Rollback chỉ cần `rbac.manage` hoặc `system.settings` dù đổi quyền | **Đã sửa**: cần `rbac.manage` khi có thay đổi |
| admin-role ROLE-12 | Tập "critical" lệch grant policy | **Đã sửa cục bộ**: critical = tập hiện có + tier `tenant_admin_only`/`high_risk` (chỉ cho quyết định rollback, không đổi `RemoveRolePermission`) |
| api-compat API-01/02/07 | 202 dạng envelope lồng; tài liệu 200 sai; thiếu test handler | **Đã sửa** + test handler |
| api-compat API-05 | Bất đối xứng thêm/gỡ quyền trực tiếp | **Đã sửa**: dùng chung `isCriticalForRestore` |
| be-security BES-05 | Cắt `reason` theo byte | **Đã sửa**: cắt theo rune |
| be-security BES-04 | Grant trực tiếp lặp tạo dòng active trùng | **Đã sửa** (chỉ cấp grant chưa có) + kiểm membership thuộc company |
| be-security BES-03, admin-role ROLE-15 | TOCTOU plan | Follow-up 2 |
| admin-role ROLE-14, api-compat API-03 | Snapshot sau apply; approval cũ | Follow-up 3, 4 |
| api-compat API-06 | Nhãn FE | Follow-up 5 |
