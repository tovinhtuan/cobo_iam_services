-- Revert 0071 grants (roles that received write via platform.cms.view only, not disclosure_type.manage).

SET NAMES utf8mb4;

DELETE rp
FROM role_permissions rp
INNER JOIN permissions p
  ON p.permission_id = rp.permission_id
 AND p.permission_code IN ('cms.template.write', 'cms.template.activate', 'cms.template.archive')
WHERE EXISTS (
  SELECT 1
  FROM role_permissions rp_view
  INNER JOIN permissions legacy
    ON legacy.permission_id = rp_view.permission_id
   AND legacy.permission_code = 'platform.cms.view'
  WHERE rp_view.role_id = rp.role_id
    AND rp_view.status = 'active'
)
AND NOT EXISTS (
  SELECT 1
  FROM role_permissions rp_manage
  INNER JOIN permissions legacy_manage
    ON legacy_manage.permission_id = rp_manage.permission_id
   AND legacy_manage.permission_code = 'disclosure_type.manage'
  WHERE rp_manage.role_id = rp.role_id
    AND rp_manage.status = 'active'
);

DELETE FROM role_default_grant_permissions
WHERE permission_code IN ('cms.template.write', 'cms.template.activate', 'cms.template.archive')
  AND role_id IN (
    SELECT role_id
    FROM role_default_grant_permissions
    WHERE permission_code = 'platform.cms.view'
  );
