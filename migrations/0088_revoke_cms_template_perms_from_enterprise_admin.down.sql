-- 0088 down: Restore cms.template.* grants to roles with disclosure_type.manage (reverting to 0058 state).
-- Only restores rows that 0058 originally created (i.e., roles with disclosure_type.manage that
-- do not have platform.cms.view — the exact set that was deleted by 0088 up).

SET NAMES utf8mb4;

INSERT IGNORE INTO role_permissions (role_id, permission_id, status)
SELECT rp.role_id, cms_perm.permission_id, 'active'
FROM role_permissions rp
INNER JOIN permissions legacy
  ON legacy.permission_id = rp.permission_id
 AND legacy.permission_code = 'disclosure_type.manage'
 AND legacy.status = 'active'
CROSS JOIN permissions cms_perm
WHERE cms_perm.permission_code IN (
  'cms.template.write',
  'cms.template.activate',
  'cms.template.archive',
  'cms.template.config.write'
)
AND cms_perm.status = 'active'
AND rp.status = 'active'
AND NOT EXISTS (
  SELECT 1
  FROM role_permissions rp2
  INNER JOIN permissions plat
    ON plat.permission_id = rp2.permission_id
   AND plat.permission_code = 'platform.cms.view'
  WHERE rp2.role_id = rp.role_id
    AND rp2.status = 'active'
);
