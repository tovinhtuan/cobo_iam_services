# Root cause (confirmed bằng test fail trước khi sửa)

## Bảng nguyên nhân

| Finding | Root cause | Layer | Test tái hiện (fail trước khi sửa) |
|---|---|---|---|
| H3 (resolver) | `ListPermissionCodes` và các truy vấn data scope không kiểm `memberships.membership_status` (`internal/authorization/infra/mysql/repository.go:20-40`); resolver không có cổng trạng thái | authorization repository/resolver | `TestResolve_InactiveMembershipGetsNoAccess` |
| H3 (refresh) | `Refresh` chỉ kiểm session revoked/context (`internal/iam/app/service.go:412-425`), không kiểm membership còn active | iam service | `TestRefresh_FailsWhenMembershipIsNoLongerActive` |
| H17 | Chỉ 5 điểm gọi `invalidateEffectiveAccessForCompany` (`admin_service.go:1032,1057,1081`, `config_approval.go:493`, `config_versioning.go:297`); các mutation role/quyền/trạng thái/phòng ban/chức danh/team không gọi | companyaccess service | `TestAccessChanges_InvalidateEffectiveAccess` (lỗi khi gỡ defer), `TestAccessChangingMutations_DeferInvalidation` (AST guard) |
| CACHE-11 | `CachedResolver` `Put` không điều kiện sau `Resolve`; không có phiên bản (version) để biết entry được tạo trước hay sau invalidate (`projection/cached_resolver.go:19-28`, `redis_store.go:47-56`) | cache | `TestCachedResolver_SnapshotResolvedBeforeInvalidationIsNotServed` (fail với "revoked permission served from cache" khi bỏ generation) |
| CACHE-12 | Helper invalidate dùng request ctx, nuốt lỗi, đi qua `ListMembershipsByCompany` (`config_versioning.go:98-111`) | companyaccess / cache | `TestAccessChange_InvalidatesEvenWhenRequestContextIsCanceled` (fail khi gỡ defer) |
| BES-10 | SQL grant direct permission khi restore thiếu `membership_status='active'` (`admin_repository_rbac_restore.go:66-75`) | MySQL repo | `TestRBACRollback_DoesNotRegrantDirectPermissionToInactiveMembership` (in-memory), `TestIntegration_RollbackDoesNotRegrantDirectPermissionToInactiveMembership` (MySQL, cần `MYSQL_TEST_DSN`) |

## Lưu ý
- **Reproduction:** đều là test tự động. Riêng test MySQL bị skip trên máy này (không có DSN).
- **Dữ liệu hỏng:** không có. Đây là lỗi về thời điểm có hiệu lực của quyền, không ghi sai dữ liệu. Riêng BES-10: các direct grant đã được restore cho membership inactive trước đây vẫn còn trong DB. Truy vấn kiểm tra (chỉ đọc, chưa chạy):
  ```sql
  SELECT mdp.membership_id, mdp.permission_code, mdp.granted_at
  FROM membership_direct_permissions mdp
  JOIN memberships m ON m.membership_id = mdp.membership_id
  WHERE mdp.revoked_at IS NULL AND m.membership_status <> 'active';
  ```
  Sau bản sửa H3, các grant này không còn hiệu lực khi membership inactive. Chúng chỉ có tác dụng nếu membership được kích hoạt lại.
