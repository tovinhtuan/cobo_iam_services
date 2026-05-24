-- 0064: Ensure platform.tenant.admin (m_107/m_108) can appear as ad-hoc process controller.
-- Covers dev servers where 0062 was not applied before 0063.

SET NAMES utf8mb4;

INSERT INTO role_permissions (role_id, permission_id, status)
SELECT r.role_id, p.permission_id, 'active'
FROM roles r
INNER JOIN permissions p ON p.permission_code = 'ad_hoc_alert.process_control' AND p.status = 'active'
WHERE r.role_code = 'admin_doanh_nghiep'
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT IGNORE INTO membership_direct_permissions
  (membership_id, company_id, permission_code, granted_by)
VALUES
  ('m_107', 'c_001', 'ad_hoc_alert.process_control', 'system_migration_0064'),
  ('m_108', 'c_002', 'ad_hoc_alert.process_control', 'system_migration_0064');
