-- 0063: Dev test account — platform CMS admin + tenant admin_doanh_nghiep on Company X (c_001) and Company Y (c_002).
-- Login: platform.tenant.admin@example.com / secret

SET NAMES utf8mb4;

INSERT INTO users (user_id, login_id, full_name, account_status) VALUES
  ('u_platform_tenant_admin', 'platform.tenant.admin@example.com', 'Platform + Tenant Admin (Dev)', 'active')
ON DUPLICATE KEY UPDATE
  login_id = VALUES(login_id),
  full_name = VALUES(full_name),
  account_status = VALUES(account_status);

INSERT INTO user_subscription_tiers (user_id, subscription_tier, source, effective_from, effective_to) VALUES
  ('u_platform_tenant_admin', 'Enterprise', 'seed', NULL, NULL)
ON DUPLICATE KEY UPDATE
  subscription_tier = VALUES(subscription_tier),
  source = VALUES(source);

INSERT INTO credentials (credential_id, user_id, credential_type, password_hash, password_algo, status) VALUES
  (
    'cred0001-0001-4000-8000-000000000010',
    'u_platform_tenant_admin',
    'password',
    '$2a$10$34UTU89qY8PQrxq78GZaHuwZSvPIfI/JteqD86am.jnNe.1qcReES',
    'bcrypt',
    'active'
  )
ON DUPLICATE KEY UPDATE password_hash = VALUES(password_hash), status = VALUES(status);

-- Tenant roles for c_002 (mirror c_001 cms_operator + admin_doanh_nghiep permission sets).
INSERT INTO roles (role_id, company_id, role_code, role_name, status) VALUES
  ('r0000001-0001-4000-8000-000000000017', 'c_002', 'cms_operator', 'CMS Operator', 'active'),
  ('r0000001-0001-4000-8000-000000000018', 'c_002', 'admin_doanh_nghiep', 'Admin Doanh Nghiep', 'active')
ON DUPLICATE KEY UPDATE role_name = VALUES(role_name), status = VALUES(status);

INSERT INTO role_permissions (role_id, permission_id, status)
SELECT 'r0000001-0001-4000-8000-000000000017', rp.permission_id, 'active'
FROM role_permissions rp
WHERE rp.role_id = 'r0000001-0001-4000-8000-000000000016'
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT INTO role_permissions (role_id, permission_id, status)
SELECT 'r0000001-0001-4000-8000-000000000018', rp.permission_id, 'active'
FROM role_permissions rp
WHERE rp.role_id = 'r0000001-0001-4000-8000-000000000012'
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT INTO role_permissions (role_id, permission_id, status)
SELECT r.role_id, p.permission_id, 'active'
FROM roles r
INNER JOIN permissions p ON p.permission_code IN (
  'ad_hoc_alert.read',
  'ad_hoc_alert.propose',
  'ad_hoc_alert.process_control',
  'ad_hoc_alert.focal_review',
  'ad_hoc_alert.admin_review',
  'workflow.read',
  'workflow.review',
  'workflow.approve',
  'workflow.confirm',
  'template.workflow.override.read',
  'template.workflow.override.write',
  'template.workflow.override.approve',
  'template.workflow.override.reset',
  'disclosure_type.manage',
  'disclosure_type.publish',
  'cms.template.read',
  'cms.template.write',
  'cms.template.activate',
  'cms.template.archive',
  'cms.template.config.write'
) AND p.status = 'active'
WHERE r.role_id IN (
  'r0000001-0001-4000-8000-000000000017',
  'r0000001-0001-4000-8000-000000000018'
)
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT INTO memberships (membership_id, user_id, company_id, membership_status) VALUES
  ('m_107', 'u_platform_tenant_admin', 'c_001', 'active'),
  ('m_108', 'u_platform_tenant_admin', 'c_002', 'active')
ON DUPLICATE KEY UPDATE membership_status = VALUES(membership_status);

INSERT INTO membership_roles (membership_id, role_id, status) VALUES
  ('m_107', 'r0000001-0001-4000-8000-000000000016', 'active'),
  ('m_107', 'r0000001-0001-4000-8000-000000000012', 'active'),
  ('m_108', 'r0000001-0001-4000-8000-000000000017', 'active'),
  ('m_108', 'r0000001-0001-4000-8000-000000000018', 'active')
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT IGNORE INTO membership_direct_permissions
  (membership_id, company_id, permission_code, granted_by)
VALUES
  ('m_107', 'c_001', 'ad_hoc_alert.read', 'system_migration_0063'),
  ('m_107', 'c_001', 'ad_hoc_alert.propose', 'system_migration_0063'),
  ('m_108', 'c_002', 'ad_hoc_alert.read', 'system_migration_0063'),
  ('m_108', 'c_002', 'ad_hoc_alert.propose', 'system_migration_0063');

INSERT INTO department_memberships (membership_id, department_id, status) VALUES
  ('m_107', 'd_legal', 'active')
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT INTO membership_effective_responsibilities (company_id, membership_id, responsibility_code, source_type, source_ref_id) VALUES
  ('c_001', 'm_107', 'workflow_approver:disclosure', 'seed', NULL),
  ('c_002', 'm_108', 'workflow_approver:disclosure', 'seed', NULL)
ON DUPLICATE KEY UPDATE source_type = VALUES(source_type);

INSERT INTO org_unit_memberships (company_id, membership_id, org_unit_id, position_code, status) VALUES
  ('c_001', 'm_107', 'ou_dept_legal', 'platform_tenant_admin', 'active')
ON DUPLICATE KEY UPDATE
  position_code = VALUES(position_code),
  status = VALUES(status);
