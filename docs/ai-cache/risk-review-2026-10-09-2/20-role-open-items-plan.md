# Kế hoạch xử lý các ROLE còn mở (đề xuất cho phiên sau)

Ngày lập: 2026-10-09. Trạng thái code: HEAD `10a5700` + thay đổi follow-up ROLE-01 chưa commit (đã deploy DEV). Vị trí code bên dưới đã kiểm lại trên cây làm việc hiện tại.

## 1. Cách đọc
- Các mã ROLE cũ bị **trùng giữa các vòng review** (ví dụ ROLE-11 có hai nghĩa khác nhau). File này đánh số lại thành **RP-01…RP-16** và có bảng đối chiếu ở mục 5.
- Mọi mục theo `wf-bugfix`: viết test đỏ trước (Gate R), sửa, verify (Gate V), review, ghi ai-cache. Dữ liệu DEV là dữ liệu mẫu, được phép ghi/dọn (memory `dev-data-is-sample`), vẫn ưu tiên thao tác qua API và ghi lại.
- Mức ưu tiên: **P0** khai thác được ngay hoặc ghi chéo tenant; **P1** hiệu lực sai của thao tác quản trị; **P2** làm cứng, nhất quán; **P3** chính sách/UX/dọn dẹp.
- Công sức: **S** ≤ nửa ngày, **M** 1–2 ngày, **L** > 2 ngày.

## 2. Tóm tắt

| Mã | Tên | Ưu tiên | Công sức | PR gợi ý | Cần quyết định |
|---|---|---|---|---|---|
| RP-01 | Tạo rule nhận `company_id` từ body | **P0** | S | PR-A | Không |
| RP-02 | Mời / tạo user cấp được role quản trị công ty | **P0** | M | PR-A | **Có** |
| RP-03 | Khoá/hạ quyền không có hiệu lực ngay; xoá membership lỗi FK | **P1** | L | PR-B | **Có** |
| RP-04 | Gỡ role / đổi role chính không bảo vệ primary admin | **P1** | M | PR-A | Có (nhỏ) |
| RP-05 | Hai định nghĩa "platform operator" song song | **P1** | S | PR-A | Không |
| RP-06 | Quyền ghi template CMS không theo định nghĩa platform operator | P2 | M | PR-D | **Có** (cần dữ liệu) |
| RP-07 | Chốt gate theo permission-as-action (phần còn lại của ROLE-04) | P2 | S | PR-D | Không |
| RP-08 | "Critical" lệch giữa gỡ quyền trực tiếp và rollback | P2 | S | PR-C | Có (nhỏ) |
| RP-09 | Admin tenant thu hồi được quyền trực tiếp `platform.*`/`cms.*` | P2 | S | PR-C | Có (nhỏ) |
| RP-10 | Chuyển quyền sở hữu không atomic | P2 | S | PR-A | Không |
| RP-11 | `AddTeamMember` kiểm phòng ban theo body | P2 | S | PR-A | Không |
| RP-12 | Bốn-mắt chỉ theo membership (tài khoản thứ hai) | P3 | M | PR-C | **Có** (chính sách) |
| RP-13 | Người duyệt rộng hơn người yêu cầu theo từng loại approval | P3 | S | PR-C | **Có** (chính sách) |
| RP-14 | `CreateMembership` gắn user bất kỳ với status tự do | P3 | M | riêng | **Có** |
| RP-15 | Guard FE rộng hơn BE (`/app/admin/*`, nút quản trị công ty, nút Hủy) | P3 | S | PR-E (web) | Không |
| RP-16 | Parity in-memory vs MySQL của repo test | P3 | S | PR-C | Không |

Thứ tự đề xuất: **PR-A** (RP-01, 02, 04, 05, 10, 11) → **PR-B** (RP-03) → **PR-C** → **PR-D**; **PR-E** (FE) độc lập.

---

## 3. Chi tiết từng mục

