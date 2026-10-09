# Risk review 2026-10-09 (lần 2) - scope: all (cobo_iam_services `7a131a1`, cobo_web_design `26ed6c4e`)

Read-only. 7 reviewer chạy song song (fe-security, be-security, admin-role, perf-reliability, cache-versioning, api-compat, secrets-cors). Mọi HIGH đã được main agent đọc lại code để xác nhận. Lần 1: `../risk-review-2026-10-09/10-risk-report.md`. Bảng dưới chỉ đếm phát hiện **mới** ở lần 2. Mục cũ còn mở được liệt kê riêng.

| Group | Status | Critical | High | Medium | Low |
|---|---|---|---|---|---|
| Security Frontend | Issues | 0 | 0 | 2 | 6 |
| Security Backend | Issues | 1 | 2 | 5 | 3 |
| Backend Performance & Reliability | Issues | 0 | 1 | 5 | 2 |
| Cache & Versioning | Issues (H17 cũ còn mở) | 0 | 0 | 1 | 3 |
| API Compatibility | Issues | 0 | 0 | 2 | 2 |
| CORS & Credentials | Issues (C1/C6/H20-H23 cũ còn mở) | 0 | 0 | 0 | 3 |

## Kiểm tra lại các bản sửa C2-C5
- be-security và admin-role đã dò toàn bộ 349 route. **Không tìm thấy đường vượt** C2 (workflowconfig authz), C3 (gỡ adhoc legacy migrate), C4/H2 (company scope route tenant) hay C5 (scope theo membership/team/department/title).
- Các khoảng trống mới (ROLE-01..03) nằm ở **đường code mà C4/C5 không chạm tới**.

## Critical / High (fix first)

### CRITICAL — Security Backend

**ROLE-01 (= BES-02): Rollback/apply ma trận RBAC của một tenant ghi đè quyền của role GLOBAL dùng chung cho mọi tenant** — **nâng lên CRITICAL (user, 2026-10-09)**, confirmed bằng code và dữ liệu DEV
- **Phân tích đầy đủ:** `../bug-rbac-rollback-global-roles-2026-10-09/00-report.md`; audit read-only: `02-data-audit.md`.
- **Dữ liệu DEV:**
  - Rollback ở c_001 về snapshot mới nhất sẽ gỡ `platform.cms.view` và `cms.*` khỏi `cms_operator` và `admin_web`, làm cả 3 operator CMS mất quyền vào `/cms`.
  - Rollback ở một tenant tự đăng ký sẽ gỡ `deadline.comment` khỏi role global `self_reg_company_owner` của 5 company.
  - Đã từng có 1 rollback RBAC thật ở c_001 (2026-07-01).
- **Quyết định đã chốt:** chỉ reconcile role `tenant_custom` của company; rollback chứa quyền critical phải qua approval.
- **Trạng thái (2026-10-09): FIXED IN BRANCH, DEPLOYED DEV (chưa commit).** Smoke có ghi 38/38: `../release-2026-10-09/09-role01-post-deploy.md`. Xem `../bug-rbac-rollback-global-roles-2026-10-09/05-completion.md`. Vòng review còn phát hiện và đã xử lý thêm: đồng bộ quyền trực tiếp cũng không được đụng quyền ngoài `GrantablePermissions`, thêm quyền vào role phải theo grant policy, rollback có thay đổi cần `rbac.manage`.
- *Nội dung gốc lần review:*
- **Vị trí:**
  - `internal/companyaccess/infra/mysql/admin_repository_versioning.go:278-321` (`RestoreRBACMatrixFromSnapshot`), `admin_repository_approval.go:264-340` (apply approval).
  - `admin_repository_rbac_read.go:65` (`ListRoles`: `r.company_id IS NULL OR r.company_id = ?`).
  - `app/config_versioning.go:250-267`; cổng `authorizeConfigurationHealth` (`configuration_health.go:67`) chỉ đòi `rbac.manage` **hoặc** `system.settings`.
- **Chuỗi lỗi:**
  - `roleSet` gồm cả role global. Bảng `role_permissions` không có cột company, nên `removeRolePermissionTx`/`INSERT` sửa role global cho **mọi** tenant.
  - Snapshot đích được lọc qua `filterEnterpriseRBACSnapshotJSON` (`enterprise_scope.go:71`), vốn bỏ quyền ngoài phạm vi doanh nghiệp. Khi restore, các quyền đó bị **xoá** khỏi role global.
  - Bỏ qua cả quy tắc protected-role mà `AssignRolePermission`/`RemoveRolePermission` có.
