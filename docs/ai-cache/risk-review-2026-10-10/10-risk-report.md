# Risk review 2026-10-10 (lần 3) - scope: all (cobo_iam_services `ff5ada2`, cobo_web_design `453a3494`)

Chỉ đọc, không sửa code. Phạm vi: `00-scope.md`.

7 reviewer chạy song song: fe-security, be-security, admin-role, perf-reliability, cache-versioning, api-compat, secrets-cors. Main agent đã đọc lại code cho mọi mục HIGH cũ còn mở và cho các MEDIUM mới mạnh nhất (ROLE-12, ROLE-13, BES-08/CACHE-13).

Bảng dưới chỉ đếm phát hiện **mới** ở lần 3. Trạng thái mục cũ xem phần "Trạng thái mục cũ".

| Group | Status | Critical | High | Medium | Low |
|---|---|---|---|---|---|
| Security Frontend | Issues (mục cũ còn mở) | 0 | 0 | 0 | 1 |
| Security Backend | Issues (ROLE-02/03, H1, H3-H6 còn mở) | 0 | 0 | 2 | 4 |
| Backend Performance & Reliability | Issues (PERF-10, H8-H16 còn mở) | 0 | 0 | 2 | 3 |
| Cache & Versioning | Issues (H17 sửa một phần, còn HIGH) | 0 | 0 | 2 | 1 |
| API Compatibility | Issues (H18/H19 còn mở) | 0 | 0 | 1 | 2 |
| CORS & Credentials | Issues (C1/C6/H20-H23 còn mở) | 0 | 0 | 0 | 3 |

Gộp trùng:
- **BES-08 = CACHE-13:** cùng gốc là proposal cũ không có `plan_digest`.
- **PERF-21 = CACHE-12:** invalidate cache chỉ là best-effort.
- **BES-11 = SEC-12:** DSN chứa password.
- **ROLE-21** gộp vào ROLE-13.

## Kết luận về bản sửa ROLE-01 (`10a5700`, `ff5ada2`)

**Đã giữ được, không tìm thấy đường vượt.** be-security và admin-role kiểm độc lập. Main agent đọc lại `rbac_restore_plan.go` và `config_approval.go`.

**Phạm vi role được sửa:**
- Rollback và apply approval đều đi qua `restoreRBACMatrixTx` (`infra/mysql/admin_repository_rbac_restore.go:119-169`).
- Plan chỉ chứa role `tenant_custom`, active, không protected của công ty (`app/rbac_restore_plan.go:253-265`).
- SQL lặp lại cùng điều kiện (`:24-46`): `r.company_id=? AND r.role_type='tenant_custom' AND r.is_protected=0`.
- Direct grant cũng ghim theo `company_id` (`:57-75`).

**Phạm vi quyền được thêm:**
- Thêm quyền phải qua `grantableOnCustomRole` (`rbac_restore_plan.go:132`), nên restore không cấp lại được `rbac.manage`/`system.settings`.
- Direct grant giới hạn trong `GrantablePermissions` trừ deny list.

**Cổng duyệt:**
- Rollback cần `rbac.manage` (`config_versioning.go:284`).
- Thay đổi critical được đưa vào hàng chờ duyệt (202), kể cả khi chỉ phát hiện critical bên trong transaction (`:287-294`).
- Plan digest được kiểm trước transaction và kiểm lại trong transaction khi đã giữ khoá (`config_approval.go:454-461`, `restore.go:159`).
- Chặn tự duyệt (`config_approval.go:418,539`).
- Chặn apply hai lần bằng `FOR UPDATE` cộng recheck status.

**Kiểm thử:**
- Unit/handler test của ROLE-01 pass. FE vitest approvals: 24/24.
- 17 integration test MySQL bị **skip** vì không có `MYSQL_TEST_DSN`.

**Rủi ro còn lại:**
- BES-08 (proposal cũ).
- API-13 (rollback BE).
- BES-07 (người duyệt thứ hai do chính người yêu cầu tạo ra).
- PERF-19 (deadlock).
- CACHE-11/PERF-21 (invalidate không chắc chắn).

