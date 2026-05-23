SET NAMES utf8mb4;

DELETE FROM role_default_grant_permissions
WHERE permission_code IN (
  'cms.template.read',
  'cms.template.write',
  'cms.template.activate',
  'cms.template.archive',
  'cms.template.config.write'
);

DELETE rp
FROM role_permissions rp
INNER JOIN permissions p
  ON p.permission_id = rp.permission_id
WHERE p.permission_code IN (
  'cms.template.read',
  'cms.template.write',
  'cms.template.activate',
  'cms.template.archive',
  'cms.template.config.write'
);

DELETE FROM permissions
WHERE permission_code IN (
  'cms.template.read',
  'cms.template.write',
  'cms.template.activate',
  'cms.template.archive',
  'cms.template.config.write'
);
