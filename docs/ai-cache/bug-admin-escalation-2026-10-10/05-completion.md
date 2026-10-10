# Completion — PR-B: leo quyền và thao tác chéo tenant (2026-10-10)

Trạng thái: **FIXED IN WORKING TREE**. Chưa commit, chưa deploy. Base: `2996f00` (PR-A).

## Đã sửa

Mỗi mục có test fail trước khi sửa. Các mục chính đã kiểm thêm: gỡ bản sửa ra thì test fail lại.

| Finding | Thay đổi | File |
|---|---|---|
| ROLE-02 (HIGH) | `company_id` của resource-scope/workflow-assignee rule lấy từ token; body `nil` → 400 | `app/admin_service.go` |
| ROLE-03 (HIGH, phương án A) | `grantLimitFor`/`assertCanGrant`: người không có `rbac.manage` và không phải platform operator chỉ cấp được role/direct permission ⊆ quyền hiệu lực của mình → 403 + `details.permission_codes`. Áp ở `CreateUser`, 2 nhánh invite, `AssignRole`, `ReplaceMembershipPrimaryRole`. `ListInviteRoles` lọc theo cùng quy tắc (API-17). | `app/role_grant_subset.go`, `admin_service.go`, `admin_service_primary_role.go` |
| ROLE-13 (MEDIUM) + BES-21 + ROLE-19 + API-19 | Không phải platform operator thì không được gỡ role/quyền tầng platform (`isPlatformTierPermission`: prefix `platform.`/`cms.` + `EnterpriseDenyCodes`), qua bất kỳ đường nào: `RemoveRole`, thay primary role, `RemoveDirectPermission`, submit và approve direct-remove, khoá hoặc xoá membership đang giữ quyền platform | `app/admin_service_company_scope.go`, `admin_service.go`, `admin_service_primary_role.go`, `config_approval.go` |
| ROLE-05 (MEDIUM) + BES-19 + API-20 | `assertRoleRemovalKeepsAdmin`: không gỡ role admin của primary admin, không để công ty mất admin cuối cùng. Áp cho `RemoveRole` và cho `PUT /primary-role` khi role mới không có quyền admin. Chỉ áp khi member thật sự giữ role đó. | `app/rbac_role_assignability.go`, `admin_service.go`, `admin_service_primary_role.go` |
| ROLE-12 (MEDIUM) | Rollback rule `alert_channel_prefs`: validate payload + entitlement, rồi xếp hàng duyệt (202). Xoá rule prefs → 409. Handler trả 202. | `app/config_versioning.go`, `admin_service.go`, `transport/http/admin_handler_versioning.go` |
| BES-09 + BES-20 | Duyệt prefs kiểm lại entitlement theo tier của người yêu cầu | `app/config_approval.go` |
| **BES-18 (HIGH, lỗi cũ từ `01ecdbc7`)** | Apply approval notification trên MySQL ghi vào cột `payload` không tồn tại, nên mọi lần duyệt prefs trả 500. Sửa thành `payload_json`. Khi không có dòng nào được cập nhật và rule đã bị xoá → 409 `STALE_PROPOSAL`. | `infra/mysql/admin_repository_approval.go` |
| ROLE-14 (INFO) | Target của break-glass không được duyệt chính grant đó | `app/config_break_glass.go` |
| RP-07 | Test guard: mọi action literal trong `companyaccess` phải có case tường minh trong `legacyPolicy` | `authorization/infra/mysql/legacy_policy_admin_actions_test.go` |
| BES-23, BES-24 | Guard nil (`view` của role, payload `null` của `CreateNotificationRule`) | `role_grant_subset.go`, `admin_service.go` |

## Test đổi theo hành vi mới (có chủ đích)
- `TestInviteUser_AcceptsWorkflowAndAdHocPermissions`, `TestInviteUser_AdditionalPermissionsIndependent`: người mời giờ phải giữ các quyền mình cấp.
- `TestNotificationRollback_CreatesNewSnapshot`: chuyển sang rule thường, vì rule prefs giờ đi qua approval.

## Kiểm tra
- `go build ./...`: OK.
- `go vet ./...`: chỉ còn lỗi có sẵn (PERF-25).
- `go test ./...`: tập test fail **giống hệt HEAD gốc** (11 test có sẵn); 80 package `ok`.
- `-race` (`companyaccess/app`, `authorization`): không có data race.
- Build Linux `cmd/api` và `cmd/worker`: OK.
- Docker: **BLOCKED** (daemon không chạy).
- Integration MySQL, gồm test mới `TestIntegration_ApproveNotificationPrefsPatch_WritesRule`: skip vì không có `MYSQL_TEST_DSN`. **Nên chạy trước khi merge**, vì BES-18 chỉ lộ ra trên MySQL.
- Review song song: admin-role, be-security, api-compat. Đã xử lý mọi HIGH/MEDIUM; LOW còn lại nằm ở follow-up bên dưới.

## Ảnh hưởng FE (cần PR-F ở `cobo_web_design`)
- **API-17:** invite, create-user, primary-role, gỡ role và gỡ quyền trực tiếp cần `suppressForbiddenNavigation: true`. Hiện 403 sẽ chuyển cả trang sang `/app/forbidden`. Nên hiển thị `details.permission_codes` inline trong modal mời và trong `mapPrimaryRoleChangeError`. Picker role đã được lọc phía BE nên trường hợp này hiếm hơn.
- **API-18:** ẩn nút "Phê duyệt" break-glass khi `target_membership_id === membershipId`, và `approve` cần `suppressForbiddenNavigation`.
- **Không đổi:** rollback/xoá rule notification (FE không gọi); PATCH membership (FE đã gửi `status`).

## Follow-up (chưa làm)
- **BES-22/ROLE-23:** kiểm "admin cuối cùng" rồi mới ghi, không khoá (TOCTOU). Nên khoá company row trong cùng transaction (gộp vào PR-C, cùng chỗ với PERF-19).
- **ROLE-20:** guard ROLE-13 dựa trên mã quyền. Operator có `rbac.manage` từ một role không mang quyền platform vẫn có thể bị gỡ role đó. Nên kiểm theo kết quả: target đang là operator và sẽ hết là operator.
- **ROLE-21:** `AssignUserToCompany` (service) chưa có guard role riêng; hiện chỉ được gọi từ route platform.
- **BES-25:** `permission_codes` cho người mời biết các quyền họ còn thiếu, nhưng là tên trong catalog, không phải dữ liệu tenant. Chấp nhận.
- **BES-26:** người mời được uỷ quyền mà thiếu một quyền của `user_thuong` sẽ không mời được, kể cả với role mặc định. Đây là hành vi mong muốn của phương án A; cần thông báo cho nhóm.
- **BES-28/ROLE-24:** action không nằm trong `companyaccess` vẫn rơi về `system.settings`. Có thể cân nhắc deny mặc định cho `admin.*`.
- **API-21:** runbook. Rollback rồi deploy lại trong vòng TTL thì xoá `cobo_iam:effective_access_gen:v2:*`.
- **API-23:** FE tạo rule thiếu `rule_code`/`resource_type`/`scope_type`. Lỗi riêng, ngoài phạm vi.
- **Dữ liệu:** truy vấn kiểm tra (chỉ đọc) trong `01-root-cause.md`. Kiểm thêm trên DEV các approval `rbac.direct_permission.remove` đang pending cho quyền platform (ROLE-19).
