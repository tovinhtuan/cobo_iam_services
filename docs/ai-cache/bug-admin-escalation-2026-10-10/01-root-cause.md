# Root cause PR-B (confirmed)

| Finding | Root cause | Test tái hiện |
|---|---|---|
| ROLE-02 | `CreateResourceScopeRule`/`CreateWorkflowAssigneeRule` chuyển body xuống repo nguyên vẹn; repo đọc `company_id` từ body (`infra/mysql/admin_repository.go:593,637`). `CreateNotificationRule` thì đã ép theo token. | `TestCreateRules_CompanyComesFromTheToken`, `TestCreateRules_RequirePayload` |
| ROLE-03 | `validateEnterpriseInviteRole` chỉ kiểm danh sách cấm và quyền platform. Danh sách cấm thiếu `admin_doanh_nghiep`/`company_admin`, và không so quyền của role với quyền của người cấp. `validateEnterpriseInvitePermissions` chỉ chặn `rbac.manage` khi cấp trực tiếp. | `TestInvite_WithoutRbacManage_*`, `TestInvite_SystemSettingsWithoutRbacManage_CannotGrantAdminRole`, `TestReplacePrimaryRole_WithoutRbacManage_CannotGrantAdminRole` |
| ROLE-13 | `AssignRole` có `assertRoleHasNoPlatformPermissions`, nhưng các đường gỡ không có: `RemoveRole`, thay primary role, `RemoveDirectPermission` (`platform.cms.view` risk "low" nên áp dụng ngay), `SubmitConfigApproval` direct remove. | `TestRemoveRole_TenantAdminCannotRemovePlatformRole`, `TestRemoveDirectPermission_TenantAdminCannotRemovePlatformPermission`, `TestSubmitDirectRemoveApproval_TenantAdminCannotQueuePlatformPermission` |
| ROLE-05 | `RemoveRole` không có guard primary admin và không có lockout "admin cuối cùng" như `assertPrimaryRoleChangeLockout`. | `TestRemoveRole_CannotRemoveAdminRoleFromPrimaryAdmin`, `TestRemoveRole_CannotRemoveLastAdmin` |
| ROLE-12 | `RollbackNotificationRuleVersion` gọi thẳng `RestoreNotificationRuleFromSnapshot`; nhánh prefs của `UpdateNotificationRule` (validate + entitlement + approval) không được áp dụng. `DeleteNotificationRule` không phân biệt rule prefs. | `TestNotificationRollback_PrefsRuleIsRoutedToApproval`, `TestNotificationRollback_PremiumPrefsAfterDowngradeIsRefused`, `TestDeleteNotificationRule_PrefsRuleIsRefused` |
| BES-09 | `ApproveConfigApproval` chỉ validate payload prefs, không gọi `ValidateAlertChannelPrefsMutation`. | `TestApprovePrefs_PremiumAfterDowngradeIsRefused` |
| ROLE-14 | `ApproveEmergencyAccessRequest` chỉ chặn requester, không chặn target. | `TestBreakGlass_TargetCannotApprove` |

Không có dữ liệu bị hỏng do code. Nhưng nếu ROLE-02/ROLE-03 đã từng bị khai thác thì trên DB có thể còn dấu vết. Truy vấn kiểm tra chỉ đọc (chưa chạy):

```sql
-- Rule có created_by là membership của company khác
SELECT r.rule_id, r.company_id, m.company_id AS creator_company
FROM resource_scope_rules r JOIN memberships m ON m.membership_id = r.created_by
WHERE r.company_id <> m.company_id;

-- Membership có rbac.manage được tạo bởi người không có rbac.manage
-- Cần đối chiếu audit_logs action admin.user.invite / admin.membership.create.
```

Ghi chú: bảng `workflow_assignee_rules` và cột `created_by` cần kiểm lại schema trước khi chạy.
