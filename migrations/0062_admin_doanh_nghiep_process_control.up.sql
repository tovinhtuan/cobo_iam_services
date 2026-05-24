-- 0062: Company admin may act as ad-hoc process controller (including self-assign when creating proposals).

SET NAMES utf8mb4;

INSERT INTO role_permissions (role_id, permission_id, status)
SELECT r.role_id, p.permission_id, 'active'
FROM roles r
INNER JOIN permissions p ON p.permission_code = 'ad_hoc_alert.process_control' AND p.status = 'active'
WHERE r.role_code = 'admin_doanh_nghiep'
ON DUPLICATE KEY UPDATE status = VALUES(status);
