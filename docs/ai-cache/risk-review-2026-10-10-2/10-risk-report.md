# Risk review 2026-10-10 (lần 4) - scope: all (cobo_iam_services `1479d72` → `a8acb51`, cobo_web_design `453a3494`)

Chỉ đọc, không sửa code. Phạm vi: `00-scope.md`.

## Cách chạy
- 7 reviewer (fe-security, be-security, admin-role, perf-reliability, cache-versioning, api-compat, secrets-cors) chạy song song. Subagent của project không nạp được trong phiên này, nên mỗi reviewer chạy dưới dạng agent `general-purpose` với đúng file định nghĩa `.claude/agents/<name>.md`.
- **cache-versioning bị dừng giữa chừng.** Main agent tự kiểm phần còn thiếu (xem mục Cache & Versioning).
- **HEAD đổi trong lúc review.** Có thêm `c43b9cc` (company bị deactivate thì chặn member ngay), `5715d6c` (company suspended chỉ được đọc) và `a8acb51` (commit `scripts/devqa/**`, các smoke ngày 2026-10-10 và cả `00-scope.md` + `01-secret-scan-*.txt` của thư mục này).
  - Phần lớn reviewer đọc code ở `1479d72`.
  - Main agent đã đọc lại diff của `c43b9cc` cho BES-29.
  - secrets-cors đã kiểm `c43b9cc`/`5715d6c` về CORS/secret.
  - `5715d6c` **chưa** được review bảo mật/API đầy đủ (xem mục Not checked).
- Main agent đã đọc lại code cho mọi mục HIGH (mới và cũ còn mở) và cho các MEDIUM mạnh nhất: ROLE-29, ROLE-30, BES-30.

Bảng dưới chỉ đếm phát hiện **mới** ở lần 4, sau khi gộp trùng. Trạng thái mục cũ xem phần "Trạng thái mục cũ".

| Group | Status | Critical | High | Medium | Low |
|---|---|---|---|---|---|
| Security Frontend | Issues (FES-02, 05..16 còn mở) | 0 | 0 | 1 | 1 |
| Security Backend | Issues (H1, H4-H6, BES-03..08 còn mở) | 0 | 1 (BES-29, **đã sửa ở `c43b9cc`**) | 4 | 6 |
| Backend Performance & Reliability | Issues (PERF-10 nặng hơn; H8-H15 còn mở) | 0 | 0 | 1 | 3 |
| Cache & Versioning | Kiểm một phần (reviewer bị dừng) | 0 | 0 | 0 | 0 |
| API Compatibility | Issues (H18/H19 còn mở) | 0 | 0 | 2 | 3 |
| CORS & Credentials | Issues (C1, C6, H20-H23 còn mở) | 0 | 0 | 0 | 7 |

Gộp trùng:
- **FES-18 = API-27:** 202 "chờ duyệt" bị hiển thị như đã gỡ xong.
- **FES-17 = ROLE-33 = API-31:** alias CMS ở FE rộng hơn BE.
- **ROLE-32 = API-30:** approve không kiểm lại ROLE-20.
- **FES-19** nằm trong API-14/API-29/CACHE-14.
- **BES-32, ROLE-31, ROLE-32** cùng một nhóm: các đường còn lọt khỏi guard ROLE-20.

## Kết luận về các bản sửa từ lần 3 (PR-A, PR-B, vòng ROLE #1-#11)

**Phần lớn giữ được.** be-security và admin-role kiểm độc lập và không tìm thấy đường vượt cho: ROLE-02, ROLE-03 (`assertCanGrant` ở đủ 6 đường cấp role), ROLE-05/23 (khoá `GET_LOCK` ở 5 đường gỡ admin + approval RBAC), ROLE-10, ROLE-14, ROLE-21, ROLE-24 (cho `admin.*`), ROLE-25, RP-08 (một tập critical dùng chung cho direct remove, role remove và restore), RP-10 (transfer 1 transaction + CAS).

PR-A cũng giữ được:
- Resolver trả quyền rỗng cho membership không active.
- Refresh bị chặn khi membership không còn active.
- Cache theo generation đúng thiết kế: `Put` trễ sau invalidate không bao giờ được đọc lại; key generation mất thì không cache.
- perf và be-security đều xác nhận; `go test -race` sạch.

