# ROLE-01: Verify (Gate V)

Ngày 2026-10-09, cây làm việc trên HEAD `7a131a1`.

## Kết quả
| Kiểm tra | Kết quả |
|---|---|
| Gate R → GREEN | 9 test FAIL trước khi sửa (`03-repro.md`) nay PASS; thêm 2 test quyền trực tiếp (critical → approval, không critical → áp dụng ngay) |
| `go build ./...` | OK |
| `go test ./... -count=1` so với baseline HEAD (worktree tạm) | Không có fail mới. Baseline fail 15 test (companyaccess 2, disclosure/httpserver/notification 13, đều có sẵn). Bản mới fail 14: thiếu `TestSchemaParityConditionalPeriodicity` (fail ở baseline, pass ở bản mới, không thuộc vùng đã sửa, nhiều khả năng không ổn định) |
| `go vet ./...` | Chỉ còn 2 cảnh báo copylocks có sẵn (`workflowfulfillment/required_document_gate_test.go:326-327`) |
| `go test -race -count=1 ./internal/companyaccess/...` | Không có DATA RACE; chỉ còn 2 fail có sẵn (`TestCreateSelfServiceCompany_FeatureFlagOff`, `TestUpdateNotificationRule_TierEnforcement_FlagOffAllowsPremium`) |
| `docker compose -f docker-compose.dev.yml build api` | OK |

## Revert-check (gỡ guard → test phải FAIL lại; file đã khôi phục, test xanh lại)
| Guard bị gỡ | Test FAIL |
|---|---|
| G1 điều kiện `tenant_custom`/protected trong `restorableRoles` | `TestComputeRBACMatrixRestorePlan`, `TestRBACRollback_KeepsGlobalRolePermissions`, `..._DoesNotModifyProtectedTenantDefaultRole`, `TestRBACApprovalApply_KeepsOutOfScopeRoles`, `TestRBACRollback_CriticalChange_RoutesToApproval`, `TestRBACRollback_DoesNotReintroduceCMSPermission` |
| G2 kiểm phạm vi doanh nghiệp khi gỡ quyền | `TestComputeRBACMatrixRestorePlan`, `..._ClassifiesThroughCatalog` |
| G3 guard role protected ở `SubmitConfigApproval` | `TestSubmitConfigApproval_RBACPermRemove_RejectsProtectedAndGlobalRoles` |
| G4 định tuyến approval cho rollback critical | `TestRBACRollback_CriticalChange_RoutesToApproval`, `..._CriticalApproval_StaleAfterNewVersion`, `..._DirectCriticalChange_RoutesToApproval` |
| G5 kiểm quyền critical trực tiếp | `TestRBACRollback_DirectCriticalChange_RoutesToApproval` |

Hai lớp độc lập cùng bảo vệ role `cms_operator`: G1 (không xét role protected) và G2 (không gỡ quyền `cms`/`platform`). Gỡ riêng G1 thì test `KeepsPlatformPermsOnTenantDefaultRole` vẫn xanh nhờ G2, đúng thiết kế.

## SQL MySQL (không có sqlmock)
- `PREPARE` câu `INSERT` bị MySQL từ chối trong phiên chỉ đọc (1792), và việc chuẩn bị câu DML nằm ngoài phạm vi "truy vấn chỉ đọc" đã được duyệt nên **không** chạy trong phiên ghi.
- Đã kiểm tra bằng SELECT chỉ đọc trên DEV (`role01_select.out`): phần SELECT con của INSERT và điều kiện JOIN/WHERE của DELETE chạy được; cột `roles.company_id/role_type/is_protected/status` tồn tại; `role_permissions` có unique key `(role_id, permission_id)` nên `ON DUPLICATE KEY UPDATE` đúng; điều kiện guard không với tới dòng nào của role protected/global (0 dòng).
- **Chưa chạy thật** hai câu INSERT/DELETE mới. Để xác nhận cần smoke có ghi dữ liệu DEV (xem plan T9); chỉ làm khi user duyệt.
- Plan (`ComputeRBACMatrixRestorePlan`) là hàm thuần, có test bảng; hai repo dùng chung hàm `BuildRBACRestorePlan`.

