# Production risk review — toàn repo cobo_iam_services (2026-10-09)

- Loại: review read-only (backend-production-review → security, database-transaction, go-concurrency, idempotency-worker, failure-scenario, distributed, network-resilience, performance, memory, observability).
- Commit HEAD lúc review: `6205b4d`. Chưa verify runtime trên DEV; tất cả finding là trace code tĩnh.
- Finding đánh dấu ✔ = orchestrator đã tự đọc lại code xác nhận.

## Tool đã chạy
- `go vet ./...` → FAIL: `internal/workflowfulfillment/required_document_gate_test.go:326-327` copy `atomic.Int64` (chỉ file test).
- `go test -race ./...` → 0 DATA RACE; 8 test FAIL ở 4 package:
  - `companyaccess/app` TestUpdateNotificationRule_TierEnforcement_FlagOffAllowsPremium (APPROVAL_ROUTED)
  - `companyaccess/transport/http` TestCreateSelfServiceCompany_FeatureFlagOff — test lỗi thời: gate `COMPANY_SELF_CREATE_ENABLED` bị gỡ có chủ ý ở commit `3487029`.
  - `httpserver` 5 integration test (template_category validation, CMS review state, SESSION_EXPIRED)
  - `notification/app` TestContract_VariableParity (`workflow_instance_id` khai báo trong meta.yaml nhưng không dùng)

## CRITICAL
1. ✔ Cross-tenant takeover: `CreateMembership` (`companyaccess/app/admin_service.go:873`) lấy `company_id` từ body, checker không so resource với company của token; repo chỉ check company tồn tại. Kết hợp `AssignRole` + `ensureRoleForMembership` (`infra/mysql/admin_repository.go:889`) chấp nhận global role (`company_id IS NULL`, vd seeded `self_reg_company_owner`). Self-registered tenant admin có `admin.membership.invite` → tự join company B làm owner.
2. ✔ `CreateUser` (`admin_service.go:79-100`): `isWebAdmin = hasPermission("rbac.manage")`, nhưng `rbac.manage` được cấp cho mọi chủ company tự đăng ký (`iam/registrationmysql/register_public.go:204`) → tạo user + membership trong company bất kỳ.

## HIGH
- IDOR membership theo ID: Update/Delete/AssignRole/RemoveRole/Title/PrimaryRole/OrgAssignments/DirectPermission không `requireMembershipInCompany`; `authorizeScopedMembershipMutation` return nil cho company scope (`admin_delegation_scope.go:130`); SQL chỉ `WHERE membership_id=?`.
- IDOR đọc: `GET /api/v1/admin/companies/{company_id}/memberships` không so `company_id` với token (`admin_handler.go:339`).
- `POST /api/v1/platform/cms/admin/ops/adhoc-migrate-legacy-approvals` chỉ gate `rbac.manage`, query không lọc company → approve proposal của mọi tenant (`adhoc/transport/http/handler.go:302`, `adhoc/infra/mysql/repository.go:640`).
- ✔ Membership inactive vẫn có quyền: `ListPermissionCodes` không lọc `m.membership_status` (`authorization/infra/mysql/repository.go:20`), `Refresh` không check membership (`iam/app/service.go:412`), cache effective-access TTL 5' không invalidate khi revoke.
- Không rate-limit login/refresh/register/forgot/reset/accept; `login_attempts` chỉ insert. Port 8080 publish trực tiếp bỏ qua nginx.
- Account pre-hijack: self-register tạo user `active` chưa verify email (`register_public.go:347`); invite email đã tồn tại → membership active ngay.
- ✔ Outbox reaper dùng `available_at` làm lease (`platform/outbox/mysql/repository.go:185`) → event backlog bị requeue khi đang xử lý → email trùng (≥2 worker).
- ✔ Reminder `SeedOccurrence` upsert ghi đè `status` (`reminder/infra/mysql/repository.go:282`) → SENT về PENDING → gửi lại.
- Reminder idempotency key chứa `recipient_hash`, không cận dưới `due_utc` → sửa recipients sinh loạt reminder quá hạn/trùng.
- SMTP không timeout/không ctx ở `notification/infra/smtp/adapter.go`, `binding_mailer.go`, `cmd/worker/main.go:392` (reminder đã có `boundedSendMail`) → treo request/worker.
- ✔ Holiday cache lưu cả lỗi vĩnh viễn (`holiday/infra/mysql/db_provider.go:57`) kể cả `context.Canceled` → deadline alerts/dashboard 500 đến khi restart.
- ✔ `workflow_instances` không unique `(company_id, record_id)`; `EnsureOnSubmit` check-then-insert + đường adhoc tạo instance thứ 2.
- RBAC/notification approval lost update: stale check ngoài tx, đọc bằng `r.db` trong tx, `_ = captureRBACMatrixVersion`.
- ✔ `TransferOwnership` 2 UPDATE rời, không tx/CAS → 2 hoặc 0 primary admin.