**Lỗ hổng còn lại hoặc do bản sửa sinh ra:**
- **ROLE-30.** PR-A làm effective access của member inactive rỗng, nên guard "chỉ operator mới được xoá member giữ quyền platform" bị lọt khi target đang inactive. Đường kích hoạt lại cũng không có guard.
- **ROLE-29.** ROLE-12 chỉ chặn đúng `rule_code` viết thường. DB so khớp không phân biệt hoa/thường.
- **ROLE-31, ROLE-32, BES-32.** ROLE-20 mới phủ các đường ở cấp member; còn sót approve direct revoke, route `/admin/company/admins` và gỡ quyền ở cấp role.
- **PERF-33.** Khoá admin mới bao trọn vòng 3N+1 query của PERF-10.

## Critical / High (fix first)

### HIGH mới
- **[HIGH] BES-29 Company bị platform deactivate nhưng user của company vẫn giữ toàn quyền.** Confirmed ở `1479d72` (binary đang chạy trên DEV lúc bắt đầu review). **Đã sửa ở `c43b9cc`** (nằm trong HEAD `a8acb51`).
  - Ở `1479d72`:
    - `POST /platform/cms/admin/companies/{id}/deactivate` chỉ chạy `UPDATE companies SET status = ?` (`companyaccess/infra/mysql/admin_repository_companies.go:353`).
    - `companies.status` không được đọc ở login, select/switch company, refresh, `/me/*companies`, token inspect hay resolver. Main agent đã đọc `git show 1479d72:internal/companyaccess/infra/mysql/membership_query.go` (không select `c.status`) và `authorization/infra/mysql/repository.go:25-36`.
    - Hệ quả: admin của company bị khoá vẫn gọi được mọi API, kể cả admin API, không có hạn.
  - Bản sửa `c43b9cc`, main agent đã đọc diff:
    - `sessionbound.Manager` có thêm `WithCompanyStatus`, kiểm `companies.status` ở mỗi lần inspect access token và trả 403 `COMPANY_INACTIVE`.
    - Áp dụng cho mọi token mode (opaque/jwt/dual), vì sessionbound bọc ngoài cùng khi có MySQL (`httpserver/token_builder.go`, `server.go:236,256`).
    - Membership query giờ trả `CompanyStatus`.
    - Smoke DEV `release-2026-10-10/smoke-company-inactive/*.out`: **18/18 pass**.
  - Rủi ro còn lại:
    - Thêm 1 query PK `SELECT status FROM companies` cho mỗi request (INFO, xem PERF-16/17).
    - Dòng thời gian deploy/rollback của `c43b9cc` chưa được ghi vào `10-role-loop.md`.

### HIGH cũ còn mở (main agent đã đọc lại code ở HEAD)
- **H1 Cấu hình nhắc hạn đọc/ghi chéo tenant. Phạm vi rộng hơn đã ghi; ưu tiên số 1 trong nhóm code.**
  - Cả 5 route `PUT/GET /api/v1/disclosures/{id}/reminder-config`, `.../workflow/{stepId}/reminder-config`, `GET .../reminders/history` (`reminder/transport/http/handler.go:28-32`):
    - đều chỉ cần một access token hợp lệ;
    - service không gọi `authorize`, không kiểm record thuộc company (`reminder/app/service.go:177-270`);
    - repository lọc theo `scope_id`, không có điều kiện company (`reminder/infra/mysql/repository.go:42-72,107-113`).
  - Điều kiện tấn công: biết `disclosure_id` của tenant khác. ID là UUIDv7 (`platform/idgen/idgen.go:16`), không đoán được, nên giữ mức HIGH thay vì CRITICAL.
  - Kết hợp với BES-30 (HTML injection trong email) và BES-34 (lộ danh sách người nhận trong `To:`):
    - người ngoài thêm địa chỉ của mình vào config, rồi nhận được toàn bộ danh sách người nhận của tenant nạn nhân;
    - người ngoài cũng gửi được email lừa đảo mang danh Cobo.
- **H6:** rotate refresh vẫn không CAS trên hash cũ (`iam/infra/mysql/sessions.go:122-128`).
- **H4** (không rate limit auth), **H5** (pre-hijack khi tự đăng ký): code không đổi.
- **H8** (SMTP không timeout), **H9** (reaper outbox dùng `available_at` làm lease, `platform/outbox/mysql/repository.go:188-192`), **H10** (`status = VALUES(status)`, `reminder/infra/mysql/repository.go:296`), **H11-H15:** không đổi.
- **H18:** `0038`, `0098`, `0099`, `0150` vẫn thiếu trong `migrations/run_dev_migrations.sh`. **H19:** `make deploy-be` vẫn `--no-deps api worker`, không migrate (`Makefile:147-158`).
- **C1, C6, H20, H22, H23:** không đổi (xem CORS & Credentials). **H21:** sửa một phần.

