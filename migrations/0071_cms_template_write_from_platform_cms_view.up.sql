-- 0071: Grant cms.template.write|activate|archive to roles that already have platform.cms.view.
-- Closes gap vs 0058 (which only backfilled from disclosure_type.manage).
-- Dev cms_operator / platform.tenant.admin need write after select-company (effective-access cache).

SET NAMES utf8mb4;

INSERT IGNORE INTO role_permissions (role_id, permission_id, status)
SELECT rp.role_id, p.permission_id, 'active'
FROM role_permissions rp
INNER JOIN permissions legacy
  ON legacy.permission_id = rp.permission_id
 AND legacy.permission_code = 'platform.cms.view'
 AND legacy.status = 'active'
INNER JOIN permissions p
  ON p.permission_code IN ('cms.template.write', 'cms.template.activate', 'cms.template.archive')
 AND p.status = 'active'
WHERE rp.status = 'active';

INSERT IGNORE INTO role_default_grant_permissions (role_id, permission_code)
SELECT rdgp.role_id, 'cms.template.write'
FROM role_default_grant_permissions rdgp
WHERE rdgp.permission_code = 'platform.cms.view';

INSERT IGNORE INTO role_default_grant_permissions (role_id, permission_code)
SELECT rdgp.role_id, 'cms.template.activate'
FROM role_default_grant_permissions rdgp
WHERE rdgp.permission_code = 'platform.cms.view';

INSERT IGNORE INTO role_default_grant_permissions (role_id, permission_code)
SELECT rdgp.role_id, 'cms.template.archive'
FROM role_default_grant_permissions rdgp
WHERE rdgp.permission_code = 'platform.cms.view';
