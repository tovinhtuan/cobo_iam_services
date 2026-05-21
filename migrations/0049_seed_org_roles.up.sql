SET NAMES utf8mb4;

-- New permissions for org-structure feature (Sprint 1, phase-2005).
-- IDs continue from highest used: ...000023 → ...000024, ...000025
INSERT INTO permissions (permission_id, permission_code, permission_name, module_name, status) VALUES
  ('10000000-0001-4000-8000-000000000024', 'dept.manage',                'Manage department teams and members', 'org',  'active'),
  ('10000000-0001-4000-8000-000000000025', 'company.ownership.transfer', 'Transfer primary admin ownership',    'org',  'active')
ON DUPLICATE KEY UPDATE permission_name = VALUES(permission_name), status = VALUES(status);

-- Global role: Đầu mối phòng/ban (dept_lead).
-- company_id = NULL → available to all companies (same pattern as self_reg_company_owner).
INSERT INTO roles (role_id, company_id, role_code, role_name, status) VALUES
  ('r0000000-0001-4000-8000-000099999002', NULL, 'dept_lead', 'Đầu mối phòng/ban', 'active')
ON DUPLICATE KEY UPDATE role_name = VALUES(role_name), status = VALUES(status);

-- Assign dept.manage to dept_lead.
INSERT INTO role_permissions (role_id, permission_id, status)
SELECT r.role_id, p.permission_id, 'active'
FROM roles r
JOIN permissions p ON p.permission_code = 'dept.manage'
WHERE r.role_code = 'dept_lead'
ON DUPLICATE KEY UPDATE status = 'active';