## Medium / Low

### Security Backend
- **[M] ROLE-29 Lách approval và kiểm gói của alert-channel prefs bằng cách đổi hoa/thường của `rule_code`.** Confirmed: main agent đã đọc code và collation.
  - Go so sánh chính xác: `strings.TrimSpace(code) == AlertChannelPrefsRuleCode` (`companyaccess/app/admin_service.go:1504`), và tương tự ở PATCH `:1615`, DELETE `:1696`, rollback `config_versioning.go:206`.
  - Bảng `notification_rules` dùng `utf8mb4_unicode_ci` (`migrations/0006_admin_rules_tables.up.sql:28`). Reader runtime lọc `rule_code = ?` (`reminder/infra/mysql/notification_rules_reader.go:37-41`), nên vẫn đọc ra row viết hoa như prefs của company.
  - Hệ quả: một admin có `admin.notification_rule.create` tạo `COMPANY.ALERT_CHANNEL_PREFS.V1` chứa kênh premium mà không qua validate/entitlement. Sau đó PATCH, DELETE và rollback rule này đều không qua approval.
  - Fix:
    - chuẩn hoá `rule_code` (lowercase + trim) khi create/insert;
    - so sánh bằng `strings.EqualFold` ở mọi nơi;
    - tuỳ chọn: chặn code chỉ khác hoa/thường.
- **[M] ROLE-30 Admin tenant không phải operator kích hoạt lại hoặc xoá được membership inactive của platform operator.** Confirmed: main agent đã đọc code.
  - `UpdateMembership` với `status=active` ghi thẳng (`admin_service.go:929-931`), không có guard ROLE-13. Đây là bản đối xứng phía cấp quyền của ROLE-13.
  - `assertMayDeactivatePlatformMember` đọc `GetEffectiveAccess` (`admin_service_company_scope.go:242-251`). Sau PR-A, giá trị này rỗng với member inactive, nên xoá luôn pass.
  - Gốc: ROLE-07 chưa là một nguồn duy nhất. Target được đánh giá theo effective access ở đây, nhưng theo grant đã lưu ở `isOperatorPermissionSet`.
  - Fix: đánh giá target từ role + direct grant đã lưu (`membershipPermissionSet`) bất kể trạng thái; áp dụng cả khi kích hoạt lại, trong `withCompanyAdminLock`.
- **[M] BES-30 Email HTML được render bằng `text/template`.** Confirmed: main agent đã đọc `notification/app/email_renderer.go:7,24`.
  - Tiêu đề, nội dung và lý do từ chối do member nhập được chèn nguyên vào `body.html`, ví dụ `{{.proposal_title}}` trong `templates/adhoc.proposal_rejected/vi/body.html:10-14`.
  - Hệ quả: link/HTML lừa đảo được gửi từ địa chỉ SMTP của Cobo.
  - Fix: render `body.html` bằng `html/template`; giới hạn người nhận reminder trong email của member active của company.
- **[M] BES-31 `/internal/v1/authorize/batch` nhận số check không giới hạn, mỗi check một query DB** (`authorization/transport/http/handler.go:25-26,58`; `authorization/app/service.go:48-58`).
  - Không có `MaxBytesReader`, không có deadline. Port :8080 public trực tiếp, nên vài request lớn chạy song song là chiếm hết pool 25 connection của mọi tenant.
  - Fix: `MaxBytesReader` ~64 KiB; giới hạn `len(Checks)` khoảng 50; cache policy theo action trong batch; không publish `/internal/*`.
- **[L] ROLE-31** `/admin/company/admins` assign/revoke bỏ qua guard ROLE-13/ROLE-20 (`admin_service.go:1076-1143`).
  - Plausible: chưa biết company nào trên DEV còn role `company_admin` active. Với tenant không có role này, route trả 500.
  - Fix: chuyển sang `AssignRole`/`RemoveRole`, hoặc bỏ route.
- **[L] ROLE-32 (= API-30)** Approve direct revoke không kiểm lại `assertKeepsPlatformOperator` (`config_approval.go:477-483`).
  - Hệ quả: row xếp hàng khi target chưa là operator, hoặc row xếp hàng trong cửa sổ rollback BE, khi duyệt sẽ làm member mất tư cách operator.
