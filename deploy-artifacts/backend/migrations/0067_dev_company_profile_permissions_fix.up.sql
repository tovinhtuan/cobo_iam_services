-- 0067: Fix broken 0066 when it reused permission_id 000e/000f (already template.workflow.override.*).
-- Safe to run even if 0066 was never applied.

SET NAMES utf8mb4;

INSERT INTO permissions (permission_id, permission_code, permission_name, module_name, status) VALUES
  ('10000000-0001-4000-8000-000000000017', 'company.view', 'View company profile', 'company', 'active'),
  ('10000000-0001-4000-8000-000000000018', 'company.edit', 'Edit company profile', 'company', 'active')
ON DUPLICATE KEY UPDATE
  permission_code = VALUES(permission_code),
  permission_name = VALUES(permission_name),
  module_name = VALUES(module_name),
  status = VALUES(status);

INSERT INTO role_permissions (role_id, permission_id, status)
SELECT r.role_id, p.permission_id, 'active'
FROM roles r
INNER JOIN permissions p ON p.permission_code = 'company.view' AND p.status = 'active'
WHERE r.status = 'active'
  AND r.role_code IN (
    'admin_doanh_nghiep',
    'truong_phong_ban',
    'truong_nhom',
    'user_thuong',
    'cms_operator',
    'admin_web',
    'full_access'
  )
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT INTO role_permissions (role_id, permission_id, status)
SELECT r.role_id, p.permission_id, 'active'
FROM roles r
INNER JOIN permissions p ON p.permission_code = 'company.edit' AND p.status = 'active'
WHERE r.status = 'active'
  AND r.role_code IN ('admin_doanh_nghiep', 'admin_web', 'full_access')
ON DUPLICATE KEY UPDATE status = VALUES(status);