### RP-01 (P0, S): `POST /admin/resource-scope-rules` và `/admin/workflow-assignee-rules` nhận `company_id` từ body
- **Cũ:** ROLE-02 = BES-01 (risk review lần 2, "HIGH").
- **Vị trí:** `internal/companyaccess/app/admin_service.go:1354-1364` (`CreateResourceScopeRule`, `CreateWorkflowAssigneeRule` chuyển nguyên `req.Payload`); `internal/companyaccess/infra/mysql/admin_repository.go:592-593, 636-637` (`strFromMap(rule, "company_id")`); handler `transport/http/admin_handler.go:1051` (`createRule`). So sánh `CreateNotificationRule` (`admin_service.go:~1368`) đã ghi đè `req.Payload["company_id"] = req.Subject.CompanyID`.
- **Kịch bản:** chủ tenant A (có `rbac.manage`) gửi `{"company_id":"<B>", "rule_code":"x", ...}` → ghi rule vào tenant B; 409 "rule_code đã tồn tại" lộ dữ liệu của B; rule rác đi vào pipeline validation/conflict của B và có thể chặn duyệt approval của B.
- **Sửa:** ghi đè `payload["company_id"] = sub.CompanyID` ở cả hai service (giống notification); lý tưởng thay map tự do bằng struct có kiểu, từ chối khoá lạ.
- **Test trước:** (1) service: persona tenant A tạo rule với `company_id` của B → rule chỉ nằm ở A, không có dòng ở B (in-memory có `resourceScopeRules`, `workflowAssigneeRules`); (2) handler: route sweep tương tự `admin_handler_company_scope_test.go`; (3) revert-check bỏ dòng ghi đè → test đỏ.
- **Quyết định:** không. **Phụ thuộc:** không.

### RP-02 (P0, M): Người chỉ có quyền mời (hoặc trưởng phòng được uỷ quyền) cấp được role `admin_doanh_nghiep`/`company_admin`
- **Cũ:** ROLE-03.
- **Vị trí:** `internal/companyaccess/app/admin_service_invite_org.go:28-77` (`validateEnterpriseInviteRole`: chỉ chặn danh sách deny + `assertRoleHasNoPlatformPermissions`); `enterprise_invite_roles.go:6-14` (danh sách deny không có `admin_doanh_nghiep`/`company_admin`); các nơi gọi: `admin_service.go:189` (CreateUser), `:487` (invite), `:560`, `admin_service_primary_role.go:61` (đổi role chính); cổng `authorizeMembershipInvite`; `admin.membership.invite` là quyền cấp được trực tiếp (`admin.go` `GrantablePermissions`).
- **Kịch bản:** primary admin cấp `admin.membership.invite` trực tiếp cho một trưởng phòng; người đó gọi `POST /admin/users/invite` hoặc `POST /admin/users` với `role_code:"admin_doanh_nghiep"` → tài khoản mới có `rbac.manage` toàn công ty. `validateEnterpriseInvitePermissions` chặn `rbac.manage` khi cấp *trực tiếp* nhưng không chặn khi đi qua *role*.
- **Sửa (theo quyết định):** người gọi **không có `rbac.manage`** chỉ được cấp role có tập quyền ⊆ quyền hiệu lực của chính họ (tối thiểu: cấm role chứa `rbac.manage`, `system.settings`, `admin.membership.invite`, `admin.role.permission.*`); `ListInviteRoles` chỉ trả các role họ được cấp.
- **Test trước:** persona invite-only và dept-scoped mời với `admin_doanh_nghiep` → 403; persona `rbac.manage` vẫn được; `ListInviteRoles` không liệt kê role vượt quyền; chạy cho cả 4 nơi gọi (invite, create-user, primary-role, assign-role).
- **Quyết định:** (a) quy tắc "tập con quyền của mình" hay chỉ mở rộng danh sách cấm (khuyến nghị: tập con, có test bảng); (b) role dùng chung `self_reg_company_owner` có bị coi là "vượt quyền" không.
- **Phụ thuộc:** nên làm cùng RP-04 (cùng persona test).