## MEDIUM
- Refresh rotation không CAS hash cũ, không reuse detection, expiry trượt vô hạn.
- Opaque access token không hết hạn + map in-memory tăng vô hạn (DEV dùng `ACCESS_TOKEN_MODE=opaque`).
- N+1 holiday query khi tính WORKING_DAYS (`HasCalendarForYear` không cache) — ứng viên 504 còn lại.
- Không có ctx timeout per-request/query; body JSON/multipart không `MaxBytesReader` (login, evidence, doctemplate, holiday upload).
- `_ = outbox.Publish` cho email auth; goroutine dùng request ctx (`iam/app/service.go:315,835`); 5xx không log cause/request_id.
- Lost update: `disclosure_records`, `cms_global_records`, company template lifecycle (UPDATE không guard status/version).
- Last-admin guard read-check-write; periodic cycle kẹt `CLAIMED` không reaper; auth email retry tổng ~3 phút rồi drop; event type chưa đăng ký handler bị mark processed; SIGTERM giữa batch gây gửi lại.
- Deadline alert confirm bỏ qua data-scope (`deadlinealerts/app/service.go:352`).
- `/metrics` lộ qua `deploy/nginx/cobo-api.example.conf` rewrite `/public/` (nếu dùng config này); so token bằng `==`.
- Holiday cache stale giữa replica (không TTL).
- ✔ `owner_only` data scope luôn đúng: `disclosure/app/service.go:188,286,317,363,410` gán `owner_membership_id = req.Subject.MembershipID` (của người gọi, không phải owner record) → member bị giới hạn phòng ban/assignment đọc/sửa được mọi record cùng tenant. Fix: lấy owner từ record.
- ✔ Prometheus cardinality bomb không cần auth: `portaldashboard/transport/http/handler.go:33-41` dùng query `range` làm label trước auth → series tăng vô hạn. Fix: chuẩn hoá label về tập cố định.
- Bổ sung LOW: `UpdateRecord` đổi `type_id` không validate template; `inappnotification MarkRead` thiếu `company_id`.

## LOW
Login timing enumeration; avatar signing secret default khi `ENV=development`; guard fail-closed dựa vào substring URL; token entropy UUIDv7; reset link/OTP plaintext trong `outbox_events`; `_ = Mark*` trong outbox processor; PII email trong log; goroutine không recover; `ListUsersWithNoMembership` không LIMIT; CMS write chỉ cần view permission; in-memory credential map không mutex (dev).

## Đã xem, không có vấn đề đáng kể
JWT validation, sessionbound revocation, reset/OTP token, select/switch company, template-builder OAuth (PKCE, redirect allowlist), CORS, CMS media upload signing, adhoc vote/approval locking, workflow task CAS, type version locking, reminder dispatch claim, binding email lease/fencing, DB pool config, graceful shutdown API, không có outbound HTTP client.

## Chưa rõ
Số replica API/worker; DEV `.env` có set `USER_AVATAR_UPLOAD_SIGNING_SECRET`; company_id production có đoán được; disclosure handler, personalops, portaldashboard, inappnotification, marketreference, subscription, wfhttp đã được review bổ sung (không có cross-tenant; xem 2 MEDIUM mới ở trên).
