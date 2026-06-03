-- 0090: Seed disclosure_type.publish permission and backfill all admin_doanh_nghiep roles.
--
-- Context: disclosure_type.publish (checker) was added as a maker-checker architectural decision
-- when lifecycle submit-review/publish was split. 0044 only created disclosure_type.manage (maker).
-- This migration adds the checker counterpart so publish/reject/archive work on fresh environments.
--
-- INSERT IGNORE with stable UUID: idempotent and safe to re-run.

SET NAMES utf8mb4;

INSERT IGNORE INTO permissions (permission_id, permission_code, permission_name, module_name, status)
VALUES (
  '788317a6-55b8-11f1-821a-563579f75001',
  'disclosure_type.publish',
  'Phê duyệt và ban hành template CBTT',
  'disclosure',
  'active'
);

-- Backfill all existing admin_doanh_nghiep roles (mirrors 0087 pattern).
-- INSERT IGNORE: safe to re-run — skips already-granted rows.
-- ORDER BY role_id: consistent lock order prevents deadlock with concurrent writes.
INSERT IGNORE INTO role_permissions (role_id, permission_id, status)
SELECT r.role_id, p.permission_id, 'active'
FROM roles r
CROSS JOIN permissions p
WHERE r.role_code = 'admin_doanh_nghiep'
  AND r.company_id IS NOT NULL
  AND r.status = 'active'
  AND p.permission_code = 'disclosure_type.publish'
  AND p.status = 'active'
ORDER BY r.role_id;