## Critical / High (fix first)

**Không có CRITICAL/HIGH mới ở lần 3.** Các mục CRITICAL/HIGH dưới đây vẫn mở từ lần 1/2. Main agent đã đọc lại code ở HEAD `ff5ada2` để xác nhận:

### CRITICAL cũ còn mở (hạ tầng, cần người có quyền server)
- **C1 Seed account có mật khẩu đã biết trên DEV public.**
  - Smoke ROLE-01 ngày 2026-10-09 (`release-2026-10-09/smoke-role01-approval2/*.out`, "32/32 passed") đã đăng nhập thành công bằng seed account. Đây là bằng chứng mới cho thấy C1 vẫn còn hiệu lực.
  - `run_dev_migrations.sh` vẫn apply 0009/0031/0063.
- **C6 Mailpit publish ra internet.** `docker-compose.artifacts.yml:64-66` vẫn publish `8025:8025` và `1025:1025`.

### HIGH cũ còn mở
- **H17 + H3 (ROLE-06/CACHE-01) — sửa một phần. Ưu tiên cao nhất trong nhóm code.**
  - Đã có invalidate sau rollback, apply approval, `AssignCompanyAdmin`, `RevokeCompanyAdmin`, `TransferOwnership` (`admin_service.go:1032,1057,1081`; `config_approval.go:493`; `config_versioning.go:297`).
  - **Chưa** invalidate khi:
    - `AssignRole`/`RemoveRole` (`admin_service.go:1180-1188`) và đổi primary role;
    - thêm/bớt quyền role không critical;
    - thêm/bớt direct permission không qua approval;
    - `UpdateMembership` status, `DeleteMembership`;
    - đổi phòng ban/chức danh/team.
  - `ListPermissionCodes` (`authorization/infra/mysql/repository.go:20-40`) vẫn không lọc `m.membership_status` (main agent đã xác minh).
  - Hệ quả: khoá hoặc hạ quyền một tài khoản qua route admin thông thường vẫn để quyền cũ tồn tại tối đa 5 phút (TTL). Membership inactive vẫn còn quyền. Có thêm race CACHE-11 làm mất cả lần invalidate đã có.
- **ROLE-02:** `CreateResourceScopeRule`/`CreateWorkflowAssigneeRule` vẫn lấy `company_id` từ body (`admin_service.go:1354-1365`, `admin_repository.go:592-593`). Đây là ghi chéo tenant.
- **ROLE-03:** người chỉ có quyền mời vẫn cấp được role `admin_doanh_nghiep`/`company_admin`. `enterprise_invite_roles.go:6-14` không đổi.
- **PERF-10:** `ListMembershipsByCompany` chạy 3N+1 query, không LIMIT, nuốt lỗi (`admin_repository.go:184-214`). Hàm này nay còn nằm trên đường invalidate cache của ROLE-01 (PERF-21).
- **H1:** `UpsertDisclosureReminderConfig` không có `authorize`, không kiểm record thuộc company (`reminder/app/service.go:177-198`). Main agent đã xác minh.
- **H4** (không rate limit auth), **H5** (pre-hijack: self-registration tạo user `active`), **H6** (refresh rotation không CAS, `iam/infra/mysql/sessions.go:122-128`): code không đổi.
- **H8:** SMTP vẫn gọi `smtp.SendMail`, không có timeout (`notification/infra/smtp/adapter.go:42`, `binding_mailer.go:35`, `cmd/worker/main.go:392`).
- **H9:** outbox reaper vẫn dùng `available_at` làm lease (`platform/outbox/mysql/repository.go:190-192`). Main agent đã xác minh.
- **H10:** reminder upsert vẫn có `status = VALUES(status)` (`reminder/infra/mysql/repository.go:296`). Main agent đã xác minh.
- **H11-H16:** code không đổi.
- **H18:** 4 migration vẫn không có trong `run_dev_migrations.sh`: `0038_system_worker_membership`, `0098_adhoc_multi_reviewer`, `0099_workflow_step_instructions`, `0150_template_builder_oauth_authorization_codes`. Đã kiểm 148 file `*.up.sql`.
- **H19:** `make deploy-be` vẫn chạy `--no-deps api worker` mà không migrate (`Makefile:147-158`).
- **H20-H23:**
  - H20: MySQL 3306 vẫn public. SEC-12 tạo thêm 2 bản sao mật khẩu root.
  - H21: mật khẩu Gmail vẫn nằm trong 3 script web.
  - H22: `INTERNAL_REMINDER_TOKEN` vẫn nằm trong `deploy-artifacts/_tmp_seed_reminder_smoke.py:13` (plausible: chưa biết server còn dùng giá trị này không).
  - H23: `ENV: development`/`LOG_LEVEL: debug` nằm trong `environment:` của compose, nên `.env` trên server không ghi đè được.

