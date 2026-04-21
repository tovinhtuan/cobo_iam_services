SET NAMES utf8mb4;

-- Password for all users below: secret
SET @PWD_HASH = '$2a$10$34UTU89qY8PQrxq78GZaHuwZSvPIfI/JteqD86am.jnNe.1qcReES';

INSERT INTO users (user_id, login_id, full_name, account_status) VALUES
  ('u_admin_web', 'admin.web@example.com', 'Admin Web', 'active'),
  ('u_admin_dn', 'admin.dn@example.com', 'Admin Doanh Nghiep', 'active'),
  ('u_truong_phong', 'truong.phong@example.com', 'Truong Phong Ban', 'active'),
  ('u_truong_nhom', 'truong.nhom@example.com', 'Truong Nhom', 'active'),
  ('u_nhan_vien', 'nhanvien@example.com', 'Nhan Vien Thuong', 'active')
ON DUPLICATE KEY UPDATE full_name = VALUES(full_name), account_status = VALUES(account_status);

INSERT INTO credentials (credential_id, user_id, credential_type, password_hash, password_algo, status) VALUES
  ('cred0001-0001-4000-8000-000000000003', 'u_admin_web', 'password', @PWD_HASH, 'bcrypt', 'active'),
  ('cred0001-0001-4000-8000-000000000004', 'u_admin_dn', 'password', @PWD_HASH, 'bcrypt', 'active'),
  ('cred0001-0001-4000-8000-000000000005', 'u_truong_phong', 'password', @PWD_HASH, 'bcrypt', 'active'),
  ('cred0001-0001-4000-8000-000000000006', 'u_truong_nhom', 'password', @PWD_HASH, 'bcrypt', 'active'),
  ('cred0001-0001-4000-8000-000000000007', 'u_nhan_vien', 'password', @PWD_HASH, 'bcrypt', 'active')
ON DUPLICATE KEY UPDATE password_hash = VALUES(password_hash), status = VALUES(status);

INSERT INTO roles (role_id, company_id, role_code, role_name, status) VALUES
  ('r0000001-0001-4000-8000-000000000011', 'c_001', 'admin_web', 'Admin Web', 'active'),
  ('r0000001-0001-4000-8000-000000000012', 'c_001', 'admin_doanh_nghiep', 'Admin Doanh Nghiep', 'active'),
  ('r0000001-0001-4000-8000-000000000013', 'c_001', 'truong_phong_ban', 'Truong Phong Ban', 'active'),
  ('r0000001-0001-4000-8000-000000000014', 'c_001', 'truong_nhom', 'Truong Nhom', 'active'),
  ('r0000001-0001-4000-8000-000000000015', 'c_001', 'user_thuong', 'User Thuong', 'active')
ON DUPLICATE KEY UPDATE role_name = VALUES(role_name), status = VALUES(status);

INSERT INTO memberships (membership_id, user_id, company_id, membership_status) VALUES
  ('m_101', 'u_admin_web', 'c_001', 'active'),
  ('m_102', 'u_admin_dn', 'c_001', 'active'),
  ('m_103', 'u_truong_phong', 'c_001', 'active'),
  ('m_104', 'u_truong_nhom', 'c_001', 'active'),
  ('m_105', 'u_nhan_vien', 'c_001', 'active')
ON DUPLICATE KEY UPDATE membership_status = VALUES(membership_status);