### RP-03 (P1, L): Khoá hoặc hạ quyền một thành viên không có hiệu lực ngay; xoá membership lỗi khoá ngoại
- **Cũ:** ROLE-06 + H3 + H17 + CACHE-01 + PERF-11 + PERF-12 + BES-02 (task `task_68c631db`).
- **Vị trí:**
  - Truy vấn quyền không lọc trạng thái membership: `internal/authorization/infra/mysql/repository.go:19-38` (nhánh role: `WHERE m.membership_id = ? AND m.company_id = ?`; nhánh direct: không join `memberships`).
  - Cache quyền hiệu lực (Redis, TTL 5 phút, `internal/authorization/infra/projection/*`) chỉ bị xoá ở `config_versioning.go:297` (rollback), `config_approval.go:493` (apply approval) và nay ở `admin_service.go:1032, 1057, 1081` (gán/thu hồi admin, chuyển sở hữu). **Chưa xoá** sau: `UpdateMembership` (`admin_service.go:896`), `DeleteMembership` (`:912`), `AssignRole`/`RemoveRole` (`:1148`/`:1180`), `ReplaceMembershipPrimaryRole` (`admin_service_primary_role.go:41`), `AssignTitle`/`RemoveTitle` (`:1213`/`:1222`), `UpdateMembershipOrgAssignments` (`admin_service_membership_org.go:154`), `AddDeptMember` (`admin_service_departments.go:86`), `AddTeamMember` (`:977`), `AssignRolePermission`/`RemoveRolePermission` (`:1293`/`:1326`, nhưng role-level cần xoá cả công ty), `AddDirectPermission`/`RemoveDirectPermission` (`:1662`/`:1683`), tạo/sửa/vô hiệu role tùy chỉnh.
  - `DeleteMembership` (`infra/mysql/admin_repository.go:~148-180`) chỉ xoá `membership_roles`, `department_memberships`, `membership_titles`; còn FK từ `org_unit_memberships` (0008), `membership_direct_permissions` (0043), `departments.head_membership_id` (0047), `workflow_task_assignees` (0128) → MySQL 1451 trả 500. Guard primary admin ở `admin_service.go:919-923` là `if err == nil && m.IsPrimaryAdmin` (fail-open) và nằm ngoài khoá.
  - Phiên (session/refresh token) không bị thu hồi khi khoá.
- **Kịch bản:** admin khoá một admin đã bị nghi ngờ; người đó giữ nguyên quyền (DB không lọc trạng thái, cache tới 5 phút/mỗi replica) và vẫn refresh được token; `DELETE /memberships/{id}` hầu như luôn 500 nên không có cách xoá thật.
- **Sửa (hai lớp):** (1) lọc `m.membership_status = 'active'` ở cả hai nhánh của truy vấn quyền; (2) invalidate **theo membership** (không phải cả công ty, vì `invalidateEffectiveAccessForCompany` gọi `ListMembershipsByCompany` 3N+1, xem PERF-10) sau mọi ghi trong danh sách trên, đặt sau khi commit; (3) thu hồi session khi khoá; (4) `DeleteMembership`: đọc `is_primary_admin` trong chính câu `SELECT ... FOR UPDATE`, trả lỗi khi lookup lỗi, và hoặc dọn đủ bảng con trong tx hoặc chuyển xoá mềm.
- **Test trước:** repo MySQL tích hợp (`newITWorld`): khoá membership → `ListPermissionCodes` rỗng; service: sau mỗi thao tác trong danh sách thì `InvalidateMemberships` được gọi cho đúng membership (dùng `recordingCache` trong `rbac_rollback_scope_test.go`); `DeleteMembership` với thành viên có team/direct grant/trưởng phòng → thành công hoặc 409 (không 500), primary admin bị chặn kể cả khi lookup lỗi; revert-check từng điểm.
- **Quyết định:** xoá mềm (khuyến nghị, một lần giải quyết cả H3) hay cascade đầy đủ; thu hồi session có thuộc phạm vi PR này không.
- **Phụ thuộc:** **PERF-10** (`ListMembershipsByCompany` 3N+1, `admin_repository.go:~184-210`) nên sửa trước hoặc cùng PR để invalidate theo membership không kéo cả danh sách; cần id membership đích ở mọi call site.

