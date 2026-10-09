# C4 + H2: Gate R (tái hiện), 2026-10-09

- Baseline: HEAD `7207a95` (đã gồm fix C2 và C3).
- Test mới: `internal/companyaccess/app/admin_service_company_scope_test.go` (prefix `TestScope_`), chạy **trước khi sửa code**.
- Lệnh: `go test ./internal/companyaccess/app/ -run TestScope_ -count=1`

## Kết quả
- Mọi test dùng persona non-operator (`rbac.manage`, `system.settings`, `platform.cms.view` đơn lẻ) mà gửi company khác token **FAIL**: service không trả `403 COMPANY_SCOPE_MISMATCH`.
  - Phạm vi: `CreateUser`, `CreateMembership`, `AssignUserToCompany`, `ListCompanyMemberships` (H2, gồm nhánh không có company), `InviteUser`, `ListInviteRoles`, `ResendUserInvitation`.
- `CreateUser` với company rỗng và persona `rbac.manage` không ép về company của token.
- Các test operator dùng `system.settings` cũng FAIL vì code hiện tại chỉ xét `rbac.manage`.
- Các test operator dùng `rbac.manage` (`platform.cms.view` + `rbac.manage`) PASS, tức hành vi CMS hợp lệ được giữ khi đổi sang helper.

## Output (rút gọn)
```
--- FAIL: TestScope_CreateUser_NonOperator_OtherCompany_Forbidden
    --- FAIL: TestScope_CreateUser_NonOperator_OtherCompany_Forbidden/tenantAdmin(rbac.manage)
    --- FAIL: TestScope_CreateUser_NonOperator_OtherCompany_Forbidden/tenantAdmin(system.settings)
    --- FAIL: TestScope_CreateUser_NonOperator_OtherCompany_Forbidden/cmsViewOnly
--- FAIL: TestScope_CreateUser_NonOperator_EmptyCompany_ForcedToOwn
    --- FAIL: TestScope_CreateUser_NonOperator_EmptyCompany_ForcedToOwn/tenantAdmin(rbac.manage)
--- FAIL: TestScope_CreateUser_Operator_OtherAndNoCompany_Allowed
    --- FAIL: TestScope_CreateUser_Operator_OtherAndNoCompany_Allowed/platformOperator(system.settings)
--- FAIL: TestScope_CreateMembership_NonOperator_OtherCompany_Forbidden
    --- FAIL: TestScope_CreateMembership_NonOperator_OtherCompany_Forbidden/tenantAdmin(rbac.manage)
    --- FAIL: TestScope_CreateMembership_NonOperator_OtherCompany_Forbidden/tenantAdmin(system.settings)
    --- FAIL: TestScope_CreateMembership_NonOperator_OtherCompany_Forbidden/cmsViewOnly
--- FAIL: TestScope_CreateMembership_NonOperator_EmptyCompany_ForcedToOwn
--- FAIL: TestScope_AssignUserToCompany_NonOperator_OtherCompany_Forbidden
    --- FAIL: TestScope_AssignUserToCompany_NonOperator_OtherCompany_Forbidden/tenantAdmin(rbac.manage)
    --- FAIL: TestScope_AssignUserToCompany_NonOperator_OtherCompany_Forbidden/tenantAdmin(system.settings)
    --- FAIL: TestScope_AssignUserToCompany_NonOperator_OtherCompany_Forbidden/cmsViewOnly
--- FAIL: TestScope_ListCompanyMemberships_NonOperator
    --- FAIL: TestScope_ListCompanyMemberships_NonOperator/tenantAdmin(rbac.manage)
    --- FAIL: TestScope_ListCompanyMemberships_NonOperator/tenantAdmin(system.settings)
    --- FAIL: TestScope_ListCompanyMemberships_NonOperator/cmsViewOnly
--- FAIL: TestScope_ListCompanyMemberships_Operator
    --- FAIL: TestScope_ListCompanyMemberships_Operator/platformOperator(system.settings)
--- FAIL: TestScope_InviteUser_NonOperator
    --- FAIL: TestScope_InviteUser_NonOperator/tenantAdmin(rbac.manage)
    --- FAIL: TestScope_InviteUser_NonOperator/tenantAdmin(system.settings)
    --- FAIL: TestScope_InviteUser_NonOperator/cmsViewOnly
--- FAIL: TestScope_InviteUser_Operator_OtherAndNoCompany_Allowed
    --- FAIL: TestScope_InviteUser_Operator_OtherAndNoCompany_Allowed/platformOperator(system.settings)
--- FAIL: TestScope_ListInviteRoles_NonOperator_OtherCompany_Forbidden
    --- FAIL: TestScope_ListInviteRoles_NonOperator_OtherCompany_Forbidden/tenantAdmin(rbac.manage)
    --- FAIL: TestScope_ListInviteRoles_NonOperator_OtherCompany_Forbidden/tenantAdmin(system.settings)
    --- FAIL: TestScope_ListInviteRoles_NonOperator_OtherCompany_Forbidden/cmsViewOnly
--- FAIL: TestScope_ListInviteRoles_Operator_OtherCompany_Allowed
    --- FAIL: TestScope_ListInviteRoles_Operator_OtherCompany_Allowed/platformOperator(system.settings)
--- FAIL: TestScope_ResendUserInvitation_NoCompanyScope_NonOperator_Forbidden
    --- FAIL: TestScope_ResendUserInvitation_NoCompanyScope_NonOperator_Forbidden/tenantAdmin(rbac.manage)
--- FAIL: TestScope_ResendUserInvitation_OtherCompany_NonOperator_Forbidden
    --- FAIL: TestScope_ResendUserInvitation_OtherCompany_NonOperator_Forbidden/tenantAdmin(rbac.manage)
    --- FAIL: TestScope_ResendUserInvitation_OtherCompany_NonOperator_Forbidden/tenantAdmin(system.settings)
    --- FAIL: TestScope_ResendUserInvitation_OtherCompany_NonOperator_Forbidden/cmsViewOnly
FAIL
FAIL	github.com/cobo/cobo_iam_services/internal/companyaccess/app	0.324s
FAIL
```
