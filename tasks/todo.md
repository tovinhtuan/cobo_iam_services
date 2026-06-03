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
