-- 0073: Self-registered company owners need company profile CRUD while company is still unverified.
-- Role self_reg_company_owner was cloned from full_access in 0026 before company.view/edit existed (0066).

SET NAMES utf8mb4;

INSERT INTO role_permissions (role_id, permission_id, status)
SELECT 'r0000000-0001-4000-8000-000099999001', p.permission_id, 'active'
FROM permissions p
WHERE p.permission_code IN ('company.view', 'company.edit')
  AND p.status = 'active'
ON DUPLICATE KEY UPDATE status = VALUES(status);