- **[L] BES-32** Gỡ quyền khỏi một role (`admin_service.go:1442-1471`, approval apply, rollback) không tính lại tư cách operator của các holder.
  - Plausible: chưa kiểm dữ liệu role tenant_custom của c_001.
  - Fix: theo mẫu `assertRBACPlanKeepsAdmin`.
- **[L] ROLE-34** Phép đếm "còn admin" tính cả member có `account_status=invited` (`rbac_role_assignability.go:81-99`). Company có thể còn lại một admin không đăng nhập được. Fix: chỉ đếm `AccountStatus=active`.
- **[L] BES-33 (mở rộng BES-17)** Deactivate member không thu hồi session.
  - Opaque token không hết hạn; session sống tới 720h.
  - Các đường chỉ kiểm danh tính vẫn nhận token này, ví dụ `adhoc/app/service.go:611-623` (`admin-approve` của process controller) và H1.
  - Fix: revoke session của user trong company khi deactivate/delete, hoặc kiểm membership status ở `sessionbound` (cùng chỗ với bản sửa BES-29).
- **[L] BES-34** Email reminder đưa mọi người nhận vào một header `To:` (`reminder/infra/email/smtp_sender.go:97`). Adhoc notifier log email người nhận (`adhoc/infra/notification/notifier.go:268-271`).
- **[I] BES-35** Action không phải `admin.*`, không có case và không có dòng matrix vẫn rơi về `system.settings` (`authorization/infra/mysql/repository.go:186-194,221-235`). Chỉ `/internal/v1/authorize[/batch]` với action tuỳ ý chạm tới được, và route này bind theo token. Gate CMS vẫn nhận `platform.cms.view` đơn lẻ; an toàn chừng nào quyền này còn `GrantTierSystemOnly`.

### Backend Performance & Reliability
- **[M] PERF-33 Khoá admin bao trọn vòng query theo từng member.**
  - Deactivate, delete, demote, approve RBAC chạy `ListMembershipsByCompany` (3N+1 query, nuốt lỗi) cộng `isMembershipAdminCapable` cho từng member, trong lúc giữ `GET_LOCK` (`rbac_role_assignability.go:81-223`, `admin_service.go:933-976`, `admin_service_primary_role.go:74-131`, `config_approval.go:468-532`).
  - N≈300 nghĩa là khoảng 1,5-3 nghìn round trip; mọi thay đổi admin khác của company chờ 5s rồi nhận 409.
  - Request vượt WriteTimeout 15s thì handler vẫn chạy tiếp và giữ khoá.
  - Fix: một query đếm tổng hợp; một query lấy mọi holder `rbac.manage` rồi áp plan trong bộ nhớ; nhả khoá ngay sau `ApplyPendingApprovalInTx`.
- **[L] PERF-34** Connection `GET_LOCK` giữ một slot của pool 25, trong khi phần việc cần thêm slot thứ hai; request không có deadline. Không tự deadlock trên khoá, nhưng 25+ người giữ khoá ở các company khác nhau sẽ làm treo API. Fix: semaphore, giảm thời gian chờ, hoặc `SELECT … FOR UPDATE` trên `companies` trong một transaction.
- **[L] PERF-35** Thay primary role chạy `RemoveRole` rồi `AddRole` thành 2 lần ghi autocommit trên request context (`admin_service_primary_role.go:109-129`).
  - Nếu `AddRole` lỗi (role R vừa bị inactivate, hoặc client ngắt kết nối), member mất role mà không qua kiểm "còn admin".
  - Fix: gộp vào một transaction, hoặc `AddRole` trước.
- **[L] PERF-36** `AssignCompanyAdmin` đếm giới hạn 5 admin rồi mới insert, không có khoá (`admin_service.go:1083-1094`).
- **[I] PERF-37** Store effective-access in-memory không giới hạn kích thước; invalidate quét toàn map dưới write lock. Khi fallback (PERF-32) với nhiều replica, mỗi replica có cache riêng.

### Cache & Versioning (kiểm một phần)
reviewer bị dừng giữa chừng. Phần lõi và phần còn thiếu được kiểm như sau.
- **Do perf/be-security kiểm:** thiết kế generation đúng; `InvalidateCompany` O(1) với `WithoutCancel` + 2s + log. CACHE-11/12 và PERF-21 đã sửa.
- **Do main agent kiểm:**
  - Mọi route ghi quyền (kể cả platform CMS `AssignUserToCompany`) đều đi qua companyaccess, và companyaccess có 43 chỗ gọi invalidate.
  - Ở HEAD không có code nào ghi vào bảng `assignments`, nên follow-up "assignments ngoài phạm vi PR-A" thực tế chưa có đường ghi nào.
  - nginx web: `index.html` có `no-store`, `.js/.css` có `immutable` (`deploy-artifacts/web/nginx.conf:79-97`), đúng.
