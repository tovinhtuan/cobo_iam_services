-- Demo package 2 seed: disclosure + workflow + notification history.
-- Apply after: seed_demo_package_1.sql and migration 0004.

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

INSERT INTO users (user_id, login_id, full_name, email, phone, account_status) VALUES
  ('usr_demo_reviewer_001', 'demo.reviewer@example.com', 'Demo Reviewer', 'demo.reviewer@example.com', '0900000002', 'active')
ON DUPLICATE KEY UPDATE
  full_name = VALUES(full_name),
  email = VALUES(email),
  phone = VALUES(phone),
  account_status = VALUES(account_status);

INSERT INTO credentials (credential_id, user_id, credential_type, password_hash, password_algo, status) VALUES
  ('cred_demo_reviewer_001', 'usr_demo_reviewer_001', 'password', '$2a$10$34UTU89qY8PQrxq78GZaHuwZSvPIfI/JteqD86am.jnNe.1qcReES', 'bcrypt', 'active')
ON DUPLICATE KEY UPDATE
  password_hash = VALUES(password_hash),
  password_algo = VALUES(password_algo),
  status = VALUES(status);

INSERT INTO memberships (membership_id, user_id, company_id, membership_status, effective_from) VALUES
  ('mbr_demo_reviewer_001', 'usr_demo_reviewer_001', 'cmp_demo_001', 'active', NOW())
ON DUPLICATE KEY UPDATE
  membership_status = VALUES(membership_status),
  effective_from = VALUES(effective_from);

INSERT INTO roles (role_id, company_id, role_code, role_name, status) VALUES
  ('role_demo_reviewer_001', 'cmp_demo_001', 'disclosure_reviewer', 'Disclosure Reviewer', 'active')
ON DUPLICATE KEY UPDATE
  role_name = VALUES(role_name),
  status = VALUES(status);

INSERT INTO role_permissions (role_id, permission_id, status) VALUES
  ('role_demo_reviewer_001', 'perm_demo_004', 'active'),
  ('role_demo_reviewer_001', 'perm_demo_007', 'active'),
  ('role_demo_reviewer_001', 'perm_demo_008', 'active'),
  ('role_demo_reviewer_001', 'perm_demo_018', 'active')
ON DUPLICATE KEY UPDATE
  status = VALUES(status);

INSERT INTO membership_roles (membership_id, role_id, status, effective_from) VALUES
  ('mbr_demo_reviewer_001', 'role_demo_reviewer_001', 'active', NOW())
ON DUPLICATE KEY UPDATE
  status = VALUES(status),
  effective_from = VALUES(effective_from);

INSERT INTO departments (department_id, company_id, department_code, department_name, status) VALUES
  ('dep_demo_compliance_001', 'cmp_demo_001', 'compliance', 'Compliance', 'active')
ON DUPLICATE KEY UPDATE
  department_name = VALUES(department_name),
  status = VALUES(status);

INSERT INTO department_memberships (membership_id, department_id, status, effective_from) VALUES
  ('mbr_demo_reviewer_001', 'dep_demo_compliance_001', 'active', NOW())
ON DUPLICATE KEY UPDATE
  status = VALUES(status),
  effective_from = VALUES(effective_from);

INSERT INTO titles (title_id, company_id, title_code, title_name, status) VALUES
  ('ttl_demo_reviewer_001', 'cmp_demo_001', 'compliance-reviewer', 'Compliance Reviewer', 'active')
ON DUPLICATE KEY UPDATE
  title_name = VALUES(title_name),
  status = VALUES(status);

INSERT INTO membership_titles (membership_id, title_id, status, effective_from) VALUES
  ('mbr_demo_reviewer_001', 'ttl_demo_reviewer_001', 'active', NOW())
ON DUPLICATE KEY UPDATE
  status = VALUES(status),
  effective_from = VALUES(effective_from);

INSERT INTO membership_effective_responsibilities (
  company_id, membership_id, responsibility_code, source_type, source_ref_id
) VALUES
  ('cmp_demo_001', 'mbr_demo_reviewer_001', 'workflow_reviewer:disclosure', 'seed', 'role_demo_reviewer_001'),
  ('cmp_demo_001', 'mbr_demo_reviewer_001', 'notification_recipient:review_queue', 'seed', 'role_demo_reviewer_001')
ON DUPLICATE KEY UPDATE
  source_type = VALUES(source_type),
  source_ref_id = VALUES(source_ref_id);

