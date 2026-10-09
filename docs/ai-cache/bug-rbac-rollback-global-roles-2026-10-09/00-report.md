# ROLE-01: Rollback / apply approval ma trận RBAC ghi đè role ngoài phạm vi được phép

Nguồn: risk review lần 2 (`../risk-review-2026-10-09-2/10-risk-report.md`, ROLE-01 = BES-02). Phân tích read-only trên HEAD `7a131a1`; chưa sửa code, chưa truy vấn DB.

## 1. Mô hình phân quyền hiện tại (theo code)

| Tầng | Thành phần | Nguồn |
|---|---|---|
| Danh mục quyền | `permissions` (code, `module_name`) | migrations |
| Vai trò | `roles` có `company_id` (NULL = dùng chung), `role_type`, `is_protected` | 0123 |
| Gán quyền cho vai trò | `role_permissions(role_id, permission_id)`: **không có cột company** | 0001 |
| Gán vai trò cho người | `membership_roles` (membership = user trong một company) | 0001 |
| Quyền ngoại lệ | `membership_direct_permissions`, chỉ 9 mã trong `GrantablePermissions` | `admin.go:447` |
| Quyết định | action → quyền yêu cầu (`action_policy_matrix`, fallback `legacyPolicy`); quyền hiệu lực = quyền từ role ∪ quyền trực tiếp, cache 5 phút | `authorization/infra/mysql/repository.go` |
| Kiểm soát thay đổi | snapshot ma trận (`captureRBACMatrixVersion`), rollback, approval 2 người cho quyền critical | `config_versioning.go`, `config_approval.go` |

### Ba loại vai trò (migration 0123)
| `role_type` | Phạm vi | Ví dụ | Sửa quyền được không |
|---|---|---|---|
| `system_global` (`company_id IS NULL`, protected) | dùng chung mọi tenant | `self_reg_company_owner` (0026, role của mọi chủ công ty tự đăng ký), `dept_lead` (0049) | Không. Chỉ đọc, nhân bản. |
| `tenant_default` (protected) | một company | `admin_doanh_nghiep`, `user_thuong`, `cms_operator` (c_001, c_002 giữ `platform.cms.view` + `cms.template.*`) | Không. Chỉ đọc, nhân bản. |
| `tenant_custom` | một company | role do admin tạo hoặc nhân bản | Có, trong giới hạn grant tier |

### Luật trên đường sửa trực tiếp (`AssignRolePermission` / `RemoveRolePermission`, `admin_service.go:1281-1341`)
1. Role phải nhìn thấy được từ company của token (`roleForRBACMutation`).
2. Role protected (`system_global`, `tenant_default`, `is_protected`) → 403 `PROTECTED_ROLE_READ_ONLY`, thông báo "Hãy nhân bản thành vai trò tùy chỉnh" (`rbac_grant_policy.go:147`).
3. Chỉ quyền trong phạm vi doanh nghiệp; module `cms`/`platform` và danh sách `EnterpriseDenyCodes` bị từ chối (`enterprise_scope.go`).
4. Grant tier: `system_only` không bao giờ gán được; `tenant_admin_only`/`high_risk` có giới hạn (`ValidateAssignPermissionGrant`).
5. Gỡ quyền critical phải qua approval 2 người.

## 2. Đối chiếu với bài toán quản lý thư viện (RBAC kinh điển)

| Nguyên tắc | Ví dụ thư viện | Cobo, đường sửa trực tiếp | Cobo, rollback / apply approval |
|---|---|---|---|
| User → Role → Permission | Độc giả, Thủ thư, Quản trị | Có | Có |
| Bộ vai trò mẫu do quản trị hệ thống giữ, chi nhánh chỉ dùng hoặc sao chép | Thủ thư chi nhánh không sửa định nghĩa "Độc giả" của cả hệ thống | **Đúng**: protected chỉ đọc, phải nhân bản | **Sai**: sửa thẳng role global và protected |
| Quản trị chỉ quản lý trong phạm vi của mình (ARBAC) | Thủ thư chi nhánh A không đổi quyền ở chi nhánh B | **Đúng** | **Sai**: `role_permissions` không có company, nên sửa role global ảnh hưởng mọi tenant |
| Không cấp hoặc gỡ quyền vượt cấp | Thủ thư không cấp quyền quản trị hệ thống | **Đúng**: grant tier, phạm vi doanh nghiệp | **Sai**: còn chủ động **xoá** quyền cấp hệ thống (`cms`/`platform`) |
| Tách nhiệm vụ cho thay đổi nhạy cảm | Huỷ thẻ thủ thư cần 2 người duyệt | **Đúng**: approval critical | **Sai**: rollback không qua approval |

Kết luận: thiết kế dự định **tuân theo** mô hình thư viện (bộ role mẫu chỉ đọc, tenant nhân bản ra role riêng, mỗi tenant tự quản trong phạm vi mình). Đường sửa trực tiếp làm đúng. **Đường rollback và apply approval phá vỡ cả bốn nguyên tắc**, vì nó không đi qua các guard trên.

## 3. Root cause (confirmed bằng code)
`RestoreRBACMatrixFromSnapshot` (`infra/mysql/admin_repository_versioning.go:278-321`) và `restoreRBACMatrixInTx` (`admin_repository_approval.go:264-300`) đều:
1. Lấy `roleSet = ListRoles(companyID)`. Câu này trả **cả** role global (`r.company_id IS NULL OR r.company_id = ?`, `admin_repository_rbac_read.go:65`) **và** role protected của company.
2. So sánh `want` (snapshot) với `current`:
   - `want` đã bị lọc bỏ quyền `cms`/`platform` (`filterEnterpriseRBACSnapshotJSON`, áp dụng cả lúc chụp `config_versioning.go:57`, lúc rollback `:262`, và lúc dựng proposal `config_approval.go:137-165`).
   - `current` lấy từ `ListRolePermissions`, vốn **không lọc** (`admin_repository_rbac_read.go:137-143`).
