-- 0077: Tenant invite/list permission (PO-01..03). CMS stays on platform.cms.view + rbac.manage (§12).

SET NAMES utf8mb4;

INSERT INTO permissions (permission_id, permission_code, permission_name, module_name, status) VALUES
  ('10000000-0001-4000-8000-000000000026', 'admin.membership.invite', 'Invite and list company members', 'admin', 'active')
ON DUPLICATE KEY UPDATE
  permission_code = VALUES(permission_code),
  permission_name = VALUES(permission_name),
  module_name = VALUES(module_name),
  status = VALUES(status);

INSERT INTO role_permissions (role_id, permission_id, status)
SELECT r.role_id, p.permission_id, 'active'
FROM roles r
INNER JOIN permissions p ON p.permission_code = 'admin.membership.invite' AND p.status = 'active'
WHERE r.status = 'active'
  AND r.role_code IN (
    'full_access',
    'admin_doanh_nghiep',
    'company_admin',
    'self_reg_company_owner'
  )
ON DUPLICATE KEY UPDATE status = VALUES(status);

-- Backfill: roles with rbac.manage except platform legacy roles (PO-03 B).
INSERT INTO role_permissions (role_id, permission_id, status)
SELECT DISTINCT rp.role_id, p_invite.permission_id, 'active'
FROM role_permissions rp
INNER JOIN permissions existing ON existing.permission_id = rp.permission_id
  AND existing.permission_code = 'rbac.manage'
  AND existing.status = 'active'
INNER JOIN roles r ON r.role_id = rp.role_id AND r.status = 'active'
  AND r.role_code NOT IN ('cms_operator', 'admin_web')
INNER JOIN permissions p_invite ON p_invite.permission_code = 'admin.membership.invite' AND p_invite.status = 'active'
WHERE rp.status = 'active'
ON DUPLICATE KEY UPDATE status = VALUES(status);