## Medium / Low

### Security Backend
- **[M] ROLE-12 Rollback/xoá notification rule bỏ qua approval, validation và kiểm gói của `alert_channel_prefs`** — confirmed (main agent đã đọc `config_versioning.go:168-186`)
  - `RollbackNotificationRuleVersion` chỉ cần `rbac.manage` **hoặc** `system.settings`; quyền break-glass cũng đủ. Sau đó nó gọi thẳng `RestoreNotificationRuleFromSnapshot`.
  - Trong khi đó `UpdateNotificationRule` cho rule prefs bắt buộc validate và đi qua `submitNotificationPatchApproval`.
  - `DELETE /notification-rules/{id}` cũng xoá được rule prefs mà không cần duyệt.
  - Hậu quả: một admin đơn lẻ bật lại kênh premium sau khi công ty hạ gói, đồng thời vượt cơ chế 4 mắt.
  - Fix: snapshot có `RuleCode == AlertChannelPrefsRuleCode` thì validate + kiểm entitlement + đưa vào hàng chờ duyệt. Chặn hoặc đưa vào hàng chờ cả thao tác xoá rule prefs.
- **[M] ROLE-13 (gộp ROLE-21) Admin công ty của tenant chủ platform (c_001) gỡ được quyền `/cms` của CMS operator** — confirmed trong code; điều kiện trên DEV chưa xác minh
  - `AssignRole` có guard `assertRoleHasNoPlatformPermissions` cho người không phải operator (`admin_service.go:1159-1162`).
  - `RemoveRole` (`:1180-1188`), thay primary role (`admin_service_primary_role.go:58-63,94-97`) và `RemoveDirectPermission` cho `platform.cms.view` (risk "low", nên áp dụng ngay) thì **không có** guard này.
  - Tức là tenant admin không cấp được quyền platform nhưng lại gỡ được.
  - Fix:
    - Người gọi không phải `isPlatformCompanyOperator` thì cấm gỡ role mang quyền platform.
    - Cấm gỡ direct permission `platform.*`/`cms.*` và `EnterpriseDenyCodes`.
    - Áp dụng tương tự cho `UpdateMembership`/`DeleteMembership` khi target giữ quyền platform.
- **[L] BES-08 (= CACHE-13) Approval RBAC xếp hàng trước bản sửa vẫn được apply như restore toàn ma trận, không qua kiểm digest** — confirmed trong code (main agent đã đọc `config_approval.go:454`, `rbac_restore_plan.go:97-99`)
  - Row cũ có `Explicit=false` và `PlanDigest=""`. Khi duyệt, toàn bộ role `tenant_custom` và direct grant của công ty quay về snapshot lúc xếp hàng, với `AllowCritical: true`.
  - Phạm vi vẫn chỉ gồm `tenant_custom` của công ty, nên **không** tái hiện ROLE-01 trên role global. Nhưng các thay đổi xảy ra trong khoảng giữa sẽ bị hoàn tác (phần còn lại của H7).
  - Kiểm base-version không bắt được trường hợp này nếu `captureRBACMatrixVersion` từng lỗi im lặng.
  - Fix: từ chối approve row RBAC có `PlanDigest` rỗng (409 `STALE_PROPOSAL`), hoặc huỷ các row cũ. Cần đếm số row trên DEV trước.
