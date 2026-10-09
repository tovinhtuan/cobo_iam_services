# Risk review 2026-10-09 - all (cobo_iam_services @0609fdf + cobo_web_design @37a799db)

Chế độ read-only, không sửa code. Scope và secret scan xem `00-scope.md`. Kế thừa `../review-production-risk-full-2026-10-09.md`.

Có 7 reviewer chạy song song: be-security, admin-role, perf-reliability, cache-versioning, api-compat, secrets-cors, fe-security.

Mọi CRITICAL/HIGH đã được main agent đọc lại code. Đánh dấu:
- ✔ = confirmed.
- (P) = plausible, chưa xác minh trên server thật.

Không có secret value nào trong file này.

| Group | Status | Critical | High | Medium | Low |
|---|---|---|---|---|---|
| Security Frontend | Issues | 0 | 0 | 3 | 4 |
| Security Backend | Issues | 4 | 7 | 8 | 6 |
| Backend Performance & Reliability | Issues | 0 | 9 | 9 | 3 |
| Cache & Versioning | Issues | 0 | 1 | 3 | 1 |
| API Compatibility | Issues | 0 | 2 | 3 | 2 |
| CORS & Credentials | Issues | 2 | 4 | 3 | 3 |

## Critical / High (fix first)

### CRITICAL

**C1 ✔ Seed account có password đã biết (gồm cả platform CMS admin) được tạo trên server dev public** — CORS & Credentials (SEC-01)
- Location:
  - `docker-compose.artifacts.yml:83`: service `migrate` chạy `run_dev_migrations.sh`.
  - Script này áp dụng `0009_seed_authz_test_accounts` (dòng 33), `0031_seed_dev_invite_accept_fixture` (39), `0063_dev_platform_tenant_dual_admin` (70), v.v.
  - Password nằm trong comment của các file migration.
  - `deploy-artifacts/t5-auth-smoke*.json` cho thấy login vào server đã thành công.
- Scenario: bất kỳ ai đọc được repo đều đăng nhập được platform admin trên `88.***`.
- Fix:
  - Tách seed `*_seed_*` / `*_dev_*` ra một danh sách chỉ dùng local, hoặc gate bằng env flag (false trên server).
  - Disable hoặc reset các account này trên server.

**C2 ✔ Route `workflowconfig` chỉ kiểm tra token** — Security Backend
- **Trạng thái (2026-10-09): FIXED IN BRANCH (chưa commit).** Đã gate theo bậc quyền template CMS. Xem `../bug-workflowconfig-authz-2026-10-09/` (repro, verify, review: không còn đường vượt quyền).
- Location: `internal/workflowconfig/transport/http/version_handler.go:121` (`actor()`), route ở `:32-44`, `catalog_handler.go:31`.
- `WORKFLOW_VERSIONING_ENABLED: "true"` có trong `docker-compose.artifacts.yml:125`.
- Scenario: bất kỳ user nào đã đăng nhập cũng có thể `POST .../templates/{type_id}/workflow/publish` hoặc `.../versions/{n}/activate`, đổi workflow toàn cục của mọi tenant. Cũng tạo được assignee role toàn cục.
- Fix:
  - Gate các route write bằng `platform.cms.view` + `rbac.manage|system.settings`, giống `requireCMSEditor`.
  - Giữ `GET /platform/cms/workflow/assignee-roles` cho tenant, vì FE `tenantWorkflowRoleLabel.ts:6` đang dùng.

**C3 ✔ `POST /api/v1/platform/cms/admin/ops/adhoc-migrate-legacy-approvals` chỉ yêu cầu `rbac.manage`** — Security Backend
- **Hiệu chỉnh (2026-10-09):**
  - `"rbac.manage"` được truyền như *action code*. Do thiếu bảng `action_policy_matrix`, `legacyPolicy` fallback về **`system.settings`**.
  - Chủ công ty tự đăng ký **không** có quyền này. Role tenant demo `admin_doanh_nghiep` (0034) có, nếu dòng permission tồn tại.
  - Mức độ hiệu chỉnh: **HIGH (có điều kiện)**. Lỗi thiết kế cross-tenant và việc mạo danh process controller vẫn là thật.