### RP-04 (P1, M): `RemoveRole` và đổi role chính không bảo vệ primary admin; thiếu lockout "admin cuối cùng"
- **Cũ:** ROLE-05.
- **Vị trí:** `internal/companyaccess/app/admin_service.go:1180` (`RemoveRole`: chỉ `requireTargetMembership` rồi `repo.RemoveRole`); `admin_service_primary_role.go:41-105` (`ReplaceMembershipPrimaryRole`); `rbac_role_assignability.go:103` (`assertPrimaryRoleChangeLockout`, chỉ chặn khi đích là admin-capable cuối cùng và chỉ ở đường primary-role). So sánh `UpdateMembership`/`DeleteMembership`/`RevokeCompanyAdmin` đã chặn `IsPrimaryAdmin`.
- **Kịch bản:** owner bổ nhiệm tới 5 admin; một admin ngang hàng gọi `DELETE /memberships/{owner}/roles/{admin_role}` hoặc `PUT .../primary-role` với `user_thuong` → owner mất `rbac.manage`; cùng với việc `TransferOwnership` đòi primary admin, owner không lấy lại được quyền.
- **Sửa:** từ chối `RemoveRole`/`ReplaceMembershipPrimaryRole` lên mục tiêu `IsPrimaryAdmin` trừ khi người gọi chính là primary admin; áp `assertPrimaryRoleChangeLockout` cả cho `RemoveRole`.
- **Test trước:** admin thường gỡ role của primary admin → 409 `CANNOT_*` (chọn mã theo `RevokeCompanyAdmin`); primary admin tự đổi role mình vẫn theo luật hiện có; gỡ role admin cuối cùng → 409.
- **Quyết định nhỏ:** cho phép primary admin tự hạ mình hay không (khuyến nghị: không, buộc dùng `TransferOwnership`).

### RP-05 (P1, S): Hai định nghĩa "platform operator"
- **Cũ:** ROLE-07 (C4 follow-up 4).
- **Vị trí:** định nghĩa yếu `isPlatformCMSOperator` = chỉ `platform.cms.view` tại `internal/companyaccess/app/admin_service.go:617`; nơi dùng `:168` (CreateUser), `:450` (invite), `:629`, `:1164` (`AssignRole`), `admin_service_primary_role.go:56`. Định nghĩa đúng: `isPlatformCompanyOperator` (`admin_service_company_scope.go`, `platform.cms.view` **và** (`rbac.manage` hoặc `system.settings`)).
- **Kịch bản:** tài khoản chỉ có `platform.cms.view` nhưng có `admin.membership.invite` ở một công ty bỏ qua `validateEnterpriseInvitePermissions`, kiểm role, `assertRoleHasNoPlatformPermissions`; hành vi không nhất quán theo route.
- **Sửa:** thay toàn bộ nơi dùng bằng `isPlatformCompanyOperator` (không overlay break-glass), xoá hàm yếu.
- **Test trước:** persona chỉ `platform.cms.view` + `admin.membership.invite` trên mỗi route trên bị xử lý như tenant admin thường (không bỏ qua kiểm role/quyền); persona operator đủ điều kiện vẫn đi đường cũ.
- **Phụ thuộc:** làm cùng RP-02 (cùng hàm `validateEnterpriseInviteRole`).

### RP-06 (P2, M): Quyền ghi template CMS toàn cục không theo định nghĩa platform operator
- **Cũ:** ROLE-08 (plausible).
- **Vị trí:** `internal/disclosure/app/cms_template_permissions.go:80` (`requireCMSTemplateWrite`) và `:12` (`permissionLegacyTemplateManage = disclosure_type.manage` được chấp nhận); danh sách cấp được `admin.go` (`GrantablePermissions` có `disclosure_type.manage`); FE `cobo_web_design/src/features/cms-core/permissionGuards.ts:9-14` (alias `cms.template.write` chấp nhận `disclosure_type.manage`, `rbac.manage`, `system.settings`); route nằm dưới `/api/v1/admin/disclosure-types/*` (`disclosure/transport/http/handler.go:45-72`).
- **Kịch bản:** tenant admin cấp `disclosure_type.manage` trực tiếp cho một người dùng CMS chỉ có `platform.cms.view` → người đó ghi được template toàn cục.
- **Sửa:** yêu cầu cùng quy tắc platform operator (`platform.cms.view` và (`rbac.manage` hoặc `system.settings`)) cho ghi, kích hoạt, lưu trữ, cấu hình template; bỏ `disclosure_type.manage` khỏi alias CMS khi client đã chuyển.
- **Quyết định + dữ liệu cần:** có role thật nào có `platform.cms.view` nhưng thiếu cả `rbac.manage` và `system.settings` (DEV: `cms_operator` c_001/c_002 và `admin_web` đều có `rbac.manage`, nên khả thi); cần query read-only trên DEV trước.
- **Test trước:** persona `platform.cms.view` + `disclosure_type.manage` (không `rbac.manage|system.settings`) → 403 ở mọi route ghi template; operator đủ điều kiện vẫn ghi được.

