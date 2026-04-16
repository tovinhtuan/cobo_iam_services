-- Demo package 1 seed: auth + company context + permission shell for cobo_web_design.
-- Apply after: 0001, 0003. Safe to apply after 0004/0005/0006 as well.
-- Login:
--   demo.admin@example.com / secret
-- Password hash below is bcrypt("secret"), reused from existing dev seed.

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

INSERT INTO companies (company_id, company_code, company_name, status) VALUES
  ('cmp_demo_001', 'demo-holdings', 'Demo Holdings', 'active'),
  ('cmp_demo_002', 'demo-energy', 'Demo Energy', 'active')
ON DUPLICATE KEY UPDATE
  company_name = VALUES(company_name),
  status = VALUES(status);

INSERT INTO users (user_id, login_id, full_name, email, phone, account_status) VALUES
  ('usr_demo_admin_001', 'demo.admin@example.com', 'Demo Admin', 'demo.admin@example.com', '0900000001', 'active')
ON DUPLICATE KEY UPDATE
  full_name = VALUES(full_name),
  email = VALUES(email),
  phone = VALUES(phone),
  account_status = VALUES(account_status);

INSERT INTO credentials (credential_id, user_id, credential_type, password_hash, password_algo, status) VALUES
  ('cred_demo_admin_001', 'usr_demo_admin_001', 'password', '$2a$10$34UTU89qY8PQrxq78GZaHuwZSvPIfI/JteqD86am.jnNe.1qcReES', 'bcrypt', 'active')
ON DUPLICATE KEY UPDATE
  password_hash = VALUES(password_hash),
  password_algo = VALUES(password_algo),
  status = VALUES(status);

INSERT INTO memberships (membership_id, user_id, company_id, membership_status, effective_from) VALUES
  ('mbr_demo_admin_001', 'usr_demo_admin_001', 'cmp_demo_001', 'active', NOW()),
  ('mbr_demo_admin_002', 'usr_demo_admin_001', 'cmp_demo_002', 'active', NOW())
ON DUPLICATE KEY UPDATE
  membership_status = VALUES(membership_status),
  effective_from = VALUES(effective_from);

INSERT INTO roles (role_id, company_id, role_code, role_name, status) VALUES
  ('role_demo_admin_001', 'cmp_demo_001', 'company_admin', 'Company Admin', 'active'),
  ('role_demo_viewer_002', 'cmp_demo_002', 'company_viewer', 'Company Viewer', 'active')
ON DUPLICATE KEY UPDATE
  role_name = VALUES(role_name),
  status = VALUES(status);

INSERT INTO permissions (permission_id, permission_code, permission_name, module_name, status) VALUES
  ('perm_demo_001', 'view_dashboard', 'View dashboard', 'dashboard', 'active'),
  ('perm_demo_002', 'company.view', 'View company', 'company', 'active'),
  ('perm_demo_003', 'deadline.view', 'View deadlines', 'deadline', 'active'),
  ('perm_demo_004', 'disclosure.view', 'View disclosures', 'disclosure', 'active'),
  ('perm_demo_005', 'disclosure.create', 'Create disclosure', 'disclosure', 'active'),
  ('perm_demo_006', 'disclosure.edit', 'Edit disclosure', 'disclosure', 'active'),
  ('perm_demo_007', 'disclosure.approve', 'Approve disclosure', 'workflow', 'active'),
  ('perm_demo_008', 'workflow.step.confirm', 'Confirm workflow step', 'workflow', 'active'),
  ('perm_demo_009', 'workflow.step.override', 'Override workflow step', 'workflow', 'active'),
  ('perm_demo_010', 'manage_users', 'Manage users', 'admin', 'active'),
  ('perm_demo_011', 'user.edit', 'Edit users', 'admin', 'active'),
  ('perm_demo_012', 'rbac.manage', 'Manage RBAC', 'admin', 'active'),
  ('perm_demo_013', 'system.settings', 'System settings', 'admin', 'active'),
  ('perm_demo_014', 'manage_departments', 'Manage departments', 'admin', 'active'),
  ('perm_demo_015', 'recipient.manage', 'Manage recipients', 'admin', 'active'),
  ('perm_demo_016', 'manage_notification_rules', 'Manage notification rules', 'notification', 'active'),
  ('perm_demo_017', 'alert.channels.manage', 'Manage alert channels', 'notification', 'active'),
  ('perm_demo_018', 'approve_disclosure', 'Approve disclosure legacy', 'workflow', 'active')
ON DUPLICATE KEY UPDATE
  permission_name = VALUES(permission_name),
  module_name = VALUES(module_name),
  status = VALUES(status);