- **Trạng thái: FIXED IN BRANCH (chưa commit).** Endpoint và code path cross-tenant đã bị gỡ (phương án A′). Xem `../bug-adhoc-legacy-migrate-authz-2026-10-09/`.
- Location: `adhoc/transport/http/handler.go:48,302`, `adhoc/app/service.go:1011,1045`, `adhoc/infra/mysql/repository.go:642` (query không lọc company).
- `rbac.manage` được cấp cho mọi chủ công ty tự đăng ký (`register_public.go:204`).
- Scenario: mọi chủ công ty tự đăng ký đều có thể tự động duyệt proposal `pending_admin_approval` của **mọi tenant**.
- Fix: yêu cầu `platform.cms.view` + `system.settings`, hoặc gỡ endpoint.

**C4 ✔ Chiếm quyền tenant khác: `CreateMembership` / `CreateUser`** — Security Backend
- **Trạng thái (2026-10-09): FIXED IN BRANCH (chưa commit, chưa deploy).** Gộp cùng H2. Xem `../bug-cross-tenant-membership-create-2026-10-09/`. Còn lại ở C5: mutation theo `membership_id`.
- `admin_handler.go:243-247`: `company_id` lấy từ body.
- `authorization/app/service.go:22-38`: `Authorize` bỏ qua `Resource.ID`.
- `admin_service.go:79,343,636,678,1063`: `isWebAdmin = hasPermission("rbac.manage")`.
- Kết hợp với `ensureRoleForMembership` (`admin_repository.go:889`), vốn chấp nhận global role.
- Scenario: owner của tenant A tự thêm mình vào tenant B với vai trò owner.
- Fix:
  - Lấy `company_id` từ token.
  - Thay tín hiệu "web admin" bằng `platform.cms.view`.

**C5 ✔ Sửa/xoá cấu trúc tổ chức của tenant khác theo ID** — Security Backend (gộp các finding cũ + BES-03, ROLE-04, ROLE-05)
- **Trạng thái (2026-10-09): FIXED IN BRANCH (chưa commit, chưa deploy).** Xem `../bug-membership-id-company-scope-2026-10-09/`.
- `admin_delegation_scope.go:125-136` trả nil cho company scope. Các SQL sau chỉ có `WHERE membership_id=?`:
  - `UpdateMembershipStatus` / `DeleteMembership` / `RemoveRole` / `AddTitle` / `RemoveTitle` (`admin_repository.go:125,137,375,527,536`).
  - `RevokeDirectPermission` / `ListActiveDirectPermissions` (`:839,851`).
- `DeleteTeamRow` (`admin_repository_teams.go:89-95`) xoá `org_unit_memberships WHERE org_unit_id=?` **trước** khi kiểm tra company, và không chạy trong transaction.
- `RemoveTeamMember` bỏ qua `companyID`; `RemoveTitleMember` không gọi `requireMembershipInCompany`.
- Scenario: admin tenant A xoá hoặc vô hiệu hoá nhân sự tenant B, gỡ role/title/team/direct-permission của B.
- Precondition: cần UUID của B. Có thể lấy qua H2 nếu biết company_id.
- Fix:
  - Gọi `requireMembershipInCompany(mid, sub.CompanyID)` (đã có ở `admin_service_departments.go:129`) ở đầu mọi mutation.
  - Thêm `AND company_id=?` vào SQL.
  - Đưa cascade delete vào transaction và thêm join company.

**C6 ✔ Mailpit publish `0.0.0.0:8025/1025` trên server IP public** — CORS & Credentials
- Location: `docker-compose.artifacts.yml:64-66`.
- Scenario: ai cũng đọc được email reset password / OTP / invite → chiếm tài khoản (khi server không override SMTP).
- Fix: bỏ `ports` hoặc bind vào `127.0.0.1`.

### HIGH — Security Backend