INSERT INTO disclosure_records (
  record_id, company_id, department_id, title, content, status, created_by, updated_by
) VALUES
  ('rec_demo_001', 'cmp_demo_001', 'dep_demo_legal_001', 'Gift Disclosure - Draft', 'Draft disclosure content for UI preview.', 'draft', 'usr_demo_admin_001', 'usr_demo_admin_001'),
  ('rec_demo_002', 'cmp_demo_001', 'dep_demo_ir_001', 'Outside Activity - Submitted', 'Submitted disclosure content for workflow history.', 'submitted', 'usr_demo_admin_001', 'usr_demo_admin_001'),
  ('rec_demo_003', 'cmp_demo_001', 'dep_demo_legal_001', 'Conflict of Interest - Approved', 'Approved disclosure content for detail page and dashboard.', 'approved', 'usr_demo_admin_001', 'usr_demo_reviewer_001')
ON DUPLICATE KEY UPDATE
  title = VALUES(title),
  content = VALUES(content),
  status = VALUES(status),
  updated_by = VALUES(updated_by);

INSERT INTO workflow_instances (
  workflow_instance_id, company_id, record_id, status, current_step_code, created_by
) VALUES
  ('wf_demo_001', 'cmp_demo_001', 'rec_demo_002', 'in_progress', 'review', 'usr_demo_admin_001'),
  ('wf_demo_002', 'cmp_demo_001', 'rec_demo_003', 'completed', 'done', 'usr_demo_admin_001')
ON DUPLICATE KEY UPDATE
  status = VALUES(status),
  current_step_code = VALUES(current_step_code),
  created_by = VALUES(created_by);

INSERT INTO workflow_tasks (
  task_id, company_id, workflow_instance_id, step_code, assignee_membership_id, status
) VALUES
  ('task_demo_001', 'cmp_demo_001', 'wf_demo_001', 'review', 'mbr_demo_reviewer_001', 'pending'),
  ('task_demo_002', 'cmp_demo_001', 'wf_demo_002', 'approve', 'mbr_demo_reviewer_001', 'completed')
ON DUPLICATE KEY UPDATE
  step_code = VALUES(step_code),
  assignee_membership_id = VALUES(assignee_membership_id),
  status = VALUES(status);

INSERT INTO assignments (
  assignment_id, company_id, assignee_type, assignee_ref_id, resource_type, resource_id, assignment_kind, status
) VALUES
  ('asg_demo_001', 'cmp_demo_001', 'membership', 'mbr_demo_admin_001', 'disclosure_record', 'rec_demo_001', 'owner', 'active'),
  ('asg_demo_002', 'cmp_demo_001', 'membership', 'mbr_demo_admin_001', 'disclosure_record', 'rec_demo_002', 'owner', 'active'),
  ('asg_demo_003', 'cmp_demo_001', 'membership', 'mbr_demo_reviewer_001', 'disclosure_record', 'rec_demo_002', 'reviewer', 'active'),
  ('asg_demo_004', 'cmp_demo_001', 'membership', 'mbr_demo_reviewer_001', 'disclosure_record', 'rec_demo_003', 'approver', 'active')
ON DUPLICATE KEY UPDATE
  assignment_kind = VALUES(assignment_kind),
  status = VALUES(status);

INSERT INTO notification_jobs (
  notification_job_id, company_id, event_type, resource_type, resource_id, payload_json, status
) VALUES
  ('noti_job_demo_001', 'cmp_demo_001', 'disclosure.submitted', 'disclosure_record', 'rec_demo_002', JSON_OBJECT('title', 'Outside Activity - Submitted', 'record_id', 'rec_demo_002'), 'processed'),
  ('noti_job_demo_002', 'cmp_demo_001', 'disclosure.approved', 'disclosure_record', 'rec_demo_003', JSON_OBJECT('title', 'Conflict of Interest - Approved', 'record_id', 'rec_demo_003'), 'processed')
ON DUPLICATE KEY UPDATE
  payload_json = VALUES(payload_json),
  status = VALUES(status);

INSERT INTO notification_deliveries (
  notification_delivery_id, notification_job_id, recipient, status
) VALUES
  ('noti_del_demo_001', 'noti_job_demo_001', 'demo.reviewer@example.com', 'sent'),
  ('noti_del_demo_002', 'noti_job_demo_001', 'demo.admin@example.com', 'sent'),
  ('noti_del_demo_003', 'noti_job_demo_002', 'demo.admin@example.com', 'sent')
ON DUPLICATE KEY UPDATE
  recipient = VALUES(recipient),
  status = VALUES(status);

SET FOREIGN_KEY_CHECKS = 1;