- **[L] BES-07 Một người có `rbac.manage` tự tạo được người duyệt thứ hai**
  - Mời một email do mình kiểm soát, rồi `AssignCompanyAdmin` hoặc đi đường ROLE-03.
  - Cơ chế 4 mắt chỉ chống nhầm lẫn, không chống người trong.
  - Fix: ghi rõ giới hạn này. Tuỳ chọn: chặn hoặc cảnh báo khi người duyệt được nâng quyền sau thời điểm xếp hàng hoặc do chính người yêu cầu nâng.
- **[L] BES-09** Duyệt alert-channel prefs không kiểm lại entitlement của gói (`config_approval.go:426-437`), nên hạ gói sau khi xếp hàng vẫn áp dụng được premium.
- **[L] BES-10** Restore cấp lại direct grant cho membership inactive (`admin_repository_rbac_restore.go:66-75`, thiếu `membership_status='active'`). Do H3 nên các quyền này dùng được.
- **[I] ROLE-14** Target của break-glass được tự duyệt grant của chính mình (`config_break_glass.go:87-131`). Tác động gần như không có.
- **Ghi chú phòng thủ chiều sâu (admin-role):** repo `AddRolePermission`/`RemoveRolePermission` (`admin_repository.go:569-590`) không tự guard theo company hoặc `role_type`; chỉ service guard. Approver lấy quyền từ cache, không lọc `membership_status`, nên người duyệt vừa bị đình chỉ vẫn duyệt được trong TTL.

### Backend Performance & Reliability
- **[M] PERF-19 Thứ tự khoá ngược giữa apply/restore và `captureRBACMatrixVersion`, có thể deadlock (1213)**
  - Restore khoá `companies` rồi đến `rbac_matrix_snapshots`. Capture khoá `MAX(version_no) FOR UPDATE` rồi INSERT, và FK làm nó lấy S lock trên `companies`.
  - Capture bị kill thì version mất im lặng (`_ = s.captureRBACMatrixVersion`, `admin_service.go:1323,1351,1679,1704`; `rbac_custom_role.go:106,166,249`).
  - Apply bị kill thì trả 500, không retry (PERF-13).
  - Fix: `InsertRBACMatrixSnapshot` khoá `companies FOR UPDATE` trước. Không bỏ lỗi capture. Retry 1213/1205 một lần.
  - Confirmed theo code; chưa tái hiện vì không có MySQL test.
- **[M] PERF-20** Plan và snapshot được đọc lại 4-6 lần mỗi request, trong đó 2 lần nằm trong transaction đang giữ khoá công ty.
  - N+1 trên từng role, cả role global.
  - `ListRoles` đếm `role_permissions` của toàn platform (`admin_repository_rbac_read.go:58-63`).
  - Fix: tính plan một lần, đọc permission bằng một query join, giới hạn subquery đếm theo company.
- **[L] PERF-22** Xếp hàng approval theo kiểu check-then-insert, không có unique key (`admin_repository_approval.go:18-33`), nên double submit tạo 2 pending.
- **[L] PERF-23** Rollback trực tiếp capture version trong transaction riêng sau commit (`config_versioning.go:290-300`). Lỗi capture trả 500 dù restore đã áp dụng.
- **[L] PERF-24** (plausible) Range lock của restore chạm index row đầu tiên của công ty kế tiếp. `RemoveRolePermission` không serialize với restore.
- **[I] PERF-25** `go vet ./...` exit 1 do copylocks trong test có sẵn từ trước (`workflowfulfillment/required_document_gate_test.go:326-327`).
- **[I] PERF-18 (cũ)** vẫn có 2 test fail: `TestUpdateNotificationRule_TierEnforcement_FlagOffAllowsPremium`, `TestCreateSelfServiceCompany_FeatureFlagOff`. be-security thấy thêm `TestLoad_UserAvatarEnvOverride` fail, nhiều khả năng do biến môi trường local.

