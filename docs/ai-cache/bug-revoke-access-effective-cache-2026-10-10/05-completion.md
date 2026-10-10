# Completion — PR-A: thu hồi quyền có hiệu lực ngay (2026-10-10)

Trạng thái: **FIXED IN WORKING TREE**. Chưa commit, chưa deploy.

```text
Bug: H3, H17/ROLE-06, CACHE-11, CACHE-12/PERF-21, BES-10, cộng các mục review phát sinh (PERF-26/CACHE-15/BES-14, BES-12, BES-13, PERF-12, PERF-30, PERF-31/CACHE-17, BES-15)
Reproduction (test fail trước khi sửa, pass sau khi sửa; với các test đánh dấu "revert" thì gỡ bản sửa ra là fail lại):
  authorization/infra/inmemory  TestResolve_InactiveMembershipGetsNoAccess
  authorization/infra/projection TestCachedResolver_SnapshotResolvedBeforeInvalidationIsNotServed (revert → "revoked permission served from cache")
  companyaccess/app TestAccessChanges_InvalidateEffectiveAccess, TestAccessChange_InvalidatesEvenWhenRequestContextIsCanceled,
                    TestAccessChangingMutations_DeferInvalidation (AST guard), TestAccessChange_PartialFailureAfterWriteStillInvalidates (revert)
                    TestRBACRollback_DoesNotRegrantDirectPermissionToInactiveMembership
                    TestUpdateMembership_PrimaryAdminCannotBeDeactivatedAnyWay / _RejectsUnknownStatus / _StoresNormalizedStatus
                    TestPrimaryAdminGuard_FailsClosedWhenLookupFails, TestBreakGlass_InactiveTargetLosesOverlay
  iam/app TestRefresh_FailsWhenMembershipIsNoLongerActive
  companyaccess/infra/mysql TestIntegration_RollbackDoesNotRegrantDirectPermissionToInactiveMembership (skip: cần MYSQL_TEST_DSN)
Root cause: 01-root-cause.md
Fix (files):
  authorization/app/contracts.go (MembershipStatusReader)
  authorization/infra/inmemory/{resolver.go,repository.go}, authorization/infra/mysql/repository.go (IsMembershipActive + compile-time assert)
  authorization/infra/projection/{store.go,redis_store.go,cached_resolver.go} (generation theo company, key v2, token ngẫu nhiên, thiếu key generation → không cache)
  companyaccess/app/config_versioning.go (InvalidateCompany, WithoutCancel + 2s, log; helper OnSuccess/IfWritten)
  companyaccess/app/{admin_service.go, admin_service_departments.go, admin_service_titles.go, admin_service_membership_org.go,
                     admin_service_primary_role.go, rbac_custom_role.go} (28 mutation defer invalidate; UpdateMembership chuẩn hoá status
                     + guard fail-closed; DeleteMembership guard fail-closed)
  companyaccess/app/config_break_glass.go (overlay yêu cầu membership active)
  companyaccess/infra/{mysql/admin_repository_rbac_restore.go, inmemory/admin_repository_versioning.go} (BES-10)
  iam/app/service.go (Refresh yêu cầu membership còn active → 401 SESSION_EXPIRED)
Verification:
  go build ./... OK
  go vet ./...: chỉ còn lỗi có sẵn (workflowfulfillment/required_document_gate_test.go copylocks, PERF-25)
  go test ./...: tập test fail GIỐNG HỆT HEAD (11 test có sẵn ở 6 package; xem risk report PERF-18 + httpserver/disclosure/notification/config); 80 package ok (HEAD: 79)
  go test -race (authorization, iam/app, companyaccess/app): không có race
  GOOS=linux go build ./cmd/api ./cmd/worker OK
  docker compose -f docker-compose.dev.yml build api: BLOCKED (Docker daemon không chạy trên máy)
  Review song song: cache-versioning, be-security, perf-reliability. Đã xử lý mọi MEDIUM; LOW còn lại đưa xuống follow-up.
Blast radius / data repair:
  Thay đổi hành vi:
    - Membership không active → mọi quyền/scope rỗng; refresh → 401 SESSION_EXPIRED (FE xử lý như hết phiên).
    - PATCH membership status chỉ nhận active|inactive (không phân biệt hoa thường, đã trim); giá trị khác → 400 (FE chỉ gửi 2 giá trị này).
    - Mọi mutation ở trên invalidate cache của cả company (O(1)).
  Redis: namespace mới cobo_iam:effective_access:v2 + cobo_iam:effective_access_gen:v2. Cache cũ bị bỏ qua, tự hết hạn sau TTL.
  Deploy: một API instance (compose artifacts) → không có cửa sổ chạy song song hai version.
    Nếu rollback về binary cũ trong vòng TTL: flush `cobo_iam:effective_access:*` (pattern này không khớp key gen) hoặc chờ 5 phút.
  Dữ liệu: không có dữ liệu hỏng. Truy vấn kiểm tra direct grant còn hiệu lực của membership inactive (chỉ đọc, chưa chạy): xem 01-root-cause.md.
Follow-ups (chưa làm):
  - PERF-27/CACHE-19: singleflight trong CachedResolver (cần golang.org/x/sync hoặc tự viết) + metric hit/miss; endpoint lưu ma trận RBAC theo batch.
  - PERF-28: metric cobo_effective_access_invalidate_failures_total / retry qua outbox.
  - PERF-29: go-redis ContextTimeoutEnabled + Read/WriteTimeout + MaxRetries (platform/redis).
  - PERF-32: /readyz fail khi REDIS_ADDR có cấu hình nhưng không kết nối được (in-memory fallback chỉ cục bộ từng replica).
  - CACHE-16: thêm vào runbook rollback (flush key effective_access khi rollback trong vòng TTL).
  - CACHE-18: kích hoạt sau verify email không invalidate (fail-closed, chỉ ảnh hưởng UX).
  - CACHE-20: hash tag {companyID} nếu chuyển sang Redis Cluster.
  - BES-12 (phần mở rộng): chặn tự deactivate / deactivate admin cuối cùng không phải primary.
  - BES-14 (phần gốc): UpdateMembershipOrgAssignments kiểm authz từng phòng ban xen giữa các lần ghi → nên kiểm hết trước khi ghi, hoặc gói trong một transaction.
  - BES-17: tuỳ chọn revoke session khi refresh bị từ chối.
  - Thêm quyền (invite accept, provision, AssignUserToCompany, InitializeCompany) có thể trễ tối đa bằng TTL (hướng an toàn).
```
