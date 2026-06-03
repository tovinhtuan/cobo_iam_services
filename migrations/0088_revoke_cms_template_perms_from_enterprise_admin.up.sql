-- 0088: Revoke cms.template.* from roles that do NOT have platform.cms.view.
-- Root cause: migration 0058 mapped disclosure_type.manage → cms.template.write/activate/archive
-- as a "legacy compatibility" backfill. This incorrectly gave enterprise admin (admin_doanh_nghiep)
-- CMS permissions that are intended only for platform admin (requires platform.cms.view gate).
-- Business contract: cms.template.* is CMS-only; enterprise admin must not hold these.
-- Impact: functionally harmless (backend gates on platform.cms.view first), but violates
-- least-privilege and pollutes effective-access responses for enterprise admins.
-- Safe to re-run: DELETE WHERE is idempotent.
-- Note: Wraps subquery in derived table to avoid MySQL ERROR 1093
--       (can't specify target table for update in FROM clause).

SET NAMES utf8mb4;

DELETE rp
FROM role_permissions rp
INNER JOIN permissions cms_perm
  ON cms_perm.permission_id = rp.permission_id
 AND cms_perm.permission_code IN (
   'cms.template.write',
   'cms.template.activate',
   'cms.template.archive',
   'cms.template.config.write'
 )
WHERE rp.role_id NOT IN (
  SELECT role_id FROM (
    SELECT rp2.role_id
    FROM role_permissions rp2
    INNER JOIN permissions plat
      ON plat.permission_id = rp2.permission_id
     AND plat.permission_code = 'platform.cms.view'
    WHERE rp2.status = 'active'
  ) AS roles_with_platform_cms
);