- **[I] CACHE-21** Kích hoạt membership sau khi xác minh email (`iam/infra/mysql/auth_recovery.go:192`) không invalidate. Đây là fail-closed: user mới có thể thiếu quyền tối đa bằng TTL. Không có rủi ro bảo mật.

### API Compatibility
Tổ hợp đang chạy trên DEV là FE cũ ↔ BE mới. Không có thay đổi shape, enum hay envelope; chỉ thêm status/error code.
- **[M] API-24 FE cũ nuốt lỗi deactivate/delete member.** `MembersTable.tsx:19,21,217,251` bỏ qua `AppError` trả về.
  - BE mới trả 409 last-admin, 409 lock timeout và 403 BES-21 trên các route này.
  - Admin không thấy thông báo gì. Với 403, cả app chuyển sang `/app/forbidden` vì thiếu `suppressForbiddenNavigation`.
- **[M] API-25 Tài liệu contract không được cập nhật bởi 10/11 commit của vòng ROLE; mã lỗi lệch hoa/thường.**
  - Doc ghi `LAST_ADMIN_ROLE_CHANGE_BLOCKED`, nhưng code gửi `last_admin_role_change_blocked` (`platform/errors/errors.go:93`).
  - Chưa có trong contract: status allowlist ROLE-09, 409 last-admin ở deactivate/delete/approval, 202 của RP-08, 403 ROLE-20, ROLE-08, transfer ownership, ROLE-10, ROLE-24.
  - Fix: chạy `api-postman-sync`.
- **[L] API-26** Transfer ownership: picker FE vẫn cho chọn người không phải admin hoặc chính mình. BE trả 409 `STATE_CONFLICT` với "TARGET_NOT_ADMIN…" nằm trong message.
- **[L] API-28** Lock timeout (có thể retry) và vi phạm bất biến (vĩnh viễn) dùng chung 409 `STATE_CONFLICT` (`admin_repository_admin_lock.go:43-46`). Fix: mã riêng `ADMIN_CHANGE_IN_PROGRESS` + `Retry-After`.
- **[L] API-29 (cùng gốc FES-10)** User bị deactivate thấy trang forbidden thay vì bị logout. Client của các feature không có `onAuthFailure`.
- **[I] API-32** Runbook rollback:
  - 11 rollback point dùng cùng format cache v2 và cùng schema, nên an toàn về dữ liệu; mỗi lần rollback chỉ mở lại lỗi mà chính bản sửa đó đã đóng.
  - Rollback về trước PR-A cần flush cache v1 và rà approval.
  - Các bước này chỉ có trong release notes, chưa có trong skill `deploy-dev-release`/`wf-release`.

Ma trận tương thích:

| Kịch bản | Kết quả |
|---|---|
| FE cũ ↔ BE mới (đang chạy) | OK, giảm trải nghiệm (API-24, 26, 29; FES-17, FES-18) |
| FE mới ↔ BE cũ | Không áp dụng (web không có commit mới) |
| Worker ↔ API | OK. Worker không dùng cache authz hay `GET_LOCK`; payload outbox không đổi |
| Migration | Ở `1479d72`: không có migration mới từ lần 3. `5715d6c` thêm `0152_companies_status_suspended` (đã có trong `run_dev_migrations.sh:155`; chưa review mixed-version). H18/H19 còn mở |
| Rollback BE (11 điểm) | An toàn dữ liệu; mở lại lỗi tương ứng; xem ROLE-32/API-30 |
| Rollback BE về trước PR-A | Cần flush cache v1 + huỷ approval RBAC pending (API-13, API-21) |

### Security Frontend
- **[M] FES-18 (= API-27) Admin gỡ quyền critical: BE trả 202 "chờ duyệt" nhưng FE hiển thị như đã gỡ.**
  - Vị trí: `membershipAdminApi.ts:302-308`, `useUsersMembershipsPage.ts:197-201`, `rolePermissionApi.ts:256-264`, `useRolesPermissionsPage.ts:213-222`, `notificationRulesApi.ts:185-194`.
  - Ví dụ: admin tắt `admin.membership.invite` của nhân viên nghỉ việc. Quyền vẫn còn tới khi có admin thứ hai duyệt. Company chỉ có một admin thì không bao giờ được duyệt, vì không được tự duyệt.
  - RP-08 mở rộng hành vi này thêm 8 mã.
  - Không có chỗ nào trong `src/` kiểm 202 hay `approval_id`.
  - Đánh giá MEDIUM chứ không HIGH vì BE vẫn thực thi đúng: quyền chưa bị gỡ là đúng thiết kế 4 mắt. Lỗi nằm ở việc admin bị đánh lừa về trạng thái.
