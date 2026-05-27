-- 0079: Manage periodic/custom auto-create preferences (PO §2.1, contract §6.3).
-- PATCH /api/v1/company/disclosure-types/{type_id}/preferences requires disclosure.auto_create.manage.
-- GET uses disclosure.view (already seeded).

SET NAMES utf8mb4;

INSERT INTO permissions (permission_id, permission_code, permission_name, module_name, status) VALUES
  ('10000000-0001-4000-8000-000000000027', 'disclosure.auto_create.manage', 'Manage disclosure auto-create preferences', 'disclosure', 'active')
ON DUPLICATE KEY UPDATE
  permission_code = VALUES(permission_code),
  permission_name = VALUES(permission_name),
  module_name = VALUES(module_name),
  status = VALUES(status);

INSERT INTO role_permissions (role_id, permission_id, status)
SELECT r.role_id, p.permission_id, 'active'
FROM roles r
INNER JOIN permissions p ON p.permission_code = 'disclosure.auto_create.manage' AND p.status = 'active'
WHERE r.status = 'active'
  AND r.role_code IN (
    'full_access',
    'admin_doanh_nghiep',
    'admin_web',
    'self_reg_company_owner',
    'company_admin'
  )
ON DUPLICATE KEY UPDATE status = VALUES(status);
