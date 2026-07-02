-- 0110: Remove remaining CMS/Platform blocked permissions from enterprise roles.
-- Extends migration 0088 which covered cms.template.write/activate/archive/config.write.
-- This migration covers the remaining three blocked codes:
--   cms.template.read          (seeded by migration 0034/0058)
--   disclosure_type.config.read  (seeded by migration 0034/0009)
--   disclosure_type.config.write (seeded by migration 0034/0009)
--
-- Safety invariant (same as 0088):
--   Roles that hold 'platform.cms.view' are CMS/Platform roles — their permissions are kept.
--   Roles that do NOT hold 'platform.cms.view' are enterprise roles — blocked codes removed.
--
-- 'platform.cms.view' itself is NOT touched: it remains in CMS roles as the access gate.
--
-- Safe to re-run: DELETE WHERE is idempotent.
-- MySQL note: subquery wrapped in derived table to avoid ERROR 1093.

SET NAMES utf8mb4;

DELETE rp
FROM role_permissions rp
INNER JOIN permissions blocked_perm
  ON blocked_perm.permission_id = rp.permission_id
 AND blocked_perm.permission_code IN (
   'cms.template.read',
   'disclosure_type.config.read',
   'disclosure_type.config.write'
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
