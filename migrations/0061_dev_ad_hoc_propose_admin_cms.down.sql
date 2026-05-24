SET NAMES utf8mb4;

DELETE FROM membership_direct_permissions
WHERE granted_by = 'system_migration_0061';

DELETE rp
FROM role_permissions rp
INNER JOIN roles r ON r.role_id = rp.role_id
INNER JOIN permissions p ON p.permission_id = rp.permission_id
WHERE r.company_id = 'c_001'
  AND r.role_code = 'cms_operator'
  AND p.permission_code IN ('ad_hoc_alert.read', 'ad_hoc_alert.propose');