INSERT INTO membership_roles (membership_id, role_id, status) VALUES
  ('m_101', 'r0000001-0001-4000-8000-000000000011', 'active'),
  ('m_102', 'r0000001-0001-4000-8000-000000000012', 'active'),
  ('m_103', 'r0000001-0001-4000-8000-000000000013', 'active'),
  ('m_104', 'r0000001-0001-4000-8000-000000000014', 'active'),
  ('m_105', 'r0000001-0001-4000-8000-000000000015', 'active')
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT INTO role_permissions (role_id, permission_id, status) VALUES
  ('r0000001-0001-4000-8000-000000000011', '10000000-0001-4000-8000-000000000001', 'active'),
  ('r0000001-0001-4000-8000-000000000011', '10000000-0001-4000-8000-000000000002', 'active'),
  ('r0000001-0001-4000-8000-000000000011', '10000000-0001-4000-8000-000000000003', 'active'),
  ('r0000001-0001-4000-8000-000000000011', '10000000-0001-4000-8000-000000000004', 'active'),
  ('r0000001-0001-4000-8000-000000000011', '10000000-0001-4000-8000-000000000005', 'active'),
  ('r0000001-0001-4000-8000-000000000011', '10000000-0001-4000-8000-000000000006', 'active'),
  ('r0000001-0001-4000-8000-000000000011', '10000000-0001-4000-8000-000000000007', 'active'),
  ('r0000001-0001-4000-8000-000000000011', '10000000-0001-4000-8000-000000000008', 'active'),
  ('r0000001-0001-4000-8000-000000000011', '10000000-0001-4000-8000-000000000009', 'active'),
  ('r0000001-0001-4000-8000-000000000011', '10000000-0001-4000-8000-00000000000a', 'active'),
  ('r0000001-0001-4000-8000-000000000011', '10000000-0001-4000-8000-00000000000b', 'active'),
  ('r0000001-0001-4000-8000-000000000011', '10000000-0001-4000-8000-00000000000c', 'active'),
  ('r0000001-0001-4000-8000-000000000012', '10000000-0001-4000-8000-000000000001', 'active'),
  ('r0000001-0001-4000-8000-000000000012', '10000000-0001-4000-8000-000000000002', 'active'),
  ('r0000001-0001-4000-8000-000000000012', '10000000-0001-4000-8000-000000000003', 'active'),
  ('r0000001-0001-4000-8000-000000000012', '10000000-0001-4000-8000-000000000004', 'active'),
  ('r0000001-0001-4000-8000-000000000012', '10000000-0001-4000-8000-000000000005', 'active'),
  ('r0000001-0001-4000-8000-000000000012', '10000000-0001-4000-8000-000000000006', 'active'),
  ('r0000001-0001-4000-8000-000000000012', '10000000-0001-4000-8000-000000000007', 'active'),
  ('r0000001-0001-4000-8000-000000000012', '10000000-0001-4000-8000-000000000008', 'active'),
  ('r0000001-0001-4000-8000-000000000012', '10000000-0001-4000-8000-000000000009', 'active'),
  ('r0000001-0001-4000-8000-000000000012', '10000000-0001-4000-8000-00000000000a', 'active'),
  ('r0000001-0001-4000-8000-000000000012', '10000000-0001-4000-8000-00000000000b', 'active'),
  ('r0000001-0001-4000-8000-000000000012', '10000000-0001-4000-8000-00000000000c', 'active'),
  ('r0000001-0001-4000-8000-000000000013', '10000000-0001-4000-8000-000000000001', 'active'),
  ('r0000001-0001-4000-8000-000000000013', '10000000-0001-4000-8000-000000000002', 'active'),
  ('r0000001-0001-4000-8000-000000000013', '10000000-0001-4000-8000-000000000003', 'active'),
  ('r0000001-0001-4000-8000-000000000013', '10000000-0001-4000-8000-000000000004', 'active'),
  ('r0000001-0001-4000-8000-000000000013', '10000000-0001-4000-8000-000000000005', 'active'),
  ('r0000001-0001-4000-8000-000000000013', '10000000-0001-4000-8000-000000000006', 'active'),
  ('r0000001-0001-4000-8000-000000000013', '10000000-0001-4000-8000-000000000008', 'active'),
  ('r0000001-0001-4000-8000-000000000013', '10000000-0001-4000-8000-000000000009', 'active'),
  ('r0000001-0001-4000-8000-000000000014', '10000000-0001-4000-8000-000000000001', 'active'),
  ('r0000001-0001-4000-8000-000000000014', '10000000-0001-4000-8000-000000000003', 'active'),
  ('r0000001-0001-4000-8000-000000000014', '10000000-0001-4000-8000-000000000004', 'active'),
  ('r0000001-0001-4000-8000-000000000014', '10000000-0001-4000-8000-000000000005', 'active'),
  ('r0000001-0001-4000-8000-000000000014', '10000000-0001-4000-8000-000000000006', 'active'),
  ('r0000001-0001-4000-8000-000000000014', '10000000-0001-4000-8000-000000000008', 'active'),
  ('r0000001-0001-4000-8000-000000000015', '10000000-0001-4000-8000-000000000001', 'active'),
  ('r0000001-0001-4000-8000-000000000015', '10000000-0001-4000-8000-000000000003', 'active'),
  ('r0000001-0001-4000-8000-000000000015', '10000000-0001-4000-8000-000000000004', 'active'),
  ('r0000001-0001-4000-8000-000000000015', '10000000-0001-4000-8000-000000000005', 'active'),
  ('r0000001-0001-4000-8000-000000000015', '10000000-0001-4000-8000-000000000006', 'active')
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT INTO department_memberships (membership_id, department_id, status) VALUES
  ('m_101', 'd_legal', 'active'),
  ('m_102', 'd_legal', 'active'),
  ('m_103', 'd_legal', 'active'),
  ('m_104', 'd_legal', 'active'),
  ('m_105', 'd_legal', 'active')
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT INTO membership_effective_responsibilities (company_id, membership_id, responsibility_code, source_type, source_ref_id) VALUES
  ('c_001', 'm_101', 'workflow_approver:disclosure', 'seed', NULL),
  ('c_001', 'm_102', 'workflow_approver:disclosure', 'seed', NULL),
  ('c_001', 'm_103', 'workflow_approver:disclosure', 'seed', NULL),
  ('c_001', 'm_104', 'workflow_approver:disclosure', 'seed', NULL)
