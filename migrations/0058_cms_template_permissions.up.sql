-- 0058: CMS-BE-AUTHZ foundation â€” additive CMS template permission family
-- Adds canonical CMS template permissions while preserving legacy compatibility.
-- Ref: portal-template-cms-fe-be-implementation-plan-2026-05-23.md Phase 1

SET NAMES utf8mb4;

INSERT INTO permissions (permission_id, permission_code, permission_name, module_name, status)
VALUES
  (UUID(), 'cms.template.read', 'Read CMS disclosure templates', 'cms', 'active'),
  (UUID(), 'cms.template.write', 'Create and update CMS disclosure templates', 'cms', 'active'),
  (UUID(), 'cms.template.activate', 'Activate CMS disclosure template versions', 'cms', 'active'),
  (UUID(), 'cms.template.archive', 'Archive CMS disclosure template versions', 'cms', 'active'),
  (UUID(), 'cms.template.config.write', 'Update CMS disclosure template config', 'cms', 'active')
ON DUPLICATE KEY UPDATE
  permission_name = VALUES(permission_name),
  module_name = VALUES(module_name),
  status = VALUES(status);

INSERT IGNORE INTO role_permissions (role_id, permission_id, status)
SELECT rp.role_id, p.permission_id, 'active'
FROM role_permissions rp
INNER JOIN permissions legacy_perm
  ON legacy_perm.permission_id = rp.permission_id
INNER JOIN permissions p
  ON p.permission_code = 'cms.template.read'
WHERE legacy_perm.permission_code = 'platform.cms.view'
  AND rp.status = 'active';

INSERT IGNORE INTO role_permissions (role_id, permission_id, status)
SELECT rp.role_id, p.permission_id, 'active'
FROM role_permissions rp
INNER JOIN permissions legacy_perm
  ON legacy_perm.permission_id = rp.permission_id
INNER JOIN permissions p
  ON p.permission_code = 'cms.template.write'
WHERE legacy_perm.permission_code = 'disclosure_type.manage'
  AND rp.status = 'active';

INSERT IGNORE INTO role_permissions (role_id, permission_id, status)
SELECT rp.role_id, p.permission_id, 'active'
FROM role_permissions rp
INNER JOIN permissions legacy_perm
  ON legacy_perm.permission_id = rp.permission_id
INNER JOIN permissions p
  ON p.permission_code = 'cms.template.activate'
WHERE legacy_perm.permission_code = 'disclosure_type.manage'
  AND rp.status = 'active';

INSERT IGNORE INTO role_permissions (role_id, permission_id, status)
SELECT rp.role_id, p.permission_id, 'active'
FROM role_permissions rp
INNER JOIN permissions legacy_perm
  ON legacy_perm.permission_id = rp.permission_id
INNER JOIN permissions p
  ON p.permission_code = 'cms.template.archive'
WHERE legacy_perm.permission_code = 'disclosure_type.manage'
  AND rp.status = 'active';

INSERT IGNORE INTO role_permissions (role_id, permission_id, status)
SELECT rp.role_id, p.permission_id, 'active'
FROM role_permissions rp
INNER JOIN permissions legacy_perm
  ON legacy_perm.permission_id = rp.permission_id
INNER JOIN permissions p
  ON p.permission_code = 'cms.template.config.write'
WHERE legacy_perm.permission_code IN ('disclosure_type.manage', 'rbac.manage')
  AND rp.status = 'active';

INSERT IGNORE INTO role_default_grant_permissions (role_id, permission_code)
SELECT role_id, 'cms.template.write'
FROM role_default_grant_permissions
WHERE permission_code = 'disclosure_type.manage';

INSERT IGNORE INTO role_default_grant_permissions (role_id, permission_code)
SELECT role_id, 'cms.template.activate'
FROM role_default_grant_permissions
WHERE permission_code = 'disclosure_type.manage';

INSERT IGNORE INTO role_default_grant_permissions (role_id, permission_code)
SELECT role_id, 'cms.template.archive'
FROM role_default_grant_permissions
WHERE permission_code = 'disclosure_type.manage';

INSERT IGNORE INTO role_default_grant_permissions (role_id, permission_code)
SELECT role_id, 'cms.template.config.write'
FROM role_default_grant_permissions
WHERE permission_code = 'disclosure_type.manage';
