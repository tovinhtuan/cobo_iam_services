DELETE rp
FROM role_permissions rp
INNER JOIN roles r ON r.role_id = rp.role_id
INNER JOIN permissions p ON p.permission_id = rp.permission_id
WHERE r.company_id = 'c_001'
  AND r.role_code IN ('admin_web', 'cms_operator', 'admin_doanh_nghiep')
  AND p.permission_code IN (
    'template.workflow.override.read',
    'template.workflow.override.write',
    'template.workflow.override.approve',
    'template.workflow.override.reset'
  );

DELETE FROM permissions
WHERE permission_code IN (
  'template.workflow.override.read',
  'template.workflow.override.write',
  'template.workflow.override.approve',
  'template.workflow.override.reset'
);
