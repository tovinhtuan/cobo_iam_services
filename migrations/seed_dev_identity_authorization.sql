-- Dev seed: aligns with former in-memory IAM + authorization fixtures.
-- Apply after: 0001, 0003 (projection responsibilities), 0004 (optional for disclosure FK), 0005 (unique refresh hash), 0006 (admin rule tables; optional if you use workflow/notification rule APIs).
-- Password for both users: secret (bcrypt cost 10, generated at seed authoring time).

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

INSERT INTO companies (company_id, company_code, company_name, status) VALUES
  ('c_001', 'c001', 'Company X', 'active'),
  ('c_002', 'c002', 'Company Y', 'active'),
  ('c_010', 'c010', 'Solo Company', 'active')
ON DUPLICATE KEY UPDATE company_name = VALUES(company_name);

INSERT INTO users (user_id, login_id, full_name, account_status) VALUES
  ('u_123', 'user@example.com', 'Nguyen Van A', 'active'),
  ('u_single', 'single@example.com', 'Single Company User', 'active')
ON DUPLICATE KEY UPDATE full_name = VALUES(full_name), account_status = VALUES(account_status);

INSERT INTO credentials (credential_id, user_id, credential_type, password_hash, password_algo, status) VALUES
  ('cred0001-0001-4000-8000-000000000001', 'u_123', 'password', '$2a$10$34UTU89qY8PQrxq78GZaHuwZSvPIfI/JteqD86am.jnNe.1qcReES', 'bcrypt', 'active'),
  ('cred0001-0001-4000-8000-000000000002', 'u_single', 'password', '$2a$10$34UTU89qY8PQrxq78GZaHuwZSvPIfI/JteqD86am.jnNe.1qcReES', 'bcrypt', 'active')
ON DUPLICATE KEY UPDATE password_hash = VALUES(password_hash);

INSERT INTO permissions (permission_id, permission_code, permission_name, module_name, status) VALUES
  ('10000000-0001-4000-8000-000000000001', 'view_disclosure', 'View disclosure', 'disclosure', 'active'),
  ('10000000-0001-4000-8000-000000000002', 'approve_disclosure', 'Approve disclosure', 'disclosure', 'active'),
  ('10000000-0001-4000-8000-000000000003', 'view_dashboard', 'View dashboard', 'dashboard', 'active'),
  ('10000000-0001-4000-8000-000000000004', 'create_disclosure', 'Create disclosure', 'disclosure', 'active'),
  ('10000000-0001-4000-8000-000000000005', 'update_disclosure', 'Update disclosure', 'disclosure', 'active'),
  ('10000000-0001-4000-8000-000000000006', 'submit_disclosure', 'Submit disclosure', 'disclosure', 'active'),
  ('10000000-0001-4000-8000-000000000007', 'create_workflow', 'Create workflow', 'workflow', 'active'),
  ('10000000-0001-4000-8000-000000000008', 'review_workflow_task', 'Review workflow task', 'workflow', 'active'),
  ('10000000-0001-4000-8000-000000000009', 'confirm_workflow_task', 'Confirm workflow task', 'workflow', 'active'),
  ('10000000-0001-4000-8000-00000000000a', 'enqueue_notification', 'Enqueue notification', 'notification', 'active'),
  ('10000000-0001-4000-8000-00000000000b', 'dispatch_notification', 'Dispatch notification', 'notification', 'active'),
  ('10000000-0001-4000-8000-00000000000c', 'admin_manage_access', 'Admin manage access', 'admin', 'active')
ON DUPLICATE KEY UPDATE permission_name = VALUES(permission_name);

INSERT INTO roles (role_id, company_id, role_code, role_name, status) VALUES
  ('r0000001-0001-4000-8000-000000000001', 'c_001', 'full_access', 'Full access (dev)', 'active'),
  ('r0000001-0001-4000-8000-000000000002', 'c_002', 'viewer', 'Viewer', 'active'),
  ('r0000001-0001-4000-8000-000000000003', 'c_010', 'dashboard_only', 'Dashboard only', 'active')
ON DUPLICATE KEY UPDATE role_name = VALUES(role_name);

