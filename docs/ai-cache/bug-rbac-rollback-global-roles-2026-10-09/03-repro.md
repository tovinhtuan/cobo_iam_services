# ROLE-01: Gate R (test fail trước khi sửa)

Test: `internal/companyaccess/app/rbac_rollback_scope_test.go` (fixture `newRollbackFixture`: c_001 + c_002; role global, `cms_operator` và `admin_doanh_nghiep` protected, role `tenant_custom` của c_001 và c_002).
Chạy trên code chưa sửa logic restore (đã có T1: in-memory repo có company cho role).

```
--- PASS: TestRBACRollback_DoesNotReintroduceCMSPermission (0.00s)
    rbac_rollback_scope_test.go:208: global role g_owner lost "deadline.view" (now map[disclosure.view:true])
--- FAIL: TestRBACRollback_KeepsGlobalRolePermissions (0.00s)
    rbac_rollback_scope_test.go:219: cms_operator lost "platform.cms.view" (now map[disclosure.view:true])
    rbac_rollback_scope_test.go:220: cms_operator lost "cms.template.write" (now map[disclosure.view:true])
--- FAIL: TestRBACRollback_KeepsPlatformPermsOnTenantDefaultRole (0.00s)
    rbac_rollback_scope_test.go:233: admin_doanh_nghiep lost "dashboard.view" (now map[deadline.view:true disclosure.view:true])
    rbac_rollback_scope_test.go:234: admin_doanh_nghiep unexpectedly has "deadline.view" (now map[deadline.view:true disclosure.view:true])
--- FAIL: TestRBACRollback_DoesNotModifyProtectedTenantDefaultRole (0.00s)
--- PASS: TestRBACRollback_RestoresTenantCustomRole (0.00s)
    rbac_rollback_scope_test.go:272: c_002 direct permission was revoked by a c_001 rollback: []
--- FAIL: TestRBACRollback_DoesNotTouchOtherCompany (0.00s)
    rbac_rollback_scope_test.go:301: global role lost "deadline.view" (now map[disclosure.view:true])
    rbac_rollback_scope_test.go:303: cms_operator lost "platform.cms.view" (now map[disclosure.view:true])
    rbac_rollback_scope_test.go:304: cms_operator lost "cms.template.write" (now map[disclosure.view:true])
--- FAIL: TestRBACApprovalApply_KeepsOutOfScopeRoles (0.00s)
    rbac_rollback_scope_test.go:321: expected HTTP error 403/protected_role_read_only, got <nil>
--- FAIL: TestSubmitConfigApproval_RBACPermRemove_RejectsProtectedAndGlobalRoles (0.00s)
    rbac_rollback_scope_test.go:333: expected HTTP error 400/PERMISSION_OUT_OF_ENTERPRISE_SCOPE, got <nil>
--- FAIL: TestSubmitConfigApproval_RBACPermRemove_RejectsOutOfEnterprisePermission (0.00s)
--- PASS: TestSubmitConfigApproval_RBACPermRemove_AcceptsCustomRole (0.00s)
    rbac_rollback_scope_test.go:359: expected HTTP error 202/APPROVAL_ROUTED, got <nil>
--- FAIL: TestRBACRollback_CriticalChange_RoutesToApproval (0.00s)
--- PASS: TestRBACRollback_NonCriticalChange_AppliesDirectly (0.00s)
    rbac_rollback_scope_test.go:403: expected HTTP error 202/APPROVAL_ROUTED, got <nil>
--- FAIL: TestRBACRollback_CriticalApproval_StaleAfterNewVersion (0.00s)
```

Đọc kết quả:
- **FAIL đúng lý do (9):** role global mất quyền; `cms_operator` mất `platform.cms.view`/`cms.template.write`; role `tenant_default` bị sửa; direct permission của c_002 bị thu hồi bởi rollback của c_001; apply approval sửa role ngoài phạm vi; `SubmitConfigApproval` nhận role protected/global và quyền ngoài phạm vi doanh nghiệp; rollback có quyền critical áp dụng ngay (không 202), nên test stale cũng FAIL.
- **PASS (giữ hành vi, 3):** rollback khôi phục role `tenant_custom`; rollback không critical áp dụng trực tiếp; submit trên role custom vẫn được nhận.