### Cache & Versioning
- **[M] CACHE-11 Race làm mất invalidation**
  - Một request miss cache đọc DB trước commit, rồi `SET` giá trị cũ sau khi lệnh `DEL` invalidate đã chạy (`projection/cached_resolver.go:19-28`, `redis_store.go:47-56`).
  - Người bị thu quyền giữ quyền cũ trọn 5 phút.
  - Fix: generation counter theo company (đưa vào key hoặc dùng CAS bằng Lua), hoặc chạy thêm một lần DEL trễ.
- **[M] CACHE-12 (= PERF-21) Invalidate là best-effort, im lặng, chạy trên request context**
  - Lỗi khi list member, lỗi `DEL` hoặc client ngắt kết nối thì bỏ qua (`config_versioning.go:98-111`), không log, không metric.
  - `DEL` chạy tuần tự từng member, cộng thêm 3N+1 của PERF-10.
  - Fix: dùng `context.WithoutCancel` kèm timeout, query chỉ lấy ID, pipeline DEL, log/metric khi lỗi. Có thể đưa qua outbox để retry.
- **[L] CACHE-14** FE không làm mới `permissions[]` của chính người duyệt sau approve/rollback (`useConfigApprovals.ts:62-76`). Chỉ ảnh hưởng UX.

### API Compatibility
- **[M] API-13 Rollback chỉ BE về bản trước ROLE-01 trong lúc còn approval RBAC pending sẽ làm ROLE-01 xuất hiện lại**
  - Binary cũ bỏ qua `explicit`/`role_revokes`/`plan_digest`, rồi chạy restore toàn ma trận không giới hạn phạm vi (`7a131a1:.../admin_repository_approval.go:177-178`). Restore này sửa được role global/default và thu hồi direct grant cấp sau thời điểm xếp hàng.
  - **Là điều kiện bắt buộc của release:** runbook rollback phải huỷ các approval `aggregate_type='rbac_matrix'` đang pending trước khi chạy BE cũ.
- **[L] API-14** Approve có thể trả 422 `DEFERRED_VALIDATION_BLOCKED` mà FE không dịch (`useConfigApprovals.ts:7-21`). `PENDING_APPROVAL_EXISTS` và 403 invite cũng vậy.
- **[L] API-15** Contract rollback/config-approvals mới chỉ có trong `api-contracts-json.md`. Thiếu ở `api-v1-implemented-contracts.json`, OpenAPI snapshot và Postman. Mục rollback cũng thiếu 409 `PENDING_APPROVAL_EXISTS`.
- **[I] API-16** Ma trận tương thích:

  | Kịch bản | Kết quả |
  |---|---|
  | FE cũ ↔ BE mới | OK, có giảm trải nghiệm |
  | FE mới ↔ BE cũ | OK; 403 hiển thị inline |
  | Worker | Không ảnh hưởng |
  | Migration | Không có migration mới |
  | Rollback chỉ BE | Rủi ro (API-13) |

### Security Frontend
- **[L] FES-15** Dialog duyệt không hiển thị plan thay đổi. Người duyệt có thể duyệt rollback critical mà chưa mở "So sánh" (`ApprovalsPanel.tsx:166-177,203-228`).
  - Fix: gọi `compare` ngay trong dialog duyệt và disable nút cho tới khi plan load xong và khác rỗng.
- **[I] FES-16** Client approvals không có refresh token và `onAuthFailure` (cùng kiểu FES-10). `approvalId` không qua `encodeURIComponent`. Mã lỗi lạ hiển thị nguyên message tiếng Anh; React đã escape nên không có XSS.

