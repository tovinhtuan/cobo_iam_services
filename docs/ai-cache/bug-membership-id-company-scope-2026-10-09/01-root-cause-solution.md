# C5: Root cause và solution

## Root cause (confirmed)
Hai thiếu sót cộng dồn trong `internal/companyaccess`:
1. **Tầng service không kiểm tra membership đích thuộc company của token.** `authorize(..., membershipID)` không giúp được vì authorizer bỏ qua resource (xem C4). `authorizeScopedMembershipMutation` (`admin_delegation_scope.go:125`) trả `nil` ngay khi scope là "company". Helper đúng (`requireMembershipInCompany`, `admin_service_departments.go:129`) chỉ được dùng ở 5 nơi.
2. **Tầng repo không ràng buộc `company_id`** ở các câu ghi/đọc theo `membership_id` (`UpdateMembershipStatus`, `DeleteMembership`, `RemoveRole`, `RemoveTitle`, `RemoveDepartment`, `RevokeDirectPermission`, `ListActiveDirectPermissions`, `RemoveTeamMember`) và `DeleteTeamRow` xoá bảng con trước khi kiểm company.

Layer: API service và repo. Không phải FE, không phải config.

## Quy tắc thiết kế
> Mọi route tenant nhận `membership_id` chỉ thao tác trên membership thuộc `Subject.CompanyID`. Membership ở company khác trả **404 `MEMBERSHIP_NOT_FOUND`** (như `requireMembershipInCompany` hiện có), không trả 403, để không lộ sự tồn tại của id.

- Không cần ngoại lệ cho platform operator: toàn bộ hàm bị ảnh hưởng chỉ có `AdminHandler` tenant gọi tới (đã grep: `platformcms` không gọi). Operator làm việc cross-company qua `/api/v1/platform/cms/*`.
- Lý do 404 thay vì 403 (khác C4): ở C4 client gửi **company_id** và 403 `COMPANY_SCOPE_MISMATCH` giúp lỗi lộ sớm; ở C5 client gửi **id của đối tượng**, nên "không tìm thấy trong company này" là phản hồi đúng và không tạo oracle tồn tại.

## Thiết kế thay đổi (chỉ `cobo_iam_services`)

### Lớp 1 — service guard (bắt buộc, không đổi chữ ký repo)
1. Thêm helper trong `admin_service_company_scope.go`: `requireTargetMembership(ctx, sub, membershipID)` = `requireMembershipInCompany(ctx, membershipID, sub.CompanyID)` (kèm fail-closed khi `sub.CompanyID` rỗng, giống `resolveTargetCompany`).
2. **Gom về một điểm** cho nhóm đã dùng `authorizeScopedMembershipMutation`: gọi `requireTargetMembership` ở đầu hàm này (trước nhánh scope). Một thay đổi phủ: `UpdateMembership`, `DeleteMembership`, `ReplaceMembershipPrimaryRole`, `AssignDepartment`, `RemoveDepartment`, `UpdateMembershipOrgAssignments`, nhánh delegation của `AddDeptMember`/`RemoveDeptMember`.
3. **Thêm lời gọi tường minh** (sau `authorize`/`requireRbacManage`, trước mọi lookup hay ghi) ở các hàm chưa đi qua helper trên: `AssignRole`, `RemoveRole`, `AssignTitle`, `RemoveTitle`, `AddDirectPermission`, `RemoveDirectPermission`, `ListDirectPermissions`, `RevokeCompanyAdmin`, `AddTeamMember`, `RemoveTeamMember`, `RemoveTitleMember`.
4. **Validate theo company của token, không theo company của đích:** `ReplaceMembershipPrimaryRole` và `UpdateMembershipOrgAssignments` đổi từ `member.CompanyID` sang `sub.CompanyID` (sau bước 2 hai giá trị bằng nhau; đổi để về sau không lệch).
5. `SubmitConfigApproval` loại `RBAC_DIRECT_PERM_REMOVE`: `requireTargetMembership` với `membership_id` trong body ngay khi submit.
6. `CreateTeam`: validate `department_id` thuộc company (repo đã có kiểu `DepartmentBelongsToCompany`; xác nhận tên khi implement). `AddTeamMember`: validate team thuộc company (thêm `OrgUnitBelongsToCompany` hoặc dùng lại hàm hiện có).

