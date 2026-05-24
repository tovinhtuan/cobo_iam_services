SET NAMES utf8mb4;

DELETE rp
FROM role_permissions rp
INNER JOIN roles r ON r.role_id = rp.role_id
INNER JOIN permissions p ON p.permission_id = rp.permission_id
WHERE r.role_code = 'admin_doanh_nghiep'
  AND p.permission_code = 'ad_hoc_alert.process_control';