- **[L] FES-17 (= ROLE-33 = API-31)** `cms-core/permissionGuards.ts:7-11` vẫn coi `disclosure_type.manage`, `rbac.manage`, `system.settings` là đủ để ghi template CMS, trong khi BE chỉ nhận `cms.template.write` (`disclosure/app/cms_template_permissions.go:83-92`). Chỉ ảnh hưởng UX; test FE đang khẳng định alias cũ.
- **[I] FES-19** FE giữ `permissions[]` suốt phiên (`App.tsx:436-455`), trong khi BE nay thu hồi quyền ngay. Mã lỗi mới hiển thị nguyên message tiếng Anh; React đã escape nên không có XSS.

### CORS & Credentials
Đã phân loại toàn bộ hit mới của secret scan: tất cả là false positive (`email, password = persona(p)`, regex, prose, `VALUES(...)`). File smoke `.out` không chứa IP, token, password hay email. CORS không đổi; không có `Set-Cookie`. nginx :3000 không proxy `/internal/*` và `/metrics`.
- **[L] SEC-18** IAM `.gitignore` không có `__pycache__/`. `a8acb51` đã commit 3 file `scripts/devqa/__pycache__/*.pyc`, trong đó có đường dẫn local `C:\Users\<user>\...`. Web còn 4 file `.pyc` (SEC-08).
- **[L] SEC-19** `qa_ro` có `SELECT` trên toàn bộ `cobo_iam.*`, kể cả `credentials.password_hash` và `outbox_events.payload` (chứa link reset/mời, BES-04). `qa_rw` sửa được `role_permissions` của role global. `run_sql` âm thầm dùng root khi thiếu `QA_DB_USER` (`scripts/devqa/provision_qa_accounts.py:49-57`, `devqa_env.py:109,124-133`).
- **[L] SEC-20** `--rotate` không revoke session của persona. Mỗi lần sync lại set `account_status='active', locked_until=NULL`, tức là mở khoá persona đã bị khoá (`provision_qa_accounts.py:100-115`).
- **[L] SEC-21** `~/.cobo/dev-qa.env` được ghi không kèm `0600`/`0700` (`provision_qa_accounts.py:81-91`).
- **[L] SEC-22** Persona đặc quyền, kể cả CMS operator, đăng nhập qua HTTP plaintext tới `:3000` ở mỗi vòng smoke (`provision_qa_accounts.py:161`). Plausible: cần kẻ tấn công nằm trên đường truyền.
- **[L] SEC-24** (main agent) `a8acb51` commit output thô của secret scan (`risk-review-2026-10-10/01-secret-scan-*.txt`, `risk-review-2026-10-10-2/01-secret-scan-*.txt`).
  - Scanner chỉ che phần giữa: `v[:2] + "***" + v[-2:]` (`.claude/skills/secrets-cors-cookie-review/scripts/scan_secrets.py:47`). File vì vậy lộ 4 ký tự của mỗi secret thật và là bản đồ vị trí secret.
  - Tác động biên nhỏ, vì phần lớn secret đó đã nằm plaintext trong compose.
  - Fix: chỉ commit bản tóm tắt (số lượng + file), gitignore output thô; hoặc scanner che toàn bộ.
- **[I] SEC-23** Lỗi SQL trong provision có thể in đoạn `IDENTIFIED BY '…'` (`provision_qa_accounts.py:224-225`). Khó xảy ra.

## Trạng thái mục cũ (tóm tắt)

