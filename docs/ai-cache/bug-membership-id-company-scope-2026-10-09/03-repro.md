# C5: Gate R (tái hiện), 2026-10-09

- Baseline: HEAD `3203961` (đã gồm C4).
- Test: `internal/companyaccess/app/admin_service_membership_scope_test.go`, chạy **trước khi sửa code**.
- Fixture: hai company (`c_001` của người gọi, `c_002`); mỗi company có một membership kèm role, phòng ban, chức danh, quyền trực tiếp, thành viên team. In-memory repo được nâng cấp để có state cho team và đếm thành viên, theo đúng ngữ nghĩa SQL hiện tại.
- Lệnh: `go test ./internal/companyaccess/app/ -run TestMembershipScope_ -count=1`

## Kết quả (trước khi sửa)
- Nhóm "membership của company khác" (hai persona: `tenantAdmin` có `rbac.manage`, `inviteOnly` chỉ có `admin.membership.invite`): **26 test FAIL**. Mọi hàm đều không trả 404 `MEMBERSHIP_NOT_FOUND` và làm đổi dữ liệu của membership đó (trạng thái, role, phòng ban, chức danh, quyền trực tiếp, thành viên team, hoặc xoá membership).
  - Hàm được kiểm: `UpdateMembership`, `DeleteMembership`, `AssignRole`, `RemoveRole`, `AssignDepartment`, `RemoveDepartment`, `AssignTitle`, `RemoveTitle`, `AddDeptMember`, `RemoveDeptMember`, `UpdateMembershipOrgAssignments` (cả hai persona); `AddDirectPermission`, `RemoveDirectPermission`, `ListDirectPermissions`, `AddTeamMember`, `RemoveTeamMember`, `RemoveTitleMember` (persona `tenantAdmin`).
- Nhóm "membership của chính company": **0 FAIL**, tức hành vi hợp lệ được khoá lại trước khi sửa.
- `TestMembershipScope_NotFoundDoesNotRevealExistence` FAIL: membership của company khác khác với membership không tồn tại.
- Ghi chú: `AssignRole` chỉ chạy với membership chưa có role chính (quy tắc "một role chính" hiện có), nên case này gỡ role khởi tạo qua repo trước khi chụp snapshot.

## Output
```
--- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/tenantAdmin/UpdateMembership
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/tenantAdmin/DeleteMembership
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/tenantAdmin/AssignRole
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/tenantAdmin/RemoveRole
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/tenantAdmin/AssignDepartment
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/tenantAdmin/RemoveDepartment
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/tenantAdmin/AssignTitle
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/tenantAdmin/RemoveTitle
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/tenantAdmin/AddDirectPermission
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/tenantAdmin/RemoveDirectPermission
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/tenantAdmin/ListDirectPermissions
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/tenantAdmin/AddTeamMember
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/tenantAdmin/RemoveTeamMember
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/tenantAdmin/RemoveTitleMember
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/tenantAdmin/UpdateMembershipOrgAssignments
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/inviteOnly/UpdateMembership
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/inviteOnly/DeleteMembership
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/inviteOnly/AssignRole
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/inviteOnly/RemoveRole
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/inviteOnly/AssignDepartment
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/inviteOnly/RemoveDepartment
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/inviteOnly/AssignTitle
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/inviteOnly/RemoveTitle
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/inviteOnly/AddDeptMember
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/inviteOnly/RemoveDeptMember
    --- FAIL: TestMembershipScope_ForeignMembership_NotFoundAndUntouched/inviteOnly/UpdateMembershipOrgAssignments
--- FAIL: TestMembershipScope_NotFoundDoesNotRevealExistence
```

## T4: team, department, title (trước khi sửa)

Test: `admin_service_org_scope_test.go` (`TestOrgScope_*`). In-memory repo được dựng theo đúng ngữ nghĩa MySQL hiện tại (kể cả thứ tự xoá thành viên team trước khi kiểm company).

- `TestOrgScope_DeleteTeam` FAIL: `DeleteTeam` trả 404 nhưng thành viên của team thuộc company khác đã bị xoá.
- `TestOrgScope_CreateTeam` FAIL: tạo được team trong department của company khác.
- `TestOrgScope_TeamMembers_ForeignTeam` FAIL: thêm và gỡ thành viên ở team ngoài company không bị từ chối.
- `TestOrgScope_DeleteDepartmentAndTitle` FAIL: department/title của company khác có thành viên trả 409 `DEPARTMENT_HAS_MEMBERS`/`TITLE_HAS_MEMBERS` thay vì 404 (lộ sự tồn tại).
- Phần "company của mình" của các test này PASS (hành vi hợp lệ được khoá trước khi sửa).

```
    admin_service_org_scope_test.go:33: members of a team of another company must be untouched, got []
--- FAIL: TestOrgScope_DeleteTeam
    admin_service_org_scope_test.go:50: CreateTeam(foreign department): expected 404, got <nil>
--- FAIL: TestOrgScope_CreateTeam
    admin_service_org_scope_test.go:69: AddTeamMember(foreign team): expected 404, got <nil>
    admin_service_org_scope_test.go:71: team of another company changed: [] -> [m_own]
    admin_service_org_scope_test.go:79: RemoveTeamMember(foreign team): expected 404, got <nil>
    admin_service_org_scope_test.go:81: RemoveTeamMember touched a team of another company: []
--- FAIL: TestOrgScope_TeamMembers_ForeignTeam
    admin_service_org_scope_test.go:98: DeleteDepartment(foreign): expected 404, got STATE_CONFLICT: DEPARTMENT_HAS_MEMBERS
    admin_service_org_scope_test.go:99: DeleteTitle(foreign): expected 404, got STATE_CONFLICT: TITLE_HAS_MEMBERS
--- FAIL: TestOrgScope_DeleteDepartmentAndTitle
```
