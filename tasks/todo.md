# Adhoc Alert Notifications — Task List

## Task 1 — Contracts & Interface Design [ ]
- [ ] Thêm `MemberInfo` struct vào `internal/adhoc/app/contracts.go`
- [ ] Thêm `ProposalNotifier` interface vào `internal/adhoc/app/contracts.go`
- [ ] Mở rộng `MembershipValidator` interface (+2 methods)
- [ ] Thêm `notifier ProposalNotifier` vào service struct + constructor
- [ ] Thêm 4 Kind constants + ResourceTypeAdHocProposal vào `internal/inappnotification/app/contracts.go`
- [ ] CP-1: `go build ./...` + `go test ./...` không regression

## Task 2 — MembershipValidator DB queries [ ]
- [ ] Implement `ResolveMembership()` trong `membership_validator.go`
- [ ] Implement `ListMembersWithPermissionFull()` trong `membership_validator.go`
- [ ] CP-2: `go test ./internal/adhoc/infra/...`

## Task 3 — Email Templates [ ]
- [ ] `adhoc.focal_review_requested/` (meta.yaml + vi/)
- [ ] `adhoc.controller_review_requested/` (meta.yaml + vi/)
- [ ] `adhoc.proposal_approved/` (meta.yaml + vi/)
- [ ] `adhoc.proposal_rejected/` (meta.yaml + vi/)
- [ ] CP-3: `go build ./...` (embed picks up new dirs)

## Task 4 — ProposalNotifier Infra Implementation [ ]
- [ ] Tạo `internal/adhoc/infra/notification/notifier.go`
- [ ] Implement `NotifyFocalsForReview`
- [ ] Implement `NotifyControllerForReview`
- [ ] Implement `NotifyCreatorApproved`
- [ ] Implement `NotifyCreatorRejected`
- [ ] CP-4a: `go test ./internal/adhoc/infra/notification/...`

## Task 5 — Service Integration [ ]
- [ ] Thêm `dispatchNotificationAsync` helper vào service.go
- [ ] Wire notification sau `SubmitProposal`
- [ ] Wire notification sau `FocalApprove`
- [ ] Wire notification sau `AdminApprove`
- [ ] Wire notification sau `Reject`
- [ ] Update `service_test.go` mock cho constructor mới
- [ ] CP-4b: `go test ./internal/adhoc/app/...`

## Task 6 — HTTP Server Wiring [ ]
- [ ] Build `AdhocProposalNotifier` trong `httpserver/server.go`
- [ ] Inject vào `adhocapp.NewService()` call
- [ ] CP-5: `go build ./...` + smoke check

## Task 7 — Frontend Notification Routing [ ]
- [ ] Extend `notificationHref()` trong `inAppNotificationsApi.ts`
- [ ] Extend `kindIcon()` trong `NotificationPanel.tsx`
- [ ] CP-6: TypeScript type check + visual verify

---

# C2 workflowconfig authz — Task List (plan: `tasks/plan.md`)

## T0 — Artefact kế hoạch [x]
- [x] `tasks/plan.md`
- [x] Append section này vào `tasks/todo.md`

## T1 — Gate publish/activate (critical path) [x]
- [x] `authz_test.go`: fakes (inspector, authorizer, version repo) + personas (tenantAdmin, tenantMember, cmsReader, cmsMaker, cmsChecker)
- [x] Gate R: chạy test, ghi output FAIL vào `docs/ai-cache/bug-workflowconfig-authz-2026-10-09/02-repro.md`
- [x] `authz.go`: subject, accessResolver, requireAnyPermission (nil → 503), requireTemplateRead/Write/Activate
- [x] `version_handler.go`: field authorizer, `NewHandler(..., authorizer)`, gate publish (write) + activate (activate)
- [x] `server.go:595` truyền `authSvc`; `version_handler_register_test.go` thêm `nil`
- [x] CP-1: `go test ./internal/workflowconfig/...` + `go build ./...`; revert gate → test fail

## T2 — Gate route đọc CMS [x]
- [x] Test: tenant → 403; cmsReader → không phải 401/403 (configuration, readiness, lifecycle, versions, versions/{n}, validate)
- [x] Gate `requireTemplateRead` ở 6 handler; xoá `actor()`

## T3 — POST assignee-roles [x]
- [x] Test: tenantMember GET → 200; tenantAdmin POST → 403; cmsMaker POST → 201
- [x] `catalog_handler.go` gate POST; `RegisterAssigneeRoleCatalog(..., authorizer)`; `server.go:580`
- [x] CP-2: `go test ./internal/workflowconfig/... ./internal/httpserver/...`

## T4 — Test quét mọi route [x]
- [x] Bảng 10 route; tenantAdmin → 403, trừ GET assignee-roles; assert số route = 10

