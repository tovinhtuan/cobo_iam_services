-- 0074: deadline.view was never seeded in dev identity (only deadline.create/manage/assign).
-- Portal tab «Cảnh báo thời hạn» + GET /company/deadline-alerts require deadline.view.

SET NAMES utf8mb4;

INSERT INTO permissions (permission_id, permission_code, permission_name, module_name, status) VALUES
  ('10000000-0001-4000-8000-000000000019', 'deadline.view', 'View deadline alerts', 'deadline', 'active')
ON DUPLICATE KEY UPDATE
  permission_code = VALUES(permission_code),
  permission_name = VALUES(permission_name),
  module_name = VALUES(module_name),
  status = VALUES(status);

INSERT INTO role_permissions (role_id, permission_id, status)
SELECT r.role_id, p.permission_id, 'active'
FROM roles r
INNER JOIN permissions p ON p.permission_code = 'deadline.view' AND p.status = 'active'
WHERE r.status = 'active'
  AND r.role_code IN (
    'full_access',
    'admin_doanh_nghiep',
    'admin_web',
    'cms_operator',
    'truong_phong_ban',
    'truong_nhom',
    'user_thuong',
    'self_reg_company_owner'
  )
ON DUPLICATE KEY UPDATE status = VALUES(status);

-- Backfill tenant roles cloned before this migration (any role with other deadline.* perms).
INSERT INTO role_permissions (role_id, permission_id, status)
SELECT DISTINCT rp.role_id, p.permission_id, 'active'
FROM role_permissions rp
INNER JOIN permissions existing ON existing.permission_id = rp.permission_id
  AND existing.permission_code IN ('deadline.create', 'deadline.manage', 'deadline.assign')
  AND existing.status = 'active'
INNER JOIN permissions p ON p.permission_code = 'deadline.view' AND p.status = 'active'
WHERE rp.status = 'active'
ON DUPLICATE KEY UPDATE status = VALUES(status);