3. Xoá mọi quyền có trong `current` mà không có trong `want`, rồi chèn phần thiếu. Câu xoá theo `role_id` (`DELETE FROM role_permissions WHERE role_id=? AND permission_id=?`), không giới hạn company.

Không có bước nào gọi `IsRoleProtectedForMutation`, kiểm grant tier hay giới hạn `role_type = tenant_custom`. `SubmitConfigApproval` loại `rbac.permission.remove` (`config_approval.go:290-299`) cũng không gọi `roleForRBACMutation`/`IsRoleProtectedForMutation`, nên proposal có thể nhắm role protected hoặc global.

Cổng: rollback dùng `authorizeConfigurationHealth`, tức `rbac.manage` **hoặc** `system.settings`. Mọi chủ công ty tự đăng ký đều có `rbac.manage`.

## 4. Hệ quả

| # | Tình huống | Điều kiện | Mức chắc chắn |
|---|---|---|---|
| A | Rollback **hoặc duyệt bất kỳ approval RBAC nào** trong company có role `cms_operator` (c_001, c_002) → role này mất `platform.cms.view` và `cms.template.*` → operator CMS mất quyền vào `/cms` | Thao tác bình thường của admin company đó | Confirmed (code + migration 0009/0063/0071) |
| B | Rollback ở bất kỳ company nào đưa quyền của role global (`self_reg_company_owner`, `dept_lead`) về theo snapshot của company đó → thay đổi quyền cho **mọi** tenant dùng role này (cấp lại quyền đã thu hồi, gỡ quyền mới thêm, gỡ quyền `cms`/`platform` nếu có) | Quyền role global khác snapshot, hoặc role global giữ quyền ngoài phạm vi doanh nghiệp | Code confirmed; độ lớn **chưa xác minh** (cần query DB) |
| C | Role global hoặc custom tạo **sau** snapshot: `want` rỗng nên **toàn bộ quyền bị xoá**. Với role global (ví dụ thêm qua migration), ảnh hưởng mọi tenant | Có role mới sau thời điểm snapshot | Confirmed (code) |
| D | Role `tenant_default` protected (`admin_doanh_nghiep`, `user_thuong`) bị sửa, bỏ qua luật "nhân bản trước". Quyền critical bị gỡ không qua approval, hoặc quyền đã thu hồi được cấp lại | Rollback trong chính company | Confirmed (code) |
| E | Cache quyền hiệu lực chỉ invalidate cho company thực hiện (`invalidateEffectiveAccessForCompany`). Tenant khác bị đổi qua role global vẫn dùng cache cũ tới 5 phút, sau đó nhận quyền mới mà không có audit ở phía họ | Kèm B/C | Confirmed (code) |

**Đề xuất nâng mức:** tình huống A xảy ra trong thao tác bình thường và làm hỏng dữ liệu phân quyền của operator platform. Theo rubric (data corruption, không cần điều kiện đặc biệt), ROLE-01 nên là **CRITICAL**. B và C bổ sung tính chéo tenant.

## 5. Hướng xử lý (chưa thực hiện, chờ quyết định)
1. **Giới hạn phạm vi reconcile** ở cả hai hàm restore: chỉ role `company_id = ?` và `role_type = 'tenant_custom'` (không protected). Câu `DELETE`/`INSERT` thêm điều kiện qua join `roles.company_id = ?`.
2. **Không bao giờ xoá quyền ngoài phạm vi doanh nghiệp:** reconcile chỉ trên tập quyền doanh nghiệp, tức lọc `current` giống `want`.
3. **Snapshot chỉ chụp role `tenant_custom` của company** (role global và protected không thuộc "ma trận của tenant"). Snapshot cũ đã chứa role global thì bỏ qua các mục đó khi restore.
4. `SubmitConfigApproval` (`rbac.permission.remove`): gọi `roleForRBACMutation` và `IsRoleProtectedForMutation` như `RemoveRolePermission`.
5. Rollback: tối thiểu áp grant tier như đường trực tiếp; cân nhắc đòi approval nếu diff chứa quyền critical.
6. **Test (Gate R):** fixture có role global, role `tenant_default` mang quyền `cms`/`platform`, role custom tạo sau snapshot. Rollback và apply approval phải giữ nguyên hai loại đầu và chỉ đổi role custom.
7. **Dữ liệu:** query read-only DEV để xem rollback/approval RBAC đã từng chạy chưa (`config_versions`/audit `admin.version.rbac.*`) và role `cms_operator` hiện còn `platform.cms.view` không. Nếu đã mất, lập kế hoạch khôi phục (cần duyệt).

## 6. Quyết định đã chốt (user, 2026-10-09)
1. Phạm vi rollback/apply: **chỉ role `tenant_custom`** của chính company.
2. Rollback **phải qua approval** khi thay đổi chứa quyền critical.
3. Đã cho phép và đã chạy query read-only DB DEV: `02-data-audit.md`. Tình huống A và B được xác nhận bằng dữ liệu.
4. ROLE-01 được **nâng lên CRITICAL** trong `../risk-review-2026-10-09-2/10-risk-report.md`.
