-- 0087: Backfill disclosure_type.manage into role_permissions for all admin_doanh_nghiep tenant roles.
-- Root cause: migration 0044 added the permission row but did not add it to existing role_permissions.
-- Bootstrap code before this fix only inserted into role_default_grant_permissions (invite UI hint),
-- not into role_permissions (effective access source of truth).
-- This migration is idempotent (INSERT IGNORE) and ORDER BY role_id prevents deadlock with concurrent writes.

SET NAMES utf8mb4;

-- Grant the full tenant admin permission pack to every existing admin_doanh_nghiep role.
-- Scoped to company-owned roles (company_id IS NOT NULL excludes global template roles).
-- INSERT IGNORE: safe to re-run — skips already-granted rows.
INSERT IGNORE INTO role_permissions (role_id, permission_id, status)
SELECT r.role_id, p.permission_id, 'active'
FROM roles r
CROSS JOIN permissions p
WHERE r.role_code = 'admin_doanh_nghiep'
  AND r.company_id IS NOT NULL
  AND r.status = 'active'
  AND p.permission_code IN (
      'disclosure_type.manage',
      'template.workflow.override.read',
      'template.workflow.override.write',
      'template.workflow.override.approve',
      'template.workflow.override.reset',
      'ad_hoc_alert.propose'
  )
  AND p.status = 'active'
ORDER BY r.role_id;  -- Consistent lock order prevents deadlock with concurrent role_permission writes.