### RP-07 (P2, S): Chốt gate theo permission-as-action (phần còn lại của ROLE-04)
- **Cũ:** ROLE-04 (phần chưa xử lý: ba route đã sửa).
- **Vị trí:** `internal/authorization/infra/mysql/repository.go:206` (`required := "system.settings"` là mặc định cho action không khai báo trong `legacyPolicy`); các action dùng kiểu `Action: "..."` không có trong `legacyPolicy`: `breakglass.session.expired` (`companyaccess/app/config_break_glass.go`), `cms.admin.companies.members.add`, `cms.admin.users.assign_company`, `cms.admin.users.invite.resend`, `cms.admin.users.password_reset` (`platformcms/transport/http/handler.go`), `company.create_self_service`, `company.initialize` (`companyaccess/transport/http/admin_handler_provision.go`). **Cần xác minh** từng cái là action đi qua `Authorize` hay chỉ nhãn audit. Quét `s.authorize(ctx, ..., "<mã>")` trong `companyaccess/app` cho 42/42 mã đều có trong `legacyPolicy`.
- **Việc:** (1) xác minh 7 mã trên; (2) thêm test bảo vệ: mọi action literal truyền vào `Authorize` phải có trong `legacyPolicy` hoặc `action_policy_matrix`, và không action nào trùng một permission code; (3) kiểm hàng `action_policy_matrix` trên DEV có dòng `rbac.manage` hay không (ROLE-04 chưa biết).
- **Test trước:** test quét mã nguồn (parse literal) đỏ khi thêm action chưa khai báo.

### RP-08 (P2, S): Tập "critical" lệch giữa gỡ quyền thường và rollback
- **Cũ:** ROLE-12 (phần chưa làm; rollback đã dùng `isCriticalForRestore`).
- **Vị trí:** `internal/companyaccess/app/rbac_read.go:95-102` (`criticalPermissionCodes`, 6 mã) và `config_approval.go:29-40` (`isCriticalPermissionCode`, `requiresApprovalForDirectRemove`) vẫn là tập hẹp cho `RemoveRolePermission` (`admin_service.go:1326`) và `RemoveDirectPermission` (`:1683`); `rbac_restore_plan.go` (`isCriticalForRestore` = tập hẹp + tier `tenant_admin_only`/`high_risk` của `rbac_grant_policy.go`).
- **Hệ quả:** gỡ `admin.role.permission.assign`, `company.ownership.transfer`, `workflow.step.override`… khỏi role/thành viên không cần duyệt, nhưng rollback chạm các mã đó thì cần duyệt.
- **Sửa:** dùng một định nghĩa duy nhất (`isCriticalForRestore`) cho `RemoveRolePermission` và `RemoveDirectPermission`, thêm test bảng "mọi mã `tenant_admin_only`/`high_risk` đều là critical".
- **Quyết định nhỏ:** mở rộng yêu cầu duyệt cho gỡ quyền thường (thay đổi hành vi, cập nhật FE nếu cần).