- **Role global có thật** (main agent đã xác minh): `self_reg_company_owner` (migration 0026, role của mọi chủ công ty tự đăng ký, sao quyền từ `full_access` của c_001; 0073 bổ sung quyền) và `dept_lead` (0049). Migration 0123 gắn `system_global`/`is_protected=1` cho chúng.
- **Ai làm được:** chủ bất kỳ công ty tự đăng ký nào (có `rbac.manage`) gọi `POST /api/v1/admin/rbac/matrix/versions/{n}/rollback` sau khi đã có một snapshot.
- **Hậu quả:** đưa quyền của role chủ sở hữu dùng chung về trạng thái cũ (cấp lại quyền đã thu hồi hoặc gỡ quyền mới thêm) và gỡ mọi quyền ngoài phạm vi doanh nghiệp của nó, trên toàn hệ thống.
- **Đã xác minh (02-data-audit.md):** role global hiện không giữ quyền `cms`/`platform`, nhưng đã đổi quyền sau snapshot (`deadline.comment`). Role `tenant_default` của c_001 đang giữ quyền platform và sẽ bị gỡ.
- **Fix:**
  - Chỉ reconcile role `company_id = ?` và `role_type = 'tenant_custom'`; bỏ qua role global và protected.
  - `SubmitConfigApproval` gọi `roleForRBACMutation` + `IsRoleProtectedForMutation`.
  - Test hồi quy có role global.

### HIGH — Security Backend

**ROLE-02 (= BES-01): `POST /api/v1/admin/resource-scope-rules` và `/workflow-assignee-rules` lấy `company_id` từ body** — confirmed
- **Vị trí:** `app/admin_service.go:1342-1353`; `infra/mysql/admin_repository.go:592` (`strFromMap(rule, "company_id")`). Riêng `CreateNotificationRule` (`:1358`) đã ép `company_id` theo token.
- **Hậu quả:**
  - Chủ tenant A ghi rule vào tenant B (cùng loại lỗi với C4).
  - Chiếm `rule_code` của B; 409 trả về lộ sự tồn tại.
  - Rule rác đi vào pipeline validation/conflict của B, có thể sinh finding chặn duyệt approval của B.
- **Ghi chú mức độ:** be-security chấm MEDIUM vì nơi tiêu thụ hiện chỉ là cảnh báo. Giữ HIGH vì là ghi chéo tenant, không cần điều kiện gì.
- **Fix:** ghi đè `payload["company_id"] = sub.CompanyID` ở cả hai service + test chéo tenant.

**ROLE-03: Người chỉ có quyền mời (hoặc trưởng phòng được uỷ quyền) cấp được role quản trị công ty khi mời/tạo user** — confirmed
- **Vị trí:** `app/admin_service_invite_org.go:28-77` (`validateEnterpriseInviteRole`); `app/enterprise_invite_roles.go:6-14`; `admin.go:448`.
- **Chuỗi lỗi:**
  - `admin.membership.invite` là quyền cấp trực tiếp được (`GrantablePermissions`, `admin.go:448`).
  - Danh sách cấm role chỉ chặn `dept_lead`, `admin_web`, `cms_operator`, `full_access`, `truong_phong_ban`, `truong_nhom`, `self_reg_company_owner`. Không chặn `admin_doanh_nghiep`/`company_admin` (mang `rbac.manage`).
  - Không có quy tắc "chỉ được cấp role có quyền là tập con của quyền mình".
- **Hậu quả:** người có quyền mời mà không có `rbac.manage` tạo được tài khoản admin toàn công ty. Lưu ý: `validateEnterpriseInvitePermissions` chặn `rbac.manage` khi cấp *trực tiếp*, nhưng không chặn khi đi qua *role*.
- **Fix:** người gọi không có `rbac.manage` chỉ được cấp role có quyền ⊆ quyền hiệu lực của mình (tối thiểu: cấm role chứa `rbac.manage`, `system.settings`, `admin.membership.invite`, `admin.role.permission.*`) + test.

### HIGH — Backend Performance & Reliability

**PERF-10: `ListMembershipsByCompany` chạy 3N+1 query tuần tự, không LIMIT, nuốt lỗi** — confirmed (code); tác động tăng theo số thành viên
- **Vị trí:** `infra/mysql/admin_repository.go:184-210` (vòng lặp gọi `listMembershipDeptViews/TitleViews/RoleViews`, lỗi bị `_`).
- **Nơi gọi:**
  - `ListCompanyMemberships` (`admin_service.go:1114`, phân trang **trong bộ nhớ** sau khi tải toàn bộ).
  - `countActiveAdminCapableMembers` (`rbac_role_assignability.go:82`, thêm tối đa 2 query mỗi member).
  - `invalidateEffectiveAccessForCompany` (`config_versioning.go:97`, chỉ cần id).