### Lớp 2 — repo hardening cho thao tác phá huỷ và các lỗi riêng lẻ (nên làm cùng PR)
- `DeleteTeamRow`: bọc trong transaction, kiểm `org_units` thuộc company **trước**, rồi mới xoá `org_unit_memberships` (hoặc `DELETE oum FROM org_unit_memberships oum JOIN org_units ou … AND ou.company_id=?`).
- `RemoveTeamMember`: dùng tham số company đang bị bỏ qua (`_`) qua join với `org_units`.
- `CountTeamsInDepartment`, `CountDepartmentMembers`, `CountTitleMembers`: thêm lọc company, và kiểm tra tồn tại theo company **trước** khi đếm để id ngoài company trả 404 thay vì 409.
- `UpdateMembershipStatus`, `DeleteMembership`: thêm tham số `companyID` và điều kiện `company_id` (hai thao tác phá huỷ nhất). Đổi chữ ký interface `AdminRepository` ở `app/admin.go`, hai implementer (`infra/mysql`, `infra/inmemory`) và các fake trong test.
- Các hàm theo `membership_id` còn lại (`RemoveRole`, `RemoveTitle`, `RemoveDepartment`, `RevokeDirectPermission`, `ListActiveDirectPermissions`): giữ nguyên chữ ký ở PR này; đã được lớp 1 chặn. Ghi follow-up nếu muốn hardening toàn bộ.

### Không làm trong PR này
- Sửa authorizer để so company của resource (gốc chung của C4 và C5): thay đổi hệ thống, cần thiết kế riêng. Lớp 1 và 2 làm cho phần quan trọng không còn phụ thuộc vào authorizer.
- `CreateMembership` gắn user bất kỳ vào company của mình (BES-02): thuộc hướng "thay bằng luồng mời", ngoài phạm vi.

## Kế hoạch test (viết trước khi sửa, Gate R)
Mẫu theo `admin_service_company_scope_test.go` (C4): `fakeAuthService` với persona `tenantAdmin` (`rbac.manage`), `tenantAdminSys`, một persona **delegation/department-scoped**, và `platformOperator`.

1. **Test bảng cross-company cho mọi hàm ở nhóm A** (khoảng trống lớn nhất hiện nay): dựng hai company (`c_001` của người gọi, `c_002` có một membership thật), gọi từng hàm với membership của `c_002`:
   - kỳ vọng 404 `MEMBERSHIP_NOT_FOUND`;
   - kỳ vọng **không thay đổi dữ liệu**: trạng thái, role, department, title, direct permission, team member của membership `c_002` giữ nguyên (đọc lại qua repo).
2. **Test giữ hành vi:** cùng hàm với membership của chính company mình vẫn chạy như cũ (đã có nhiều test; thêm vài case đại diện).
3. **Nhánh delegation/department-scope** của `AddDeptMember`/`RemoveDeptMember`: membership ngoài company bị 404 trước khi xét scope.
4. **Team/department/title:** id của company khác trả 404 (không còn 409 oracle); `DeleteTeam` với team_id của company khác **không xoá** member của team đó (kiểm bằng repo).
5. **Config approval** `RBAC_DIRECT_PERM_REMOVE` với `membership_id` ngoài company → từ chối khi submit.
6. **Test quét route ở handler:** bảng mọi route tenant có `{membership_id}`, gọi bằng token `c_001` với id của `c_002` → 404; thêm assert đếm route để route mới phải được khai báo.
7. **Kiểm tra ngược:** gỡ guard ở `authorizeScopedMembershipMutation` và ở một hàm tường minh → test phải fail lại.
- Đổi tên hoặc cập nhật test cũ nếu dùng membership giả không tồn tại trong repo (guard mới đòi membership phải có thật trong company).