### RP-09 (P2, S): Admin tenant thu hồi được quyền trực tiếp `platform.*`/`cms.*` của thành viên trong công ty mình
- **Cũ:** ROLE-21.
- **Vị trí:** `internal/companyaccess/app/admin_service.go:1683` (`RemoveDirectPermission`, không lọc mã; `platform.cms.view` có `PermissionRiskLevel` "low" nên thu hồi ngay, không cần duyệt); `config_approval.go` nhánh `ChangeTypeRBACDirectPermRemove` (`SubmitConfigApproval`); `rbac_restore_plan.go` (`explicitRevokes` áp dụng cho mọi mã); `infra/mysql/admin_repository.go:~856-870` (`RevokeDirectPermission`). DEV có 1 quyền trực tiếp `platform.cms.view` đang active.
- **Sửa:** từ chối mã trong `EnterpriseDenyCodes` hoặc có tiền tố `platform.`/`cms.` ở cả `RemoveDirectPermission` và `SubmitConfigApproval` (chỉ platform operator được thao tác qua route `/platform/cms`).
- **Test trước:** admin tenant gỡ `platform.cms.view` trực tiếp của thành viên → 403; vẫn gỡ được quyền grantable; approval tương tự.
- **Quyết định nhỏ:** có route platform nào cần cho operator gỡ quyền này không (nếu có thì đã đủ).

### RP-10 (P2, S): `TransferOwnership` không atomic
- **Cũ:** ROLE-22 (phần chưa làm).
- **Vị trí:** `internal/companyaccess/app/admin_service.go:1061-1085` (`ClearMembershipPrimaryAdmin` rồi `SetMembershipPrimaryAdmin`); repo `infra/mysql/admin_repository.go:300-312`.
- **Kịch bản:** UPDATE thứ hai lỗi → công ty không còn primary admin.
- **Sửa:** một phương thức repo `TransferPrimaryAdmin(ctx, companyID, from, to)` trong một transaction có điều kiện (`from` đang là primary, `to` thuộc company); đổi interface `AdminRepository` (MySQL + in-memory).
- **Test trước:** tích hợp MySQL với lỗi giả ở bước hai (đóng kết nối/ép lỗi) → trạng thái không đổi; hai chuyển đồng thời → đúng một primary admin.

### RP-11 (P2, S): `AddTeamMember` kiểm phòng ban theo body
- **Cũ:** ROLE-10.
- **Vị trí:** `internal/companyaccess/app/admin_service.go:977-996` (`MemberBelongsToDepartment(ctx, req.MembershipID, req.DepartmentID)` dùng `department_id` do client gửi, trong khi thêm vào `req.TeamID`); handler `transport/http/admin_handler.go:~1612`.
- **Sửa:** đọc `department_id` của chính team (repo đã có `TeamBelongsToCompany`; thêm lấy phòng ban cha) và so với đó; bỏ `department_id` khỏi body hoặc kiểm bằng nhau.
- **Test trước:** thành viên của phòng X thêm vào team của phòng Y với `department_id = X` trong body → 409 `MEMBER_NOT_IN_DEPARTMENT`.

### RP-12 (P3, M): Bốn-mắt chỉ theo membership
- **Cũ:** ROLE-20.
- **Vị trí:** `internal/companyaccess/app/config_approval.go:419` (approve) và `:540` (reject): `row.RequestedBy == req.Subject.MembershipID`; `RequestedBy` lưu membership.
- **Kịch bản:** chủ công ty mời tài khoản thứ hai do mình kiểm soát, gán `company_admin` (nay làm được nhờ ROLE-04) rồi tự duyệt yêu cầu của mình.
- **Chọn một:** (a) giữ nguyên (đã chốt cho công ty một admin); (b) so thêm `user_id` người yêu cầu (cần lưu `requested_by_user_id` hoặc tra từ membership); (c) với thay đổi critical, yêu cầu người duyệt khác `user_id` **và** membership đã tồn tại trước thời điểm yêu cầu (chống tạo tài khoản ngay trước khi duyệt).
- **Quyết định:** chính sách, không phải lỗi; khuyến nghị (c) cho approval critical.

### RP-13 (P3, S): Người duyệt rộng hơn người yêu cầu theo từng loại approval
- **Cũ:** ROLE-23 + BES-05.
- **Vị trí:** `internal/companyaccess/app/config_approval.go:22-24` (`authorizeConfigApprovalDecide` = `rbac.manage` hoặc `system.settings` cho mọi loại).
- **Sửa (tuỳ chọn):** với aggregate `rbac_matrix` yêu cầu `rbac.manage`; với `notification_rule` yêu cầu quyền cập nhật rule; `system.settings` đơn lẻ chỉ còn hợp lệ khi nó đi kèm quyền tương ứng.
- **Quyết định:** chính sách (hiện không có công ty nào giữ `system.settings`, nên rủi ro thấp).

