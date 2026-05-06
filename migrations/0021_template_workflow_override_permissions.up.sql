INSERT INTO permissions (permission_id, permission_code, permission_name, module_name, status) VALUES
  ('10000000-0001-4000-8000-00000000000e', 'template.workflow.override.read', 'Read template workflow override', 'template', 'active'),
  ('10000000-0001-4000-8000-00000000000f', 'template.workflow.override.write', 'Write template workflow override draft', 'template', 'active'),
  ('10000000-0001-4000-8000-000000000010', 'template.workflow.override.approve', 'Approve template workflow override', 'template', 'active'),
  ('10000000-0001-4000-8000-000000000011', 'template.workflow.override.reset', 'Reset template workflow override', 'template', 'active')
ON DUPLICATE KEY UPDATE
  permission_code = VALUES(permission_code),
  permission_name = VALUES(permission_name),
  module_name = VALUES(module_name),
  status = VALUES(status);

INSERT INTO role_permissions (role_id, permission_id, status)
SELECT r.role_id, p.permission_id, 'active'
FROM roles r
INNER JOIN permissions p
  ON p.permission_code IN (
    'template.workflow.override.read',
    'template.workflow.override.write',
    'template.workflow.override.approve',
    'template.workflow.override.reset'
  )
WHERE r.company_id = 'c_001'
  AND r.role_code IN ('admin_web', 'cms_operator', 'admin_doanh_nghiep')
ON DUPLICATE KEY UPDATE status = VALUES(status);