- **Hậu quả:** công ty vài trăm người làm một request giữ connection hàng giây. Pool `MaxOpen 25` (`platform/db/mysql.go:21`) có thể cạn khi vài admin thao tác cùng lúc. Lỗi DB bị nuốt nên trả 200 với role/phòng ban rỗng, và lockout check tính sai.
- **Fix:** query chỉ lấy id cho invalidate; nạp role/phòng ban/chức danh theo trang bằng `IN (...)`; LIMIT/OFFSET ở SQL; đếm admin bằng một `COUNT(DISTINCT ...)`; không bỏ lỗi.

### HIGH cũ vẫn còn mở (không đổi từ lần 1)
- **H17 + H3 (ROLE-06/CACHE-01):** cache quyền hiệu lực (Redis, TTL 5 phút) chỉ invalidate ở rollback/apply approval. Truy vấn quyền không lọc `membership_status`. Hệ quả: khoá hoặc hạ quyền một admin không có tác dụng ngay. Ưu tiên cao vì đây là công cụ xử lý tài khoản bị chiếm.
- **H1** reminder authz/company scope.
- **H4–H16, H18–H23**, cùng các CRITICAL hạ tầng **C1** (seed account) và **C6** (Mailpit public): không có commit nào chạm tới.

## Medium / Low

### Security Backend
- **[M] ROLE-04** `AssignCompanyAdmin`/`RevokeCompanyAdmin`/`TransferOwnership` gọi `authorize(..., "rbac.manage")` như một action code. `legacyPolicy` (`authorization/infra/mysql/repository.go:206`) không có case cho chuỗi này nên rơi về mặc định `system.settings`.
  - Chủ công ty tự đăng ký (chỉ có `rbac.manage`) bị 403. Ai có `system.settings` lại làm được.
  - Confirmed với fallback; chưa xác minh `action_policy_matrix` trên DEV có dòng `rbac.manage` không.
  - Fix: `requireRbacManage`.
- **[M] ROLE-05** `RemoveRole` và đổi primary-role không bảo vệ primary admin (owner). Một admin ngang hàng có thể gỡ quyền owner. `RemoveRole` cũng thiếu lockout "admin cuối cùng".
- **[M] ROLE-08** (plausible) Ghi template CMS chỉ đòi `platform.cms.view` + (`cms.template.write` | `disclosure_type.manage`), không đòi `rbac.manage|system.settings`. `disclosure_type.manage` lại cấp trực tiếp được bởi tenant admin.
- **[M] BES-03** Đổi email hồ sơ không kiểm định dạng/duy nhất/xác minh lại. `FindUserByEmail` `LIMIT 1` không `ORDER BY`, nên luồng quên mật khẩu có thể trỏ nhầm user.
- **[M] BES-04** Link reset mật khẩu/mời (chứa token thô) nằm nguyên trong `outbox_events.payload` và không bao giờ bị xoá. Rủi ro tăng do H20 (MySQL public).
- **[M] ROLE-07** (cũ, C4 follow-up) hai định nghĩa platform operator còn song song (`isPlatformCMSOperator` yếu ở `admin_service.go:617`).
- **[L] ROLE-10** `AddTeamMember` kiểm member thuộc `department_id` trong body thay vì phòng ban của team (chỉ trong cùng tenant).
- **[L] BES-05** `GET/PATCH /company/disclosure-types/{type_id}/preferences` đọc cấu hình hạn của template riêng của tenant khác (`disclosure/infra/mysql/repository.go:2005`), tạo oracle tồn tại.
- **[L] BES-06** Ciphertext đăng nhập RSA không có nonce, phát lại được. Mời/gửi lại lời mời không có quota.
- **[L] ROLE-09** (cũ) `CreateMembership` gắn `user_id` bất kỳ với `status` tự do.