### RP-14 (P3, M): `CreateMembership` gắn `user_id` bất kỳ với `status` tự do
- **Cũ:** ROLE-09 (= BES-02 của C4).
- **Vị trí:** `internal/companyaccess/app/admin_service.go:873-894`; `infra/mysql/admin_repository.go:~96-121` (chỉ kiểm user và company tồn tại).
- **Kịch bản:** chủ tenant A gắn một `user_id` đã biết (từ lỗi 409 khi mời) vào công ty A ở trạng thái `active` không cần đồng ý của người đó; mở đường cho pre-hijack, và người đó nhìn thấy dữ liệu công ty.
- **Sửa:** allowlist `status` (chỉ `invited`/`pending`); đường chính là luồng mời + chấp nhận; xem xét bỏ route `POST /admin/memberships` khỏi tenant.
- **Quyết định:** giữ route hay thay bằng luồng mời; ảnh hưởng FE (`membershipAdminApi`).

### RP-15 (P3, S): Guard FE rộng hơn BE
- **Cũ:** ROLE-11 (bản risk report) + API-06 + ghi chú `ApprovalsPanel`.
- **Vị trí (repo `cobo_web_design`):** `src/config/menuPermissionMatrix.ts:43` (`admin: ['system.settings','rbac.manage','admin.membership.invite']` dùng chung cho users/roles/rules/audit/departments/titles, BE cần `rbac.manage`), `:14` (`company` chấp nhận `rbac.manage`/`system.settings`, BE cần `company.view|company.edit`), `dashboard` chấp nhận `system.settings`; `src/features/admin-core/pages/AdminHubPage.tsx:16` (guard hub `['system.settings','rbac.manage']` hiển thị nút "Gán admin"/chuyển sở hữu cho người chỉ có `system.settings`, nay nhận 403); `src/features/admin-core/screens/approvals/ApprovalsPanel.tsx` (nút Hủy hiện chỉ khi `canDecide`, trong khi người yêu cầu huỷ không cần quyền).
- **Sửa:** guard `anyOf` theo từng route, phản ánh đúng cổng BE; nút quản trị công ty theo `rbac.manage`; nút Hủy theo `isRequester`.
- **Test trước:** vitest cho `useGuard`/`ApprovalsPanel`/menu matrix; kiểm bằng `npm test`, `lint`, `build`, `check:mojibake`.

### RP-16 (P3, S): Parity in-memory vs MySQL của repo test
- **Cũ:** BES-04 (INFO).
- **Vị trí:** `internal/companyaccess/infra/inmemory/admin_repository_approval.go` (khi dựng snapshot sau apply lỗi thì giữ bản đề xuất, MySQL trả lỗi); các thao tác `_ =` nuốt lỗi trong `admin_repository_versioning.go` (in-memory restore).
- **Sửa:** trả lỗi như MySQL; thêm test chạy cùng kịch bản trên cả hai repo (bảng test dùng chung).

---

## 4. Việc chéo cần chuẩn bị chung
- **Fixture dùng lại:** `newRollbackFixture`, `newMSFixture`, `newScopeSvc`, persona `fakeAuthService{decision, permissions}`; `recordingCache`, `countingRepo`, `racyRepo` trong `rbac_rollback_scope_test.go`; hạ tầng MySQL thật `newITWorld` (`infra/mysql/rbac_restore_integration_test.go`, cần `MYSQL_TEST_DSN`, hướng dẫn trong `bug-rbac-rollback-global-roles-2026-10-09/07-integration-tests.md`).
- **Baseline so sánh:** worktree tạm của commit trước thay đổi; `go test ./...` không được có fail mới (hiện có sẵn 20 dòng FAIL ở các package khác, trong đó 2 ở `companyaccess`).
- **Kiểm tra dữ liệu DEV (chỉ đọc) trước khi làm:** RP-06 (role có `platform.cms.view` thiếu `rbac.manage|system.settings`), RP-07 (`action_policy_matrix`), RP-09 (quyền trực tiếp `platform.*`/`cms.*` đang có), RP-14 (số membership `active` được tạo bằng `POST /admin/memberships`).
- **Deploy:** backend trước, FE sau; DEV được phép ghi/dọn nhưng vẫn sao lưu binary trước (`bin/*.bak-<ngày>-<tag>`), quy trình `wf-release`.