INSERT INTO role_permissions (role_id, permission_id, status) VALUES
  ('role_demo_admin_001', 'perm_demo_001', 'active'),
  ('role_demo_admin_001', 'perm_demo_002', 'active'),
  ('role_demo_admin_001', 'perm_demo_003', 'active'),
  ('role_demo_admin_001', 'perm_demo_004', 'active'),
  ('role_demo_admin_001', 'perm_demo_005', 'active'),
  ('role_demo_admin_001', 'perm_demo_006', 'active'),
  ('role_demo_admin_001', 'perm_demo_007', 'active'),
  ('role_demo_admin_001', 'perm_demo_008', 'active'),
  ('role_demo_admin_001', 'perm_demo_009', 'active'),
  ('role_demo_admin_001', 'perm_demo_010', 'active'),
  ('role_demo_admin_001', 'perm_demo_011', 'active'),
  ('role_demo_admin_001', 'perm_demo_012', 'active'),
  ('role_demo_admin_001', 'perm_demo_013', 'active'),
  ('role_demo_admin_001', 'perm_demo_014', 'active'),
  ('role_demo_admin_001', 'perm_demo_015', 'active'),
  ('role_demo_admin_001', 'perm_demo_016', 'active'),
  ('role_demo_admin_001', 'perm_demo_017', 'active'),
  ('role_demo_admin_001', 'perm_demo_018', 'active'),
  ('role_demo_viewer_002', 'perm_demo_001', 'active'),
  ('role_demo_viewer_002', 'perm_demo_002', 'active'),
  ('role_demo_viewer_002', 'perm_demo_004', 'active')
ON DUPLICATE KEY UPDATE
  status = VALUES(status);

INSERT INTO membership_roles (membership_id, role_id, status, effective_from) VALUES
  ('mbr_demo_admin_001', 'role_demo_admin_001', 'active', NOW()),
  ('mbr_demo_admin_002', 'role_demo_viewer_002', 'active', NOW())
ON DUPLICATE KEY UPDATE
  status = VALUES(status),
  effective_from = VALUES(effective_from);

INSERT INTO departments (department_id, company_id, department_code, department_name, status) VALUES
  ('dep_demo_legal_001', 'cmp_demo_001', 'legal', 'Legal', 'active'),
  ('dep_demo_ir_001', 'cmp_demo_001', 'ir', 'Investor Relations', 'active'),
  ('dep_demo_ops_002', 'cmp_demo_002', 'ops', 'Operations', 'active')
ON DUPLICATE KEY UPDATE
  department_name = VALUES(department_name),
  status = VALUES(status);

INSERT INTO department_memberships (membership_id, department_id, status, effective_from) VALUES
  ('mbr_demo_admin_001', 'dep_demo_legal_001', 'active', NOW()),
  ('mbr_demo_admin_001', 'dep_demo_ir_001', 'active', NOW()),
  ('mbr_demo_admin_002', 'dep_demo_ops_002', 'active', NOW())
ON DUPLICATE KEY UPDATE
  status = VALUES(status),
  effective_from = VALUES(effective_from);

INSERT INTO titles (title_id, company_id, title_code, title_name, status) VALUES
  ('ttl_demo_head_legal_001', 'cmp_demo_001', 'head-legal', 'Head of Legal', 'active'),
  ('ttl_demo_manager_ops_002', 'cmp_demo_002', 'ops-manager', 'Operations Manager', 'active')
ON DUPLICATE KEY UPDATE
  title_name = VALUES(title_name),
  status = VALUES(status);

INSERT INTO membership_titles (membership_id, title_id, status, effective_from) VALUES
  ('mbr_demo_admin_001', 'ttl_demo_head_legal_001', 'active', NOW()),
  ('mbr_demo_admin_002', 'ttl_demo_manager_ops_002', 'active', NOW())
ON DUPLICATE KEY UPDATE
  status = VALUES(status),
  effective_from = VALUES(effective_from);

INSERT INTO membership_effective_responsibilities (
  company_id, membership_id, responsibility_code, source_type, source_ref_id
) VALUES
  ('cmp_demo_001', 'mbr_demo_admin_001', 'workflow_approver:disclosure', 'seed', 'role_demo_admin_001'),
  ('cmp_demo_001', 'mbr_demo_admin_001', 'notification_recipient:disclosure', 'seed', 'role_demo_admin_001'),
  ('cmp_demo_002', 'mbr_demo_admin_002', 'viewer:disclosure', 'seed', 'role_demo_viewer_002')
ON DUPLICATE KEY UPDATE
  source_type = VALUES(source_type),
  source_ref_id = VALUES(source_ref_id);

SET FOREIGN_KEY_CHECKS = 1;
