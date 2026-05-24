DELETE FROM role_permissions
WHERE permission_id IN (
  SELECT permission_id FROM permissions WHERE permission_code IN ('company.view', 'company.edit')
);

DELETE FROM permissions WHERE permission_code IN ('company.view', 'company.edit');