ON DUPLICATE KEY UPDATE source_type = VALUES(source_type);

INSERT INTO org_units (org_unit_id, company_id, parent_org_unit_id, unit_code, unit_name, unit_type, status) VALUES
  ('ou_dept_legal', 'c_001', NULL, 'dept_legal', 'Phong Phap Che', 'department', 'active'),
  ('ou_team_legal_1', 'c_001', 'ou_dept_legal', 'team_legal_1', 'Nhom Phap Che 1', 'team', 'active')
ON DUPLICATE KEY UPDATE
  parent_org_unit_id = VALUES(parent_org_unit_id),
  unit_name = VALUES(unit_name),
  unit_type = VALUES(unit_type),
  status = VALUES(status);

INSERT INTO org_unit_closure (ancestor_org_unit_id, descendant_org_unit_id, depth) VALUES
  ('ou_dept_legal', 'ou_dept_legal', 0),
  ('ou_team_legal_1', 'ou_team_legal_1', 0),
  ('ou_dept_legal', 'ou_team_legal_1', 1)
ON DUPLICATE KEY UPDATE depth = VALUES(depth);

INSERT INTO org_unit_memberships (company_id, membership_id, org_unit_id, position_code, status) VALUES
  ('c_001', 'm_101', 'ou_dept_legal', 'admin_web', 'active'),
  ('c_001', 'm_102', 'ou_dept_legal', 'admin_dn', 'active'),
  ('c_001', 'm_103', 'ou_dept_legal', 'truong_phong', 'active'),
  ('c_001', 'm_104', 'ou_team_legal_1', 'truong_nhom', 'active'),
  ('c_001', 'm_105', 'ou_team_legal_1', 'nhan_vien', 'active')
ON DUPLICATE KEY UPDATE position_code = VALUES(position_code), status = VALUES(status);