- **H1 ✔ Reminder config/history không có authz và không lọc company** (BES-01)
  - Location: `reminder/transport/http/handler.go:28-32,39-62` (chỉ gọi `subjectFromToken`), `reminder/app/service.go:177-215` (không có `authorize`), `reminder/infra/mysql/repository.go:42-71` (`WHERE scope_type=? AND scope_id=?`).
  - Scenario: bất kỳ user nào đã đăng nhập cũng có thể `PUT /api/v1/disclosures/{record_id}/reminder-config` với `recipients=["attacker@x"]`, khiến email nhắc hạn của tenant khác được gửi cho kẻ tấn công. Hai endpoint GET config/history cũng đọc được dữ liệu cross-tenant.
  - Precondition: cần biết `record_id`.
  - Fix:
    - Thêm `authorize` + kiểm tra `disclosure_records WHERE company_id=? AND record_id=?`.
    - Giới hạn recipients trong danh sách email membership của company.
- **H2 ✔ IDOR đọc `GET /api/v1/admin/companies/{company_id}/memberships`** (ROLE-01)
  - Location: `admin_handler.go:345` → `admin_service.go:1079-1087`.
  - Lộ email, role và trạng thái nhân sự của tenant khác.
  - `list_without_company` cho phép tenant owner liệt kê mọi user chưa có membership.
  - Fix: `cid != sub.CompanyID` → 403.
- **H3 ✔ Membership inactive hoặc bị xoá vẫn giữ quyền và vẫn refresh được token** (BES-02, ROLE-02)
  - Location: `authorization/infra/mysql/repository.go:19-38` (không lọc `membership_status`; nhánh direct permission không join `memberships`), `iam/app/service.go:412-434` (Refresh không kiểm tra membership).
  - `DeleteMembership` không xoá `membership_direct_permissions` và không revoke session.
  - Fix:
    - Lọc `m.membership_status='active'` ở cả hai nhánh.
    - Kiểm tra trạng thái membership trong Refresh/SelectCompany.
    - Revoke session khi deactivate/delete.
- **H4 ✔ Không có rate limit cho auth**
  - Áp dụng cho login, refresh, register, forgot, reset và accept-invite.
  - `login_attempts` chỉ được insert (`iam/infra/mysql/login_attempts.go:38`).
  - API 8080 publish thẳng, không qua nginx.
- **H5 ✔ Account pre-hijack**
  - Self-register tạo user `active` mà không verify email (`register_public.go:347,421`).
  - Invite tới email đã tồn tại → membership `active` ngay (`admin_service.go:482-505`).
- **H6 ✔ Refresh rotation không CAS, không reuse detection, expiry trượt vô hạn**
  - Location: `iam/infra/mysql/sessions.go:122-128`.
  - Opaque access token là mặc định (`config.go:243`, compose), không hết hạn, map in-memory tăng mãi, restart thì mất (`opaque/manager.go:29-58`).
- **H7 ✔ Lost update khi duyệt RBAC → cấp lại quyền đã thu hồi**
  - Stale check nằm ngoài transaction (`config_approval.go:53,355`).
  - `restoreRBACMatrixInTx` đọc qua `r.db` thay vì transaction (`admin_repository_approval.go:264-340`).
  - Kết quả `captureRBACMatrixVersion` bị bỏ qua: `_ = captureRBACMatrixVersion(...)` (`admin_service.go:1263,1291,1609,1631`).

### HIGH — Backend Performance & Reliability

- **H8 ✔ SMTP không có timeout và chạy đồng bộ trong request**
  - Location: `notification/infra/smtp/adapter.go:42`, `binding_mailer.go:35`, `cmd/worker/main.go:392`.
  - Request path: approve workflow (`workflow/infra/notification/notifier.go:107`) và adhoc (`adhoc/infra/notification/notifier.go:261`).
  - Nguyên nhân là `ADHOC_EMAIL_OUTBOX_ENABLED` mặc định false (`config.go:272`).
  - Scenario: SMTP treo → request treo → 504, cạn connection pool, worker đứng.