### CORS & Credentials
- **[L] SEC-11** 3 script smoke ROLE-01 mới gắn cứng IP DEV, SSH bằng `root` và đọc seed password từ migration (`release-2026-10-09/smoke-role01*/*.py`). Đây là phần mở rộng của SEC-07 và là bằng chứng C1 còn hiệu lực.
- **[L] SEC-12 (= BES-11)** Mật khẩu MySQL root của DEV (trùng với `docker-compose.artifacts.yml:39`) bị chép vào comment `rbac_restore_integration_test.go:8` và `bug-rbac-rollback-global-roles-2026-10-09/07-integration-tests.md:9-10`. Phải đưa vào danh sách rotate của H20.
- **[L] SEC-13** IAM đang track `.playwright-mcp/` (28 file, 6 file chứa seed password và IP DEV). `.gitignore` của IAM không có mục cho thư mục này, và `scan_secrets.py` bỏ qua nó (`SKIP_DIRS`), nên con số 479 bị đếm thiếu.
- Các hit mới khác của secret scan đều là tên biến hoặc định danh (false positive). Commit web `453a3494` không thêm secret.

## Trạng thái mục cũ (tóm tắt)

| Trạng thái | Mục |
|---|---|
| **Đã sửa** | ROLE-01; H2; ROLE-04 phần `requireRbacManage` |
| **Sửa một phần** | H17/CACHE-01; H7 (RBAC đã kiểm digest trong tx; còn BES-08 và lỗi capture bị nuốt); PERF-15 (đọc trong tx; còn PERF-20); ROLE-04 (`legacyPolicy` vẫn rơi về `system.settings`, RP-07); ROLE-11 (nút Cancel đã đúng) |
| **Còn mở, code không đổi** | C1, C6, H1, H3-H6, H8-H16, H18-H23; ROLE-02, 03, 05, 07, 08, 09, 10; BES-03..06; PERF-10..14, 16, 17; CACHE-04, 06, 08, 09, 10; API-07, 08, 09, 10, 11; FES-02, 05..14; SEC-05..10 |

## Quick wins (≤1h each)
1. **ROLE-02:** `payload["company_id"] = sub.CompanyID` ở hai service tạo rule, kèm test chéo tenant.
2. **BES-08:** từ chối approve row RBAC có `PlanDigest` rỗng (409 `STALE_PROPOSAL`).
3. **BES-10:** thêm `AND m.membership_status='active'` vào direct grant của restore.
4. **ROLE-13 (phần chính):** thêm `assertRoleHasNoPlatformPermissions` vào `RemoveRole`/thay primary role; cấm `platform.*`/`cms.*` trong `RemoveDirectPermission`.
5. **H18:** thêm 4 migration còn thiếu vào `run_dev_migrations.sh`.
6. **ROLE-14:** chặn khi `TargetMembershipID == approver`.
7. **API-14:** thêm text tiếng Việt cho `DEFERRED_VALIDATION_BLOCKED`, `PENDING_APPROVAL_EXISTS`.
8. **PERF-25:** sửa `go vet` (`deleteOK.Load()`).
9. **SEC-12:** đổi DSN trong comment/doc thành placeholder.
10. **SEC-08/SEC-13:** `.gitignore` thêm `__pycache__/` (web) và `.playwright-mcp/` (IAM), rồi `git rm --cached`.
11. **PERF-12:** trả lỗi khi lookup primary admin lỗi (`admin_service.go:905,920`).

## Decisions needed from the team
- **H17/H3 (cách xử lý thu hồi quyền):**
  - Phương án A: invalidate ở mọi mutation, cộng generation counter để chống CACHE-11.
  - Phương án B: đưa `membership_status` vào resolver và giảm TTL.
  - Kết hợp với quyết định xoá mềm membership (PERF-11) đang chờ từ lần 2.