### Backend Performance & Reliability
- **[M] PERF-11** `DeleteMembership` chỉ xoá `membership_roles`, `department_memberships`, `membership_titles`. Còn FK từ `org_unit_memberships` (0008), `membership_direct_permissions` (0043), `departments.head_membership_id` (0047), `workflow_task_assignees` (0128), nên MySQL 1451 trả 500 thô. **Đây là gốc của BES-02 (task đã tách).** Main agent đã xác minh.
- **[M] PERF-12** Guard "không xoá primary admin" là `if err == nil && m.IsPrimaryAdmin` (`admin_service.go:919`): fail-open khi lookup lỗi, và nằm ngoài khoá `FOR UPDATE`. Main agent đã xác minh.
- **[M] PERF-13** (plausible) Không có retry cho deadlock 1213/lock-wait 1205. Transaction mới (khoá cha rồi xoá con) có thể xung đột với insert con đồng thời.
- **[M] PERF-14** Mỗi lần `hasPermission` trả false lại chạy `UPDATE emergency_access_grants ... expires_at<=?` (`admin_service.go:758`), tức là ghi DB và khoá next-key trên đường đọc authz.
- **[M] PERF-15** `restoreRBACMatrixInTx` đọc qua `r.db` khi đang giữ tx (2 connection/request, N+1 trong tx), nên có thể bỏ đói pool khi apply đồng thời.
- **[L] PERF-16** Cache quyền không có singleflight. C4/C5 thêm lượt `GetEffectiveAccess` cho mỗi request.
- **[L] PERF-17** Thêm round-trip mỗi request admin (đều là PK lookup).
- **[I] PERF-18** 2 test fail có sẵn ở HEAD (`TestUpdateNotificationRule_TierEnforcement_FlagOffAllowsPremium`, `TestCreateSelfServiceCompany_FeatureFlagOff`).

### Cache & Versioning
- **[M] CACHE-08** Worker không dùng `CompositeProvider` lịch nghỉ như API (`cmd/worker/main.go:135`). Hạn định kỳ do worker seed bỏ qua lịch nghỉ upload qua CMS, nên lệch với API.
- **[L] CACHE-06** (tinh chỉnh) chunk `node-forge` 404 sau deploy trên HTTP; chưa có handler `vite:preloadError`.
- **[L] CACHE-09** nginx: asset không phải js/css trả `index.html` 200; `location /api/` thiếu `^~`; không gzip bundle 2.5 MB.
- **[L] CACHE-10** Store cache quyền trong bộ nhớ không giới hạn, không janitor; `DEL` tuần tự.

### API Compatibility
- **[M] API-08** (plausible) Proposal adhoc `pending_admin_approval` không có `process_controller_id` không còn đường chốt sau khi gỡ endpoint migrate (C3). Cần đếm số dòng trên DB.
- **[M] API-09** `api-contracts-json.md` ghi foreign title → 404, nhưng assign-title và `POST /titles/{id}/members` trả 400. Chưa ghi 403 khi gán role mang quyền platform, cũng chưa ghi `COMPANY_SCOPE_MISMATCH` cho invite-roles/resend/assign-company.
- **[L] API-10** FE: 403 `COMPANY_SCOPE_MISMATCH` khi tạo user/membership kích hoạt chuyển trang forbidden toàn cục.
- **[L] API-11** FE: GET template (C2) giờ cần quyền template, nhưng FE không chặn chuyển trang forbidden cho lời gọi đọc.
- **[I] API-12** Đổi mã lỗi trên điều kiện cũ (`PERMISSION_DENIED`→`COMPANY_SCOPE_MISMATCH`, `INVALID_REQUEST`→`MEMBERSHIP_NOT_FOUND`). FE không bị ảnh hưởng.

### Security Frontend
- **[M] FES-07** Đổi company thất bại giữa chừng thì token và `SELECTED_COMPANY` của company mới đã ghi vào localStorage, trong khi UI vẫn ở company cũ (`App.tsx:712-848`). Lời gọi không mang id sẽ chạy ở tenant khác với tenant đang hiển thị.
- **[M] FES-08** (plausible, chỉ khi token mode jwt/dual) `DisclosureTypeDetail.tsx:946` tự gọi `switch-company` bằng `fetch` thô, đảo token dùng chung giữa các tab.
- **[M] FES-09** (cũ CACHE-03, cập nhật) Không có listener `storage`. `COMPANY_SCOPE_MISMATCH`/404 ở tab cũ bị coi là "không có quyền" thay vì resync.
- **[L] FES-10** Không có xử lý hết phiên toàn cục: `onAuthFailure` xoá token nhưng không `resetSession()`.
- **[L] FES-11** Đăng nhập rơi về mật khẩu dạng rõ khi không lấy được khoá RSA; BE vẫn nhận.
- **[L] FES-12** Token mời gửi qua query string GET.
- **[L] FES-13** `rememberMe` không có tác dụng ở client.
- **[L] FES-14** Bearer token gửi tới `qr_url` do API trả về (phòng thủ chiều sâu).
- **[L] ROLE-11** Guard route FE `/app/admin/*` rộng hơn BE (chỉ ảnh hưởng UX).
- **Cập nhật mục cũ:** FES-02 (thiếu CSP/HSTS...) không đổi. FES-06: `npm audit` 19 lỗ hổng (2 critical ở `express`/`proxy-addr`, không dùng trong `src/`).