| Trạng thái | Mục |
|---|---|
| **Đã sửa (từ lần 3)** | H3 (resolver + refresh), H16, H17/ROLE-06, CACHE-11, CACHE-12/PERF-21, BES-09, BES-10, BES-12, BES-13, BES-18 (code; chưa chạy test MySQL), BES-19..24, ROLE-02, 03, 05, 10, 14, 15..19, 21, 22, 23, 25..28, RP-08, RP-10, PERF-12, PERF-15, SEC-11; **BES-29** (`c43b9cc`) |
| **Sửa một phần** | ROLE-04/RP-07 (`admin.*` deny; action khác vẫn rơi về `system.settings`, BES-35); ROLE-07 (→ ROLE-30); ROLE-08 (BE xong; FE → FES-17); ROLE-09 (allowlist xong; gắn user có sẵn không cần đồng ý vẫn chờ quyết định); ROLE-12 (→ ROLE-29); ROLE-13 (→ ROLE-30; phía gán vẫn phân loại theo module); ROLE-20 (→ ROLE-31, ROLE-32, BES-32); ROLE-24 (`/internal/v1/authorize` nhận action tuỳ ý); BES-14; H21 (3 script web đã dùng `devQaEnv.mjs` nhưng **chưa commit**; password vẫn nằm trong 3 doc web + history; chưa rotate); SEC-07; API-09; API-13; API-21 |
| **Còn mở, code không đổi** | C1, C6, H1 (rộng hơn), H4, H5, H6, H8-H15, H18, H19, H20, H22, H23; BES-03..08, BES-17 (→ BES-33); PERF-05, 10 (nặng hơn, nay chạy dưới khoá), 11, 13, 14, 16/27, 17, 19, 20, 22..25, 28, 29, 32; CACHE-08; API-07, 08, 10, 11, 14 (rộng hơn), 15, 17 (rộng hơn), 18, 23; FES-02, 05..16; SEC-05, 06, 08, 09, 10, 12, 13; RP-16 (parity in-memory) |

## Quick wins (≤1h each)
1. **ROLE-29:** lowercase + trim `rule_code` khi create; `strings.EqualFold` ở 4 chỗ so sánh; thêm test viết hoa.
2. **BES-30:** đổi renderer `body.html` sang `html/template`; thêm test escape.
3. **BES-31:** `http.MaxBytesReader` + giới hạn số check trong `/internal/v1/authorize/batch`.
4. **ROLE-32:** gọi `assertKeepsPlatformOperator` cho từng `DirectRevokes` lúc approve.
5. **ROLE-34:** đếm admin chỉ với `AccountStatus=active`.
6. **PERF-35:** `AddRole` trước `RemoveRole` (hoặc một transaction) khi thay primary role.
7. **API-28:** mã lỗi riêng cho lock timeout.
8. **SEC-18/SEC-08:** `.gitignore` thêm `__pycache__/`, `*.pyc` ở cả hai repo; `git rm --cached` 3 + 4 file.
9. **SEC-20/SEC-21:** revoke session khi `--rotate`; ghi file credential với `0600`.
10. **FES-17:** đồng bộ alias CMS ở FE với BE và sửa test.
11. **H18:** thêm 4 migration còn thiếu vào `run_dev_migrations.sh` (đã ở danh sách từ lần 2).

## Decisions needed from the team
- **H1:** endpoint reminder-config nên yêu cầu quyền gì (gợi ý: cùng quyền sửa disclosure, kiểm record thuộc `company_id` của token)? Người nhận `individuals` có được ngoài danh sách member không (gợi ý: không)?
- **ROLE-30:** người không phải operator có được kích hoạt lại member đang giữ grant platform không? Khuyến nghị: không.
- **FES-18:** UX cho "đang chờ duyệt" (banner + link tab Approvals). Có cho phép company chỉ có một admin gỡ quyền critical không, hay báo trước là cần admin thứ hai?
- **API-25:** mã lỗi chuẩn là chữ thường (như code) hay chữ hoa (như doc)? Cần chốt trước khi làm PR-F.
- **BES-31/H23:** ngừng publish :8080 ra ngoài (chỉ đi qua nginx :3000)?
- **SEC-19/SEC-24:** phạm vi của `qa_ro`; có giữ output thô của secret scan trong git không?
- **Mục cũ:** ROLE-09 (gắn user có sẵn), phân loại `disclosure_type.manage`, BES-07, BES-08 (row không có digest).
- **Hạ tầng (không đổi):** rotate credential (MySQL root/app + SEC-12, seed account, `INTERNAL_REMINDER_TOKEN`, Gmail/H21, `configs/login_password_rsa_dev.pem`); rewrite history; đóng 3306/8025/1025/8080; TLS cho DEV (SEC-22).

