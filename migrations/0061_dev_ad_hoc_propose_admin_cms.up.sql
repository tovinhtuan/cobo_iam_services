-- 0061: Dev/smoke — grant ad-hoc proposal permissions to admin.dn and cms.operator.
-- admin.dn (m_102 / admin_doanh_nghiep) is backfilled idempotently; cms.operator (m_106 / cms_operator) is new.

SET NAMES utf8mb4;

INSERT INTO role_permissions (role_id, permission_id, status)
SELECT r.role_id, p.permission_id, 'active'
FROM roles r
INNER JOIN permissions p ON p.permission_code IN (
  'ad_hoc_alert.read',
  'ad_hoc_alert.propose'
)
WHERE r.company_id = 'c_001'
  AND r.role_code IN ('admin_doanh_nghiep', 'cms_operator')
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT IGNORE INTO membership_direct_permissions
  (membership_id, company_id, permission_code, granted_by)
VALUES
  ('m_102', 'c_001', 'ad_hoc_alert.read', 'system_migration_0061'),
  ('m_102', 'c_001', 'ad_hoc_alert.propose', 'system_migration_0061'),
  ('m_106', 'c_001', 'ad_hoc_alert.read', 'system_migration_0061'),
  ('m_106', 'c_001', 'ad_hoc_alert.propose', 'system_migration_0061');
