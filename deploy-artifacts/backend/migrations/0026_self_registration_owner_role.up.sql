SET NAMES utf8mb4;

-- Global role for users who self-register and create a company (company_id NULL = assignable to any company membership).
INSERT INTO roles (role_id, company_id, role_code, role_name, status) VALUES
  ('r0000000-0001-4000-8000-000099999001', NULL, 'self_reg_company_owner', 'Self-registered company owner', 'active')
ON DUPLICATE KEY UPDATE role_name = VALUES(role_name), status = VALUES(status);

-- Same permission surface as dev "full_access" on c_001 (enterprise-style portal owner).
INSERT INTO role_permissions (role_id, permission_id, status)
SELECT 'r0000000-0001-4000-8000-000099999001', rp.permission_id, 'active'
FROM role_permissions rp
WHERE rp.role_id = 'r0000001-0001-4000-8000-000000000001' AND rp.status = 'active'
ON DUPLICATE KEY UPDATE status = VALUES(status);