INSERT INTO role_permissions (role_id, permission_id, status) VALUES
  ('r0000001-0001-4000-8000-000000000001', '10000000-0001-4000-8000-000000000001', 'active'),
  ('r0000001-0001-4000-8000-000000000001', '10000000-0001-4000-8000-000000000002', 'active'),
  ('r0000001-0001-4000-8000-000000000001', '10000000-0001-4000-8000-000000000003', 'active'),
  ('r0000001-0001-4000-8000-000000000001', '10000000-0001-4000-8000-000000000004', 'active'),
  ('r0000001-0001-4000-8000-000000000001', '10000000-0001-4000-8000-000000000005', 'active'),
  ('r0000001-0001-4000-8000-000000000001', '10000000-0001-4000-8000-000000000006', 'active'),
  ('r0000001-0001-4000-8000-000000000001', '10000000-0001-4000-8000-000000000007', 'active'),
  ('r0000001-0001-4000-8000-000000000001', '10000000-0001-4000-8000-000000000008', 'active'),
  ('r0000001-0001-4000-8000-000000000001', '10000000-0001-4000-8000-000000000009', 'active'),
  ('r0000001-0001-4000-8000-000000000001', '10000000-0001-4000-8000-00000000000a', 'active'),
  ('r0000001-0001-4000-8000-000000000001', '10000000-0001-4000-8000-00000000000b', 'active'),
  ('r0000001-0001-4000-8000-000000000001', '10000000-0001-4000-8000-00000000000c', 'active'),
  ('r0000001-0001-4000-8000-000000000002', '10000000-0001-4000-8000-000000000001', 'active'),
  ('r0000001-0001-4000-8000-000000000003', '10000000-0001-4000-8000-000000000003', 'active')
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT INTO memberships (membership_id, user_id, company_id, membership_status) VALUES
  ('m_001', 'u_123', 'c_001', 'active'),
  ('m_002', 'u_123', 'c_002', 'active'),
  ('m_010', 'u_single', 'c_010', 'active')
ON DUPLICATE KEY UPDATE membership_status = VALUES(membership_status);

INSERT INTO membership_roles (membership_id, role_id, status) VALUES
  ('m_001', 'r0000001-0001-4000-8000-000000000001', 'active'),
  ('m_002', 'r0000001-0001-4000-8000-000000000002', 'active'),
  ('m_010', 'r0000001-0001-4000-8000-000000000003', 'active')
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT INTO departments (department_id, company_id, department_code, department_name, status) VALUES
  ('d_legal', 'c_001', 'legal', 'Legal', 'active'),
  ('d_ir', 'c_001', 'ir', 'IR', 'active')
ON DUPLICATE KEY UPDATE department_name = VALUES(department_name);

INSERT INTO department_memberships (membership_id, department_id, status) VALUES
  ('m_001', 'd_legal', 'active'),
  ('m_001', 'd_ir', 'active')
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT INTO assignments (assignment_id, company_id, assignee_type, assignee_ref_id, resource_type, resource_id, assignment_kind, status) VALUES
  ('asgn0001-0001-4000-8000-000000000001', 'c_001', 'membership', 'm_001', 'disclosure_record', 'r_1001', 'direct', 'active')
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT INTO titles (title_id, company_id, title_code, title_name, status) VALUES
  ('t0000001-0001-4000-8000-000000000001', 'c_001', 'cbtt', 'Dau moi CBTT', 'active')
ON DUPLICATE KEY UPDATE title_name = VALUES(title_name);

INSERT INTO membership_titles (membership_id, title_id, status) VALUES
  ('m_001', 't0000001-0001-4000-8000-000000000001', 'active')
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT INTO membership_effective_responsibilities (company_id, membership_id, responsibility_code, source_type, source_ref_id) VALUES
  ('c_001', 'm_001', 'workflow_approver:disclosure', 'seed', NULL),
  ('c_001', 'm_001', 'notification_recipient:disclosure', 'seed', NULL)
ON DUPLICATE KEY UPDATE source_type = VALUES(source_type);

SET FOREIGN_KEY_CHECKS = 1;