- **H9 ✔ Outbox reaper dùng `available_at` làm lease** (`platform/outbox/mysql/repository.go:185-192`) → gửi email trùng.
- **H10 ✔ Reminder `SeedOccurrence` upsert ghi đè `status`** (`reminder/infra/mysql/repository.go:282-297`) → bản ghi SENT quay về PENDING → gửi lại.
- **H11 ✔ Reminder bị gửi dồn khi sửa người nhận**
  - Idempotency key chứa `recipient_hash` và không có cận dưới cho `due_utc` (`reminder/infra/mysql/repository.go:328-388`).
- **H12 ✔ N+1 holiday query**
  - `HasCalendarForYear` không cache (`holiday/infra/mysql/composite.go:31`).
  - Được gọi cho từng ngày/dòng trong lúc cursor còn mở (`deadlinealerts/infra/mysql/repository.go:316-334`).
  - Đây là ứng viên chính cho 504 còn lại (commit `71e2fc3` chỉ sửa luồng không có filter).
- **H13 ✔ personalops "mine records" quét `disclosure_records` của mọi tenant, không có LIMIT** (PERF-01)
  - Location: `personalops/infra/mysql/repository.go:111-140`.
  - Kết quả vẫn đúng (lọc theo membership của chính user), nhưng chi phí tăng theo dữ liệu của toàn platform.
  - Fix: thêm `dr.company_id IN (...)` và giới hạn kết quả.
- **H14 ✔ `workflow_instances` chỉ có index `(company_id)`** (PERF-02)
  - Location: `migrations/0004_p1_business_tables.up.sql:27`.
  - Các query tương quan theo `record_id` trong deadlinealerts, personalops và reminder bị ảnh hưởng. Reminder history thậm chí không có `company_id` → full scan.
  - Fix: thêm migration `KEY (company_id, record_id, workflow_instance_id)`.
- **H15 ✔ `workflow_instances` thiếu unique `(company_id, record_id)`**
  - `EnsureOnSubmit` làm check-then-insert, và đường adhoc tạo instance thứ hai.
- **H16 ✔ `TransferOwnership` gồm 2 UPDATE rời, không có transaction/CAS** → có thể ra 0 hoặc 2 primary admin.

### HIGH — Cache & Versioning