## Grep: mọi chỗ ghi `role_permissions`
| Vị trí | Đánh giá |
|---|---|
| `infra/mysql/admin_repository_rbac_restore.go` (mới) | Có điều kiện `company_id`, `tenant_custom`, `is_protected = 0` |
| `infra/mysql/admin_repository.go:571,580` (`AddRolePermission`/`RemoveRolePermission`) | Chỉ gọi từ `AssignRolePermission`/`RemoveRolePermission` (guard protected ở service) và `CloneRole` (role vừa tạo). Repo method chưa tự kiểm company/role_type: follow-up phòng thủ chiều sâu |
| `infra/mysql/admin_repository_provision.go:242` | Xoá quyền của role thuộc đúng company đang bị xoá |
| `iam/registrationmysql/register_public.go:57,139` | Tạo role cho company mới đăng ký |

## Không thể kiểm chứng ở đây
- Hành vi thật của hai câu SQL mới trên MySQL (xem trên).
- FE: không gọi route rollback; nhãn `rbac.matrix.rollback` trên màn Approvals sẽ hiển thị mã thô (follow-up web).

## Bổ sung sau vòng review (be-security, admin-role, api-compat)
Gate R/V chạy lại cho các điều chỉnh. Test viết trước, RED rồi mới sửa:
- RED trước khi nối: `TestRBACRollback_KeepsOutOfScopeDirectGrants`, `..._DoesNotRevokeUnlistedDirectGrants`, `..._RequiresRbacManageWhenItChangesSomething` (service), `TestRollbackRBACMatrix_ResponseShapes` (handler, hiện trả envelope lồng), cộng test bảng `TestComputeRBACDirectRestorePlan` và các case grant-policy/tier của `TestComputeRBACMatrixRestorePlan`.
- GREEN sau khi sửa. `go test ./...` so với baseline HEAD: không có fail mới (baseline 21 dòng FAIL gồm tóm tắt package, bản mới 20). `go vet ./...` chỉ còn 2 cảnh báo có sẵn. `go test -race ./internal/companyaccess/...`: không có DATA RACE, chỉ 2 fail có sẵn. Docker build api OK.
- Revert-check bổ sung (gỡ guard → test FAIL, đã khôi phục):

| Guard bị gỡ | Test FAIL |
|---|---|
| G6 đồng bộ quyền trực tiếp không giới hạn theo `GrantablePermissions` | `TestComputeRBACDirectRestorePlan`, `TestRBACRollback_DoesNotRevokeUnlistedDirectGrants`, `TestRBACRollback_KeepsOutOfScopeDirectGrants` |
| G7 thêm quyền vào role không theo grant policy | `TestComputeRBACMatrixRestorePlan`, `TestRBACRollback_DoesNotGrantNonGrantablePermissionToCustomRole` |
| G8 rollback không đòi `rbac.manage` | `TestRBACRollback_RequiresRbacManageWhenItChangesSomething` |
| G9 handler trả envelope lồng cho 202 | `TestRollbackRBACMatrix_ResponseShapes` |
| G10 quyền `tenant_admin_only`/`high_risk` không tính critical | `TestComputeRBACMatrixRestorePlan` |

- SQL quyền trực tiếp mới (`restoreDirectGrantsTx`): đã kiểm bằng SELECT chỉ đọc trên DEV (`role01_direct_sql.out`): các cột `membership_direct_permissions` có đủ; phần SELECT con (derived table) chạy được; điều kiện `company_id` loại membership của company khác (0 dòng). Hai câu INSERT/UPDATE vẫn **chưa chạy thật** (cần smoke có ghi, chờ user duyệt).