- **BES-08/API-13:** đếm số approval `rbac_matrix` đang pending trên DEV. Chọn giữa từ chối row không có digest và huỷ hàng loạt. Đưa bước "huỷ approval RBAC pending" vào runbook rollback BE.
- **ROLE-03** (tập con quyền hay chỉ mở rộng deny list) và **BES-07** (có chặn người duyệt mới được nâng quyền không): chưa chốt.
- **ROLE-12:** rollback rule prefs có bắt buộc qua approval không? Khuyến nghị: có.
- **ROLE-13:** c_001 trên DEV có admin nào giữ `rbac.manage` mà không có `platform.cms.view` không? Nếu có, nâng ROLE-13 lên HIGH.
- **Hạ tầng (không đổi từ lần 1):** rotate credential (MySQL root/app, seed account, `INTERNAL_REMINDER_TOKEN`, Gmail, dev RSA key), rewrite history, `ENV` production, đóng port 3306/8025/1025.

## Not checked / BLOCKED
- **DB/server:** không truy vấn DB, không gọi server (theo chế độ read-only). Các mục cần dữ liệu:
  - số approval RBAC pending (BES-08/API-13);
  - điều kiện của ROLE-13;
  - API-08;
  - token mode thực tế (FES-08);
  - header runtime (FES-02/CACHE-09);
  - giá trị `INTERNAL_REMINDER_TOKEN` thực tế (H22).
- **MySQL integration test** (`rbac_restore_integration_test.go`, 17 test): BLOCKED do không có `MYSQL_TEST_DSN`. Deadlock PERF-19 chưa tái hiện.
- **Docker build** (`docker compose -f docker-compose.dev.yml build api`): không chạy, vì đây là review read-only, không có thay đổi code.
- **`govulncheck`:** BLOCKED (cần network).
- **`npm audit`:** 19 lỗ hổng (2 critical ở dependency không dùng: `protobufjs` qua `@google/genai`, `proxy-addr` qua `express`).
- **`dist/`:** không rebuild, chỉ quét bản build hiện có.
- **Đã chạy:**
  - `go vet ./...`: exit 1, chỉ do PERF-25.
  - `go test ./internal/companyaccess/... -race`: không có race; 2 fail có sẵn từ trước.
  - vitest approvals: 24/24.
- **Lịch sử git:** không quét toàn bộ.

## Suggested fix plan (grouped by repo and PR)
1. **iam PR-A (thu hồi quyền, ưu tiên 1):**
   - Thu hồi quyền phải có hiệu lực ngay: H17 (invalidate ở mọi mutation RBAC/membership) + H3 (lọc `membership_status` trong `ListPermissionCodes` và refresh).
   - Cache: CACHE-11 (generation counter) + CACHE-12/PERF-21 (`WithoutCancel`, chỉ lấy ID, pipeline, metric).
   - Thêm BES-10.
2. **iam PR-B (quyền admin):** ROLE-02 + ROLE-03 + ROLE-05 + ROLE-13 + ROLE-12 + BES-09 + ROLE-14 + RP-07 (`legacyPolicy`).
3. **iam PR-C (hậu kỳ ROLE-01):**
   - BES-08 (từ chối row không có digest).
   - PERF-19 (thứ tự khoá trong `InsertRBACMatrixSnapshot`, không bỏ lỗi capture, retry 1213/1205 = PERF-13).
   - PERF-23, PERF-22, PERF-20.
   - Chạy 17 integration test MySQL trên DB local.
4. **iam PR-D (perf/worker):** PERF-10, PERF-11/12, PERF-14, H8-H16, CACHE-08.
5. **iam PR-E (deploy/docs):**
   - H18 (4 migration), H19 (migrate trong `deploy-be`).
   - Đồng bộ API-15 (`api-postman-sync`), API-09.
   - Runbook rollback cho API-13.
   - SEC-12, SEC-13, PERF-25.
6. **web PR-F:** FES-15 (compare trong dialog duyệt), API-14, CACHE-14, FES-16, FES-07/09/10, API-10/11, SEC-08.
7. **Hạ tầng (cần người có quyền server):** C1, C6, H20-H23, rotate credential (gồm SEC-12). Không đổi so với lần 1/2.