## 5. Bảng đối chiếu mã cũ → RP
| Mã cũ | Nguồn | Trạng thái | RP |
|---|---|---|---|
| ROLE-01 | risk lần 2 (CRITICAL) | **Đã sửa**, deploy DEV | — |
| ROLE-02 (= BES-01) | risk lần 2 | Mở | RP-01 |
| ROLE-03 | risk lần 2 | Mở | RP-02 |
| ROLE-04 | risk lần 2 | **Một phần**: ba route đã sửa | RP-07 (phần còn lại) |
| ROLE-05 | risk lần 2 | Mở | RP-04 |
| ROLE-06 (= H3 + H17 + CACHE-01) | risk lần 2 / lần 1 | Mở; invalidate đã thêm cho ba route admin công ty | RP-03 |
| ROLE-07 | risk lần 2 | Mở | RP-05 |
| ROLE-08 | risk lần 2 | Mở (plausible) | RP-06 |
| ROLE-09 | risk lần 2 | Mở | RP-14 |
| ROLE-10 | risk lần 2 | Mở | RP-11 |
| ROLE-11 (bản risk report) | risk lần 2 | Mở | RP-15 |
| ROLE-11 (bản review ROLE-01: grant policy khi restore) | review ROLE-01 | **Đã sửa** | — (**trùng số** với dòng trên) |
| ROLE-12 (critical lệch grant policy) | review ROLE-01 | **Một phần**: chỉ rollback | RP-08 |
| ROLE-13 (đồng bộ quyền trực tiếp không giới hạn) | review ROLE-01 | **Đã sửa** | — |
| ROLE-14 (snapshot sau apply) | review ROLE-01 | **Đã sửa** | — |
| ROLE-15 (TOCTOU kế hoạch) | review ROLE-01 | **Đã sửa** (khoá + kế hoạch trong tx + dấu vân tay) | — |
| ROLE-16 (ngữ nghĩa duyệt, INFO) | review ROLE-01 | Đã xử lý bằng người duyệt A1 | — |
| ROLE-17 (notification qua queue) | review follow-up | **Đã sửa** | — |
| ROLE-18 (quyền mời qua approval/rollback) | review follow-up | **Đã sửa** | — |
| ROLE-19 (approval hội tụ cả ma trận) | review follow-up | **Đã sửa** | — |
| ROLE-20 | review follow-up | Mở (chính sách) | RP-12 |
| ROLE-21 | review follow-up | Mở | RP-09 |
| ROLE-22 | review follow-up | **Một phần**: đã invalidate cache | RP-10 (chuyển sở hữu atomic) |
| ROLE-23 (= BES-05) | review follow-up | Mở (chính sách) | RP-13 |
| BES-04 (parity) | review follow-up | Mở (INFO) | RP-16 |
| API-06 (nút FE) | review follow-up | Mở | RP-15 |

## 6. Cách bắt đầu phiên sau
1. Đọc file này, `bug-rbac-rollback-global-roles-2026-10-09/08-followup-completion.md` (những gì đã làm) và memory `dev-data-is-sample`.
2. Chạy lại baseline: `go build ./... && go test ./internal/companyaccess/... -count=1` (kỳ vọng đúng 2 fail có sẵn) và, nếu cần test tích hợp, dựng MySQL local theo `07-integration-tests.md`.
3. Lập plan chi tiết cho **PR-A** (RP-01, 02, 04, 05, 10, 11) bằng `/wf-bugfix` hoặc skill `plan`; chốt trước các quyết định của RP-02 (quy tắc tập con quyền) và RP-04 (primary admin tự hạ mình).
4. RP-03 cần quyết định xoá mềm so với cascade và nên có plan riêng (L, đụng schema/phiên).
