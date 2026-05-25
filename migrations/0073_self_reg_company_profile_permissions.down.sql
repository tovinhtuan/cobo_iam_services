SET NAMES utf8mb4;

DELETE rp FROM role_permissions rp
INNER JOIN permissions p ON p.permission_id = rp.permission_id
WHERE rp.role_id = 'r0000000-0001-4000-8000-000099999001'
  AND p.permission_code IN ('company.view', 'company.edit');