- **H17 ✔ Effective-access cache (Redis, TTL 5') không được invalidate sau hầu hết thay đổi RBAC/membership** (CACHE-01, ROLE-03)
  - Chỉ gọi invalidate ở rollback và approval apply (`config_versioning.go:97-109`).
  - Fix: gọi `InvalidateMemberships` sau mọi mutation role/permission/membership/status.

### HIGH — API Compatibility

- **H18 ✔ 4 migration không có trong `run_dev_migrations.sh` nhưng code vẫn dùng** (API-01)
  - `0038_system_worker_membership`, `0098_adhoc_multi_reviewer`, `0099_workflow_step_instructions`, `0150_template_builder_oauth_authorization_codes`.
  - DB mới hoặc môi trường không được vá tay → lỗi "Unknown column / table", ảnh hưởng CMS workflow, reminder recipient và adhoc.
  - Ghi chú: trạng thái thực tế trên server dev chưa được kiểm tra.
- **H19 ✔ `make deploy-be` restart api + worker với `--no-deps` mà không chạy migrate** (API-02)
  - Location: `Makefile:147-158`, `deploy-dev.sh:336-341`.
  - Hậu quả: binary mới chạy trên schema cũ.

### HIGH — CORS & Credentials

- **H20 ✔ MySQL 3306 publish ra internet, root/app password yếu và đã commit**
  - Location: `docker-compose.artifacts.yml:37-41`.
  - Thêm nữa (SEC-02): `run_dev_migrations.sh:183-185` chạy `ALTER USER root/cobo ... BY '<committed>'` mỗi lần migrate → mọi lần rotate password đều bị âm thầm hoàn tác.
- **H21 ✔ Password thật của tài khoản Gmail cá nhân hardcode trong script web** (SEC-04, FES-01)
  - Location: `cobo_web_design/smoke-qa-create-type.mjs:4-6`, `scripts/cms-workflow-tab-debug.mjs`, `scripts/sidebar-toggle-verify.mjs`, cộng một số file docs/ai-cache.
  - Trỏ tới `http://88.***:3000`.
- **H22 (P) `INTERNAL_REMINDER_TOKEN` 64-hex bị commit trong `deploy-artifacts/_tmp_seed_reminder_smoke.py`** (SEC-03)
  - File được dùng với server. Chưa xác minh token trên server còn đúng giá trị này hay không.
- **H23 ✔ Server dev chạy với cấu hình development**
  - `ENV=development` + `LOG_LEVEL=debug` (`artifacts.yml:102-103,190`), API 8080 publish thẳng (`:147`), build không có `-tags prod`.
  - Hệ quả:
    - Avatar signing secret dùng giá trị mặc định `dev-…` (`config.go:280,519`).
    - Route `/internal/dev/reminders/seed-occurrence` được bật.
    - Adhoc bật mặc định.

## Medium / Low

### Security Backend
**MEDIUM**
- BES-04: PATCH với body `{}` trả về department/title/team của tenant khác (`admin_repository_departments.go:113-175`, `_titles.go:76`, `_teams.go:72`).
- `owner_only` data scope luôn đúng: `owner_membership_id` được gán bằng người gọi (`disclosure/app/service.go:188,286,317,363,410`).
- Data scope gần như bị tắt: mọi policy là `ScopeType:"*"` (`authorization/infra/mysql/repository.go:296-300`).
- BES-05: `UpdateRecord` không authorize phòng ban đích và không validate `type_id` (`disclosure/app/service.go:277-299`).
- ROLE-06: `AssignRole` bỏ qua denylist và lockout của primary-role (`admin_service.go:1109-1128`).
- Upload multipart và JSON body không có `MaxBytesReader`:
  - `workflowstepevidence/.../handler.go:161`
  - `workflowdoctemplate/.../handler.go:123`
  - `workflowfulfillment/.../handler.go:161`
  - `platformcms/holiday_handlers.go:51,97`
  - login
- XLSX import: không giới hạn `UnzipSizeLimit` hay số dòng (`holiday/app/parser.go:39`) → nguy cơ OOM.
- `/metrics`:
  - Tin mọi IP private (`server.go:972-1006`).
  - Label `range` trong portaldashboard gây cardinality bomb khi chưa auth (`portaldashboard/transport/http/handler.go:33-41`).

**LOW**
- BES-06: `evidence_link` không kiểm tra scheme.
- BES-07 / ROLE-07: membership `status` nhận chuỗi bất kỳ.
- BES-08: reset token bị tiêu thụ trước khi validate password; endpoint adhoc trả về `err.Error()`.
- BES-09: `dea4e59` bỏ kiểm tra deadline của template định kỳ khi `use_structure_deadline=false` (`applicability/validate.go:29`).
- Avatar không sniff nội dung file.
- Login có thể bị dò user qua timing.

### Backend Performance & Reliability
**MEDIUM**
- PERF-03: `GET /api/v1/disclosures` không phân trang và trả kèm `content` MEDIUMTEXT.
- PERF-04: SIGTERM hủy tick sau khi đã gửi SMTP → email trùng mỗi lần deploy.
- PERF-05: goroutine dùng request ctx, không có recover (`iam/app/service.go:315,835`, `reminder/app/service.go:378`).
- PERF-07: Redis dùng timeout mặc định, không có breaker, lỗi bị nuốt im lặng.
- Periodic cycle kẹt ở `CLAIMED`, không có reaper (`disclosure/infra/mysql/repository.go:2490`).
- Race giữa adhoc finalize và reject/cancel → record mồ côi (`adhoc/app/service.go:528-575`).
- Không có ctx timeout cho từng request.
- Lost update trên `disclosure_records`, `cms_global_records` và lifecycle template.
- Last-admin guard kiểu read-check-write.

**LOW**
- PERF-08: `LOWER(TRIM(status))` làm mất index.
- PERF-09: binding email chỉ gửi 1 email mỗi tick.
- PERF-10: test race của adhoc flaky (`close of closed channel`).

### Cache & Versioning
**MEDIUM**
- CACHE-02: holiday cache lưu lỗi vĩnh viễn, không có TTL, và stale giữa các replica (`holiday/infra/mysql/db_provider.go:57`).
- CACHE-04: Redis key không có version, TTL không có jitter, fallback in-memory chỉ chọn một lần lúc boot.
- CACHE-05 / PERF-06: cache preview workflow-override không bao giờ evict, và theo từng process.

**LOW**
- CACHE-06: deploy chạy `rm -rf dist` trước khi SCP; không có `vite:preloadError`; không có build SHA.

**INFO**
- CACHE-07: `location /api/` thiếu `^~`.

### API Compatibility
**MEDIUM**
- API-03: `push-migration.sh` luôn exit 0 kể cả khi migration lỗi.
- API-04: outbox mark processed các event type lạ, và `MarkFailedPermanent` các template key lạ (khi worker/API lệch version). Payload không có field `version`.
- API-05: FE không map các mã `IMPORT_ATTEMPT_*` (422) sang luồng re-validate.

**LOW**
- API-06: schema/fixture import của FE vẫn cho phép offset 0.
- API-07: FE gọi `/api/v1/admin/hub/summary` nhưng BE không có route này.

### Security Frontend
**MEDIUM**
- FES-02: không có CSP, `frame-ancestors`, nosniff hay HSTS; token nằm trong localStorage; trang OAuth consent có thể bị framing.
- FES-03: mỗi client refresh độc lập (~48 instance) → logout ngẫu nhiên do refresh rotation (P).
- CACHE-03: switch company ở tab này làm đổi tenant của các tab khác (không có listener `storage`).

**LOW**
- FES-04: còn raw `fetch` bỏ qua api client.
- FES-05: `href` do user nhập không qua `isSafeHttpUrl`.
- FES-06: `npm audit --omit=dev` báo 2 critical / 10 high, chủ yếu đến từ dependency không dùng (`@google/genai`, `express`).
- ROLE-08: alias quyền CMS ở FE lệch so với BE.

### CORS & Credentials
**MEDIUM**
- SEC-05: script nháp trong `deploy-artifacts/` (`t5-*`, `_tmp_*`) hardcode endpoint/credential. Riêng `t5-token-tmp.json` không được gitignore.
- RSA key dev `configs/login_password_rsa_dev.pem` bị commit và được mount lên server.
- `isLocalOrTestRuntime` dựa vào chuỗi "localhost" trong URL.

**LOW**
- SEC-06: còn `define GEMINI_API_KEY` chết trong vite config.
- `.dockerignore` thiếu `.env`/`.pem`; container chạy root.
- Worker log toàn bộ payload notification (`cmd/worker/main.go:84`).

## Quick wins (≤1h each)
1. Bỏ publish port Mailpit và MySQL trong `docker-compose.artifacts.yml` (C6, H20).
2. Đặt `WORKFLOW_VERSIONING_ENABLED=false` trên server cho tới khi vá C2. Vá C2 = thêm permission check vào `actor()` cho các route write.
3. C3: thêm `requireCMSAccess` cho endpoint adhoc-migrate, hoặc unregister.
4. H2: kiểm tra `cid == sub.CompanyID`.
5. H1: thêm `authorize` + kiểm tra record thuộc company ở 5 handler reminder.
6. H3: thêm `AND m.membership_status='active'` vào `ListPermissionCodes`.
7. H18: thêm 4 migration vào `MIGRATIONS` (0099 cần preflight `column_exists`).
8. H19: thêm bước migrate vào `deploy-be`. API-03: thêm `exit 1` khi migration lỗi.
9. Đặt `ADHOC_EMAIL_OUTBOX_ENABLED=true` trên server (giảm H8 trên request path).
10. Gitignore `deploy-artifacts/_tmp_*`, `t5-*`, `*token*.json`; xoá credential khỏi script web.

## Decisions needed from the team
- **Rotate credential** (cần người có quyền server): mật khẩu tài khoản Gmail cá nhân trong script web; DB root/app/vnstock (sau khi gỡ `ALTER USER`); `INTERNAL_REMINDER_TOKEN`; RSA login key; signing secret của avatar/CMS media; vô hiệu hoá các seed account trên server.
- **Có rewrite git history** (filter-repo) để gỡ password/token đã commit hay không? Việc này cần force-push.
- **Server dev có dữ liệu/người dùng thật không?** Nếu có, coi nó như production: `ENV=staging`, build `-tags prod`, chỉ publish nginx.
- **Access token mode** cho môi trường deploy: `jwt`/`dual` hay opaque có TTL?
- **Bật data scope thật** (thay `ScopeType "*"`): nhiều finding Medium (owner_only, BES-05) sẽ trở thành exploitable khi bật.
- **`rbac.manage` của chủ công ty tự đăng ký**: giữ quyền này nhưng đổi tín hiệu "web admin", hay thu hẹp quyền?
- **Evidence/fulfillment**: có bắt buộc người thao tác phải là assignee của step không? (hiện chỉ cần `deadline.confirm`).

## Not checked / BLOCKED
- BLOCKED: `govulncheck` (chưa cài, cần mạng).
- BLOCKED: trạng thái thực tế trên server dev: seed account/token còn hiệu lực không, 4 migration đã được vá tay chưa, nội dung `.env`. Lý do: không gọi tới môi trường dùng chung.
- BLOCKED: `check_http_headers.sh` (không có server local chạy); EXPLAIN / row count (không truy cập DB).
- Chưa trace hết từ đầu đến cuối các route `/api/v1/admin` delegations, config-approvals, emergency-access, notification-rules và `disclosure-types/{type_id}`.
- Chưa so khớp `platformcms/handler.go` theo từng route với ngữ nghĩa thao tác trên tenant (mới chỉ xác nhận gate `platform.cms.view`).
- Lệnh đã chạy:
  - `go build ./...` PASS.
  - `go vet ./...` FAIL, chỉ ở file test `workflowfulfillment/required_document_gate_test.go:326-327`.
  - `go test -race` cho reminder/outbox/adhoc: không có data race; có 1 lỗi flaky (PERF-10).
  - `npm audit --omit=dev` (web) đã chạy.
  - Chưa chạy lại toàn bộ `go test ./...` (review sáng nay: 8 test fail ở 4 package).

## Suggested fix plan (grouped by repo and PR)

**Ops / server (không cần code, làm ngay)**
- Gỡ port Mailpit/MySQL, tắt versioning flag, bật outbox email, rotate credential, disable seed account.

**cobo_iam_services**
1. PR `sec/tenant-isolation-admin`: C4, C5, H2, BES-04, ROLE-06, ROLE-07. Kèm test cross-tenant cho từng endpoint `/api/v1/admin`.
2. PR `sec/platform-route-gates`: C2, C3. Kèm test tenant admin gọi route platform → 403.
3. PR `sec/reminder-authz`: H1. Thêm `company_id` vào config/occurrence.
4. PR `sec/membership-lifecycle`: H3, H17, H6 (CAS + reuse detection + max lifetime).
5. PR `sec/auth-abuse`: H4 (Redis limiter), H5 (pending verification).
6. PR `ops/deploy-safety`: C1 (tách seed), H18, H19, API-03, SEC-02 (gỡ `ALTER USER`), H23, gitignore scratch files.
7. PR `rel/email-reliability`: H8 (`boundedSendMail` cho mọi adapter), H9, H10, H11, PERF-04, API-04.
8. PR `perf/deadline-hotpath`: H12, H13, H14 (migration index), PERF-03 (cần phối hợp FE).
9. PR `data/integrity`: H7, H15, H16, periodic CLAIMED reaper.
10. PR `hardening`: các Medium/Low còn lại (body limits, XLSX limits, `/metrics`, cache TTL/jitter).

**cobo_web_design**
1. PR `sec/remove-hardcoded-creds`: H21, SEC-05 phía web.
2. PR `sec/headers-and-session`: FES-02 (CSP/headers ở nginx, phối hợp file `deploy-artifacts/web/nginx.conf` của IAM), FES-03 (refresh singleton), CACHE-03 (listener `storage`).
3. PR `chore/deps`: FES-06, SEC-06.
4. PR `fix/contract-drift`: API-05, API-06, API-07, ROLE-08, FES-04, FES-05.