### CORS & Credentials
- **[L] SEC-07** `smoke_dev_c5.py` và tài liệu release gắn cứng IP server DEV và dùng seed account (hệ quả của C1). Không chứa mật khẩu hay token.
- **[L] SEC-08** Repo web track 4 file `__pycache__/*.pyc` (một file thêm ở `26ed6c4e`).
- **[L] SEC-09** CORS: `Vary: Origin` chỉ đặt khi origin được phép; không có `Max-Age`/`Expose-Headers`.
- **[I] SEC-10** Deploy SSH `StrictHostKeyChecking=accept-new`.
- Commit mới không thêm secret nào (đã quét các commit gần đây ở cả hai repo).

## Quick wins (≤1h each)
1. ROLE-02: ghi đè `company_id` theo token ở hai service tạo rule + test.
2. ROLE-04: đổi ba cổng sang `requireRbacManage`.
3. PERF-12: trả lỗi khi lookup lỗi; đọc `is_primary_admin` trong `SELECT ... FOR UPDATE`.
4. PERF-11: map MySQL 1451 → 409 `STATE_CONFLICT` (giải pháp tạm cho BES-02).
5. `AddTitleMember`: gọi `requireTitleInCompany` trước, để trả 404 như các route khác (cũng sửa lệch API-09).
6. ROLE-10: so với phòng ban của team.
7. SEC-08: `.gitignore` `__pycache__/`, `git rm --cached` 4 file.
8. API-09: cập nhật `api-contracts-json.md`.

## Decisions needed from the team
- ~~ROLE-01: phạm vi rollback, approval, query DEV~~ → đã chốt (chỉ `tenant_custom`; rollback chứa quyền critical qua approval; audit xong; CRITICAL).
- **ROLE-03:** quy tắc "chỉ cấp role có quyền ⊆ quyền của mình" hay chỉ mở rộng danh sách cấm?
- **DeleteMembership:** xoá cứng với cascade đầy đủ hay chuyển sang xoá mềm (khuyến nghị, đồng thời giải quyết H3/ROLE-06)?
- **API-08:** đếm proposal mồ côi trên DB và chọn đường sửa (route reject/cancel hay SQL repair).
- Các quyết định hạ tầng từ lần 1 (rotate credential, rewrite history, ENV, token mode) vẫn chờ.

## Not checked / BLOCKED
- Không truy vấn DB, không gọi server: ROLE-01 (quyền của role global), ROLE-04 (`action_policy_matrix`), API-08 (số proposal), FES-08 (token mode thực tế).
- Không chạy header check, load test, `govulncheck`. Không quét toàn bộ lịch sử git (chỉ ~10 commit gần nhất).
- `go vet`/`go test -race ./internal/companyaccess/...` có chạy (perf reviewer): sạch race, còn 2 fail có sẵn.
- `personalops`, `portaldashboard`, holiday XLSX: chỉ kiểm tenant scope, chưa kiểm parse.

## Suggested fix plan (grouped by repo and PR)
1. **iam PR-A (security, ưu tiên 1):** ROLE-01 + ROLE-02 + ROLE-03 + ROLE-04 + ROLE-05 + ROLE-07 (cùng vùng `companyaccess`, cùng persona test).
2. **iam PR-B (thu hồi quyền):** H17/ROLE-06 invalidate cache sau mọi thay đổi RBAC/membership + lọc `membership_status` (H3) + DeleteMembership (PERF-11/12, xoá mềm hoặc cascade đầy đủ).
3. **iam PR-C (perf):** PERF-10, PERF-14, PERF-15, PERF-13.
4. **web PR-D:** FES-07/08/09/10, API-10/11, SEC-08.
5. **iam PR-E (khác):** BES-03, BES-04 (+ retention outbox), BES-05, CACHE-08, API-08 repair, API-09 docs, `AddTitleMember` 404.
6. **Hạ tầng (cần người có quyền server):** C1, C6, H20–H23. Không đổi so với lần 1.