## T5 — Verify (Gate V) [x]
- [x] `go test ./...` (không phát sinh thêm fail so với baseline 8 test fail đã biết)
- [x] `go vet ./...` (chỉ còn lỗi copylocks có sẵn)
- [x] `go test -race ./internal/workflowconfig/...`
- [x] `docker compose -f docker-compose.dev.yml build api`
- [x] Smoke local: BLOCKED (stack local không chạy); đã ghi `03-verify.md`

## T6 — Review & đóng task [x]
- [x] be-security-reviewer + admin-role-reviewer trên diff
- [x] premerge-system-review (short)
- [x] Grep lại các route `/api/v1/platform/cms`
- [x] Cập nhật ai-cache + đánh dấu C2 trong `10-risk-report.md`

## T7 — Ops (cần user duyệt từng bước) [ ]
- [ ] Tắt `WORKFLOW_VERSIONING_ENABLED` trên server / chặn POST assignee-roles bằng nginx
- [ ] Audit read-only: version/role do user không phải CMS tạo

---

# C3 adhoc legacy migrate — Task List (plan: `tasks/plan.md`, section "Plan (C3)")

## T0 — Artefact kế hoạch [x]
- [x] Append plan C3 vào `tasks/plan.md`
- [x] Append section này vào `tasks/todo.md`

## T1 — Gate R: tái hiện lỗi [x]
- [x] `TestLegacyMigrationRoute_NotExposed` trong `internal/adhoc/transport/http/handler_test.go` (kỳ vọng 404)
- [x] Fake trả dòng `{proposal-B, company-B}`; chạy trên code hiện tại → FAIL (200, `processed:1`, ghi nhận `company-B`)
- [x] Ghi output vào `docs/ai-cache/bug-adhoc-legacy-migrate-authz-2026-10-09/02-repro.md`

## T2 — Gỡ endpoint + code path cross-tenant [x]
- [x] handler.go: xoá route `adhoc-migrate-legacy-approvals` + handler `migrateLegacyApprovals`
- [x] contracts.go: xoá 2 method trong Service, type `PendingApprovalRow`, `ListPendingAdminApproval` trong repo interface; sửa doc AdminApprove
- [x] service.go: xoá `FinalizeLegacyApproval`, `ListPendingLegacyApprovals`; sửa comment @deprecated
- [x] mysql/repository.go: xoá `ListPendingAdminApproval`
- [x] Fakes: handler_test (fakeService), service_test (fakeRepository, concurrencyFakeRepo, raceFakeRepo); xoá `TestFinalizeLegacyApproval_DelegatesToAdminApprove`
- [x] CP-1: `go build ./...` + `go test ./internal/adhoc/... -count=1`; revert route → T1 fail lại

## T3 — Test khoá tenant-scope [x]
- [x] `TestAdminApprove_UsesTokenCompanyIgnoringBodySubject`
- [x] `TestRegister_NoPlatformRoutes` (quét source không có `/api/v1/platform/`)
- [x] Chạy lại `TestAdminApprove_ByNonController_Returns403`; ghi nhận SQL scope `company_id` (repository.go:120,328)
- [x] CP-2: `go test ./internal/adhoc/... -count=1`

## T4 — Verify (Gate V) [x]
- [x] Grep không còn symbol legacy trong `internal/`, `cmd/`
- [x] `go test ./...` so với baseline HEAD (worktree tạm), không có fail mới
- [x] `go vet ./...` (chỉ còn lỗi copylocks có sẵn)
- [x] `go test -race -count=1 ./internal/adhoc/...` (ghi chú PERF-10 nếu flaky)
- [x] `docker compose -f docker-compose.dev.yml build api`
- [x] Smoke local: BLOCKED (stack local không chạy); đã ghi `03-verify.md`

## T5 — Review & đóng task [x]
- [x] be-security-reviewer + admin-role-reviewer + api-compat-reviewer trên diff
- [x] premerge-system-review (short)
- [x] ai-cache: completion report; sửa C3 trong `10-risk-report.md` (HIGH có điều kiện, gate thật, fixed in branch)
- [x] Không commit/push; `go.mod` (`x/text`) để commit riêng

## T6 — Ops & follow-up (cần user duyệt từng bước) [ ]
- [ ] Chặn route ở nginx; thu hồi `system.settings` khỏi `role_org_admin_001`
- [ ] Query read-only: đếm pending theo company, dấu vết "Auto-approved by migration 0098 (D9)", role có `system.settings`, bảng `action_policy_matrix`
- [ ] Báo tenant còn dòng pending (người tạo rút / controller duyệt)
- [ ] Follow-up: nút FE duyệt legacy; proposal mồ côi; `companyaccess` authorize("rbac.manage"); authorizer bỏ qua Resource
- [x] Follow-up đã làm: `json:"-"` cho Subject (adhoc), PERF-10 rendezvous
