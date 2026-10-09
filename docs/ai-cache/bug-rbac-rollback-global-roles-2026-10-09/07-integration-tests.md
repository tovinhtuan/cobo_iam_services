# ROLE-01: Test tích hợp MySQL (WS-C)

File: `internal/companyaccess/infra/mysql/rbac_restore_integration_test.go`. Chạy service quản trị trên repo MySQL thật, nên các câu SQL ở `admin_repository_rbac_restore.go` và `ApplyPendingApprovalInTx` chạy thật. Mỗi test tự seed công ty, user, role, quyền với tiền tố ngẫu nhiên `itr1xxxxxxxx` và tự dọn. Bỏ qua (skip) nếu không đặt `MYSQL_TEST_DSN`.

## Chạy local
```bash
docker network create cobo-r1-it
docker run -d --name cobo-r1-it-mysql --network cobo-r1-it -e MYSQL_ROOT_PASSWORD=root -p 127.0.0.1:33307:3306 mysql:8.0 --max-allowed-packet=67108864
docker run --rm --network cobo-r1-it -v "$PWD":/work -w /work -e DB_HOST=cobo-r1-it-mysql mysql:8.0 sh migrations/run_dev_migrations.sh
MYSQL_TEST_DSN='root:root@tcp(127.0.0.1:33307)/cobo_iam?parseTime=true&loc=UTC' go test ./internal/companyaccess/infra/mysql/ -run Integration -count=1 -v
```
(Hoặc dùng `docker compose -f docker-compose.dev.yml up -d mysql` rồi chạy migration; lưu ý H18: 4 migration chưa có trong danh sách.)

## Test
| Test | Kiểm |
|---|---|
| `RollbackRoleSQL_OnlyTouchesOwnCustomRole` | DELETE và INSERT `role_permissions` chỉ đổi role custom của company; role global, `cms_operator`, role company khác giữ nguyên |
| `RollbackDirectGrantSQL` | cấp/thu hồi quyền trực tiếp chỉ mã grantable, chỉ company này; `platform.cms.view` và quyền không grantable giữ nguyên; rollback lặp không tạo dòng active trùng |
| `CriticalRollback_ApprovalApplyRunsRestoreInTx` | rollback critical trả 202; DB chưa đổi; tự duyệt 403; duyệt bởi người khác chạy `ApplyPendingApprovalInTx` thật |
| `CriticalRollback_StaleAfterNewVersion` | duyệt sau khi có phiên bản mới → 409 |
| `RolePermRemoveApproval_ApplyKeepsOutOfScopeRoles` | duyệt gỡ quyền critical khỏi role custom không đụng role ngoài phạm vi |
| `SubmitRolePermRemove_RejectsProtectedAndForeignRoles` | 403/404/400 ở submit |

## Gate R trên code trước khi sửa (commit `7a131a1`)
Cùng file test chạy trên worktree `7a131a1`: **cả 6 FAIL** đúng lý do, gồm bằng chứng thật trên MySQL: role ngoài phạm vi bị đổi; `platform.cms.view` trực tiếp bị thu hồi; dòng quyền trực tiếp bị nhân đôi (`...:2`); rollback critical áp dụng ngay (không 202); submit nhận role protected. Trên bản đã sửa (commit `10a5700`): **6/6 PASS**.

```
7a131a1 fix bug C5'
    rbac_restore_integration_test.go:354: roles outside the company's custom roles changed:
--- FAIL: TestIntegration_RollbackRoleSQL_OnlyTouchesOwnCustomRole (0.53s)
    rbac_restore_integration_test.go:386: platform.cms.view direct grant must be untouched, got map[ad_hoc_alert.process_control:1 template.workflow.override.read:1]
    rbac_restore_integration_test.go:389: a non-grantable grant must not be re-granted, got map[ad_hoc_alert.process_control:1 template.workflow.override.read:1]
    rbac_restore_integration_test.go:400: a repeated rollback must not duplicate active grants, got map[ad_hoc_alert.process_control:2 template.workflow.override.read:2]
--- FAIL: TestIntegration_RollbackDirectGrantSQL (0.43s)
    rbac_restore_integration_test.go:414: expected 202/APPROVAL_ROUTED, got <nil>
--- FAIL: TestIntegration_CriticalRollback_ApprovalApplyRunsRestoreInTx (0.39s)
    rbac_restore_integration_test.go:447: expected 202/APPROVAL_ROUTED, got <nil>
--- FAIL: TestIntegration_CriticalRollback_StaleAfterNewVersion (0.35s)
    rbac_restore_integration_test.go:485: approval apply changed roles outside the company's custom roles:
--- FAIL: TestIntegration_RolePermRemoveApproval_ApplyKeepsOutOfScopeRoles (0.36s)
    rbac_restore_integration_test.go:499: expected 403/protected_role_read_only, got <nil>
--- FAIL: TestIntegration_SubmitRolePermRemove_RejectsProtectedAndForeignRoles (0.28s)
FAIL
FAIL	github.com/cobo/cobo_iam_services/internal/companyaccess/infra/mysql	2.345s
FAIL
```