## Verify (Gate V)
- `go test ./... -count=1` so với baseline HEAD (worktree tạm), không có fail mới; `go vet ./...`; `go test -race ./internal/companyaccess/...`; `docker compose -f docker-compose.dev.yml build api`.
- Grep: mọi hàm trong `internal/companyaccess/app` nhận `MembershipID` đều có guard hoặc nằm trong danh sách ngoại lệ có chú thích.
- Smoke DEV (khi được yêu cầu deploy): token `c_001` thao tác với membership/team thật của `c_002` (đọc trước, rồi các request kỳ vọng bị từ chối, giống mẫu C4) → 404; thao tác trong company mình vẫn chạy; màn `/app/admin/users` (gán role, phòng ban, chức danh) vẫn hoạt động bằng Playwright chỉ đọc.

## Blast radius
- **FE:** mọi caller tenant gửi id lấy từ danh sách của company đang chọn, nên không đổi. Rủi ro duy nhất là tab cũ sau khi đổi company (giống API-01 của C4): id của company cũ nay nhận 404.
- **Hợp đồng:** route trả 404 `MEMBERSHIP_NOT_FOUND` thay vì thành công cho id ngoài company; message tiếng Anh sẵn có. Cập nhật `docs/api-contracts-json.md`.
- **Dữ liệu:** không có migration. Audit DEV (C4, Q3) cho 0 dòng bị tác động; cần bổ sung query cho team/title/department/direct permission nếu có audit log.
- **Rollback:** khôi phục binary cũ sẽ mở lại khoảng trống.

## Thứ tự thực hiện đề xuất
1. T1: helper `requireTargetMembership` + test bảng (Gate R, FAIL trước).
2. T2: guard tập trung trong `authorizeScopedMembershipMutation` (phủ 7 hàm).
3. T3: guard tường minh cho 11 hàm còn lại + `RevokeCompanyAdmin`.
4. T4: team/department/title (validate, count, 404 thay 409, `DeleteTeamRow` trong transaction).
5. T5: `UpdateMembershipStatus`/`DeleteMembership` ràng buộc `company_id` ở SQL (đổi chữ ký).
6. T6: config approval và validate theo `sub.CompanyID`.
7. T7: test quét route handler; cập nhật test cũ.
8. T8: verify, T9: review (be-security, admin-role, api-compat) + ai-cache; deploy và smoke chỉ khi được yêu cầu.

## Quyết định cần user
1. Phản hồi cho membership/đối tượng ngoài company: **404 `MEMBERSHIP_NOT_FOUND`** (khuyến nghị, không lộ tồn tại) hay 403 `COMPANY_SCOPE_MISMATCH` như C4?
2. Phạm vi PR: **lớp 1 + lớp 2** (khuyến nghị: service guard cộng sửa `DeleteTeamRow`, count, `UpdateMembershipStatus`/`DeleteMembership`) hay chỉ lớp 1 để diff nhỏ?
3. Có cho phép chạy thêm query read-only trên DB DEV cho team/title/department/direct permission (kiểm dấu vết thao tác chéo company, nếu có audit) không?
4. Có gộp xử lý các khuyết điểm team/department/title (mục C trong `00-report.md`) vào cùng PR không (khuyến nghị: có, cùng họ lỗi và cùng test)?

## Quyết định đã chốt (user, 2026-10-09)
1. Đối tượng ngoài company → **404 `MEMBERSHIP_NOT_FOUND`**.
2. Phạm vi PR: **lớp 1 + lớp 2**.
3. Đã cho phép và đã chạy query read-only trên DB DEV (kết quả: `02-data-audit.md`, toàn bộ sạch).
4. **Gộp** các khuyết điểm team, department, title vào cùng PR.

**Đính chính (khi lập plan):** đề xuất "bổ sung audit cho thao tác phá huỷ" là không cần. Đã đọc code: mọi handler phá huỷ (`deleteMembership`, `removeRole`, `removeTitle`, `removeDepartment`, thành viên team/title, ...) **đã có `auditLog`**. DEV không có event cho các thao tác này vì chưa ai dùng trên DEV, không phải do thiếu audit.