## Not checked / BLOCKED
- **cache-versioning reviewer:** bị dừng giữa chừng. Main agent kiểm phần coverage invalidate và nginx; **chưa** kiểm lại từng mục CACHE-04/06/09/10/14..20, chunk 404 sau deploy và Service Worker.
- **`5715d6c` và phần không thuộc token path của `c43b9cc`:** chưa được be-security/api-compat của lượt này review.
  - `5715d6c`: `writeRequestMiddleware` (company suspended chỉ được đọc), migration `0152`, sửa `personalops`, `templatebuilderoauth`, `workflowdept`, `platformcms`.
  - `c43b9cc`: `me_handler.go`, `iam/app/service.go`.
  - Theo `reusable-task-updates.md`, phiên song song đã chạy reviewer bảo mật riêng ("không có HIGH") và smoke 18/18 + 16/16.
  - Nên chạy một lượt `/pr-review 1479d72..a8acb51` nếu cần bằng chứng độc lập.
- **DB/server:** không truy vấn DB, không gọi DEV (read-only). Các mục cần dữ liệu:
  - số member mỗi company (mức độ của PERF-33);
  - role `company_admin` active (ROLE-31);
  - role tenant_custom của c_001 chứa quyền operator (BES-32);
  - rule prefs viết hoa (ROLE-29);
  - Redis `maxmemory-policy`;
  - firewall 3306/8025/1025/8080;
  - `/metrics` qua docker-proxy (plausible bypass `isInternalSource`).
- **MySQL integration test** (`admin_lock_integration_test.go`, RBAC restore, BES-18): BLOCKED vì không có `MYSQL_TEST_DSN`.
- **`govulncheck`, `npm audit` online:** BLOCKED (cần network). Kết quả gần nhất: 19 lỗ hổng npm, 2 critical ở dependency không dùng.
- **Docker build:** không chạy, vì review read-only, không có thay đổi code.
- **Đã chạy (ở `1479d72`):**
  - `go test ./...`: 16 fail, đúng tập baseline;
  - `go vet ./...`: exit 1, chỉ do PERF-25;
  - `go test -race` (companyaccess, authorization, outbox): không có race;
  - FE vitest `permissionGuards` 5/5; sanitizer/approvals/auth-guard pass.
- **Ngoài phạm vi bảo mật, ghi nhận:** web `npm run lint` (`tsc --noEmit`) đỏ với 16 lỗi type; `src/lib/loginErrorMessage.test.ts` fail 9/11 (lệch text).
- **Lịch sử git:** không quét toàn bộ; chỉ quét diff `a8acb51`.

## Suggested fix plan (grouped by repo and PR)
1. **iam PR-G (bảo mật reminder/notification, ưu tiên 1):**
   - H1: `authorize` + kiểm record thuộc company cho 5 route; giới hạn recipient.
   - BES-30 (`html/template`).
   - BES-34 (gửi riêng từng người hoặc Bcc; che email trong log).
   - BES-31 (giới hạn batch).
   - BES-33 (revoke session khi deactivate/delete, hoặc kiểm membership ở `sessionbound`).
2. **iam PR-H (hậu kỳ guard admin):** ROLE-29, ROLE-30, ROLE-31, ROLE-32/API-30, BES-32, ROLE-34, PERF-35, PERF-36, API-28.
3. **iam PR-I (perf khoá admin):** PERF-33 (đếm tổng hợp, nhả khoá sớm), PERF-34, PERF-20, PERF-16/27 (singleflight), PERF-10.
4. **iam PR-C (còn treo từ lần 3):** BES-08, PERF-19, PERF-23, PERF-22; chạy integration test MySQL.
5. **iam PR-D (worker):** H8-H15, PERF-11, PERF-14, PERF-28/29/32, CACHE-08.
6. **iam PR-E (deploy/docs):**
   - H18, H19.
   - API-25 (`api-postman-sync` cho các thay đổi của vòng ROLE + chốt hoa/thường), API-15.
   - API-32 (đưa runbook rollback vào skill).
   - SEC-18, SEC-24, SEC-12, SEC-13, PERF-25.
   - Ghi deploy/rollback `c43b9cc`/`5715d6c` vào release notes.
7. **iam/devqa tooling:** SEC-19, SEC-20, SEC-21, SEC-23.
8. **web PR-F:**
   - FES-18/API-27 (202 pending); FES-17 (alias CMS).
   - API-24 (lỗi deactivate/delete + `suppressForbiddenNavigation`), API-26, API-29/FES-10 (logout khi `SESSION_EXPIRED`/`COMPANY_INACTIVE`).
   - API-14 (mã lỗi mới); FES-19; FES-15/16; FES-07/09; API-10/11/17/18.
   - Commit thay đổi H21 + SEC-08; sửa lint/vitest đang đỏ.
9. **Hạ tầng (cần người có quyền server):** C1, C6, H20-H23, rotate credential, TLS DEV (SEC-22), đóng :8080 (BES-31).
