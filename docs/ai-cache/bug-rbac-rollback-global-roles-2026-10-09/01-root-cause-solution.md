# ROLE-01: Root cause và solution

## Root cause (confirmed: code + dữ liệu DEV)
Hai hàm restore ma trận RBAC tự quyết định phạm vi thay đổi bằng `ListRoles(companyID)`, gồm role global và role protected, và so `want` đã lọc phạm vi doanh nghiệp với `current` chưa lọc. Hệ quả: role ngoài quyền của tenant bị sửa và quyền `cms`/`platform` bị xoá. Không có guard nào của đường sửa trực tiếp được áp dụng.
- MySQL: `infra/mysql/admin_repository_versioning.go:278-321` (`RestoreRBACMatrixFromSnapshot`), `admin_repository_approval.go:264-300` (`restoreRBACMatrixInTx`).
- In-memory: `infra/inmemory/admin_repository_versioning.go:204` (cùng logic; `ListRoles` bỏ qua company; thu hồi direct permission của **mọi** company).
- Đường vào: `RollbackRBACMatrixVersion` (`app/config_versioning.go:250`), `ApproveConfigApproval` → `ApplyPendingApprovalInTx`; `SubmitConfigApproval` loại `rbac.permission.remove` không kiểm role (`config_approval.go:290`).
- Layer: service + repo. Không phải FE, config hay migration.

## Quyết định đã chốt (user, 2026-10-09)
1. Rollback/apply chỉ thay đổi role **`tenant_custom`** của chính company.
2. Rollback có thay đổi đụng quyền **critical** (`criticalPermissionCodes`, `rbac_read.go:95`) phải **qua approval**.
3. Đã audit DEV read-only (`02-data-audit.md`).
4. ROLE-01 = CRITICAL.

## Thiết kế
### 1. Một nguồn quyết định, dạng hàm thuần ở tầng app
`ComputeRBACMatrixRestorePlan(companyID, roles, current, snapshot, catalog) RBACRestorePlan` (file mới `app/rbac_restore_plan.go`):
- Chỉ xét role có `RoleType == tenant_custom` và `!IsProtected`. Mọi role khác bị bỏ qua, kể cả khi snapshot cũ có mục của chúng.
- Chỉ xét quyền thuộc phạm vi doanh nghiệp (`IsEnterprisePermission`). Quyền `cms`/`platform` **không bao giờ** nằm trong danh sách thêm hoặc gỡ. Quyền trong snapshot không có trong catalog cũng bị bỏ qua.
- Kết quả: danh sách `{RoleID, PermissionID, PermissionCode, Op: add|remove}` và cờ `TouchesCritical`.
- Hai repo (MySQL, in-memory) gọi cùng hàm này, nên unit test của hàm phủ được logic mà MySQL dùng. Repo MySQL không có sqlmock.

### 2. Phòng thủ ở SQL (MySQL)
- Gỡ: `DELETE rp FROM role_permissions rp JOIN roles r ON r.role_id = rp.role_id AND r.company_id = ? AND r.role_type = 'tenant_custom' AND r.is_protected = 0 WHERE rp.role_id = ? AND rp.permission_id = ?`.
- Thêm: `INSERT ... SELECT r.role_id, ?, 'active' FROM roles r WHERE r.role_id = ? AND r.company_id = ? AND r.role_type = 'tenant_custom' AND r.is_protected = 0 ON DUPLICATE KEY UPDATE status = 'active'`.
- Nếu tầng app có sai sót, SQL cũng không chạm được role ngoài phạm vi.

### 3. Rollback có quyền critical phải qua approval
- `RollbackRBACMatrixVersion` tính plan (đọc role, quyền hiện tại và catalog), **chưa ghi gì**.
- Plan rỗng: trả kết quả như hiện tại (không tạo thay đổi).
- `TouchesCritical`: xếp vào `pending_admin_changes` với `change_type = "rbac.matrix.rollback"` (hằng mới), `proposed` = snapshot đích đã lọc, `base_live_version_no` = bản live hiện tại. Trả **202 `APPROVAL_ROUTED`** (cùng cơ chế `RemoveRolePermission`).
- Không critical: áp dụng ngay như hiện tại.
- Duyệt: dùng luồng có sẵn (chặn tự duyệt, `checkStaleProposal`, `ValidateConfiguration`, `ApplyPendingApprovalInTx` → restore theo plan ở mục 1).

### 4. `SubmitConfigApproval` (`rbac.permission.remove`)
Gọi `roleForRBACMutation` + `IsRoleProtectedForMutation` + kiểm quyền thuộc phạm vi doanh nghiệp trước khi xếp hàng, giống `RemoveRolePermission`.

### 5. In-memory repo (độ trung thực của test)
- Lưu company cho role. `ListRoles` lọc `company rỗng (global) || company == companyID`.
- Restore dùng plan ở mục 1 và chỉ thu hồi direct permission của company đó.

### Không làm trong PR này (có lý do)
- **Giới hạn snapshot chỉ chụp role `tenant_custom`** (đề xuất 5.3 trong `00-report.md`): không cần cho an toàn, vì plan đã bỏ qua role ngoài phạm vi. Nếu làm sẽ đổi nội dung compare/export phiên bản và tạo diff giả giữa snapshot cũ và mới. Để follow-up.
- Đọc qua `r.db` khi đang giữ tx (PERF-15) và N+1 trong restore: perf, tách PR-C.
- Nhãn FE cho `rbac.matrix.rollback` (`ApprovalsPanel.tsx:18`, hiện rơi về hiển thị mã thô): follow-up web nhỏ.

## Hành vi giữ nguyên (cố ý)
- Role `tenant_custom` có trong company nhưng không có mục nào trong snapshot sẽ bị gỡ hết quyền khi rollback. Đây là ngữ nghĩa rollback hiện tại, vì snapshot không phân biệt "role không có quyền" với "role chưa tồn tại". Nếu đụng quyền critical thì phải qua approval.
- Direct permission: restore đã giới hạn theo company (MySQL `ListActiveDirectPermissionsByCompany`). Không đổi.

## Rủi ro và lưu ý
- **Approver cần `system.settings`** (`authorizeConfigApprovalDecide`). Company tự đăng ký mà không ai có `system.settings` (xem ROLE-04) sẽ không duyệt được rollback có quyền critical. Đây là hành vi đã có với `RemoveRolePermission` critical. Ghi vào hợp đồng; xử lý cùng ROLE-04.
- Hợp đồng API: `POST /api/v1/admin/rbac/matrix/versions/{n}/rollback` có thể trả **202 `APPROVAL_ROUTED`**. FE hiện không gọi route này (đã grep).
- Sau fix, rollback chỉ đổi role của chính company, nên `invalidateEffectiveAccessForCompany` là đủ (hết hệ quả E).
- Không có migration. Dữ liệu DEV hiện đúng, không cần sửa.
