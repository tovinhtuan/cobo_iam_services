SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- Org structure demo seed.
-- Creates 1 company with 2 departments × 2 teams each,
-- plus 5 user archetypes: company admin, dept head, team head,
-- regular user in a team, regular user in a dept only (no team).
--
-- Login / password for all accounts: secret
-- bcrypt hash reused from dev seed: $2a$10$34UTU89qY8PQrxq78GZaHuwZSvPIfI/JteqD86am.jnNe.1qcReES

-- ─── Company ──────────────────────────────────────────────────────────────────

INSERT INTO companies (company_id, company_code, company_name, status) VALUES
  ('cmp_org_demo_001', 'org-demo-vn', 'Công ty CP Demo Việt Nam', 'active')
ON DUPLICATE KEY UPDATE
  company_name = VALUES(company_name),
  status       = VALUES(status);

-- ─── Users ────────────────────────────────────────────────────────────────────

INSERT INTO users (user_id, login_id, full_name, email, phone, account_status) VALUES
  ('usr_org_admin_001',     'admin.org@demo.vn',     'Nguyễn Admin',    'admin.org@demo.vn',     '0900100001', 'active'),
  ('usr_org_tp_001',        'truong.phong@demo.vn',  'Trần Văn Phong',  'truong.phong@demo.vn',  '0900100002', 'active'),
  ('usr_org_tn_001',        'truong.nhom@demo.vn',   'Lê Thị Nhóm',     'truong.nhom@demo.vn',   '0900100003', 'active'),
  ('usr_org_nv_nhom_001',   'nv.nhom@demo.vn',       'Phạm Văn Thành',  'nv.nhom@demo.vn',       '0900100004', 'active'),
  ('usr_org_nv_phong_001',  'nv.phong@demo.vn',      'Hoàng Thị Mai',   'nv.phong@demo.vn',      '0900100005', 'active')
ON DUPLICATE KEY UPDATE
  full_name      = VALUES(full_name),
  email          = VALUES(email),
  phone          = VALUES(phone),
  account_status = VALUES(account_status);

-- ─── Credentials (bcrypt "secret") ────────────────────────────────────────────

INSERT INTO credentials (credential_id, user_id, credential_type, password_hash, password_algo, status) VALUES
  ('cred_org_admin_001',    'usr_org_admin_001',    'password', '$2a$10$34UTU89qY8PQrxq78GZaHuwZSvPIfI/JteqD86am.jnNe.1qcReES', 'bcrypt', 'active'),
  ('cred_org_tp_001',       'usr_org_tp_001',       'password', '$2a$10$34UTU89qY8PQrxq78GZaHuwZSvPIfI/JteqD86am.jnNe.1qcReES', 'bcrypt', 'active'),
  ('cred_org_tn_001',       'usr_org_tn_001',       'password', '$2a$10$34UTU89qY8PQrxq78GZaHuwZSvPIfI/JteqD86am.jnNe.1qcReES', 'bcrypt', 'active'),
  ('cred_org_nv_nhom_001',  'usr_org_nv_nhom_001',  'password', '$2a$10$34UTU89qY8PQrxq78GZaHuwZSvPIfI/JteqD86am.jnNe.1qcReES', 'bcrypt', 'active'),
  ('cred_org_nv_phong_001', 'usr_org_nv_phong_001', 'password', '$2a$10$34UTU89qY8PQrxq78GZaHuwZSvPIfI/JteqD86am.jnNe.1qcReES', 'bcrypt', 'active')
ON DUPLICATE KEY UPDATE
  password_hash = VALUES(password_hash),
  password_algo = VALUES(password_algo),
  status        = VALUES(status);

-- ─── Memberships ──────────────────────────────────────────────────────────────

INSERT INTO memberships (membership_id, user_id, company_id, membership_status, effective_from) VALUES
  ('mbr_org_admin_001',    'usr_org_admin_001',    'cmp_org_demo_001', 'active', NOW()),
  ('mbr_org_tp_001',       'usr_org_tp_001',       'cmp_org_demo_001', 'active', NOW()),
  ('mbr_org_tn_001',       'usr_org_tn_001',       'cmp_org_demo_001', 'active', NOW()),
  ('mbr_org_nv_nhom_001',  'usr_org_nv_nhom_001',  'cmp_org_demo_001', 'active', NOW()),
  ('mbr_org_nv_phong_001', 'usr_org_nv_phong_001', 'cmp_org_demo_001', 'active', NOW())
ON DUPLICATE KEY UPDATE
  membership_status = VALUES(membership_status),
  effective_from    = VALUES(effective_from);

-- ─── Roles ────────────────────────────────────────────────────────────────────
-- admin_doanh_nghiep: full company admin
-- truong_phong:       department head (disclosure + workflow read/review)
-- truong_nhom:        team head (disclosure view + workflow read)
-- nhan_vien:          common user (disclosure + workflow view only)

INSERT INTO roles (role_id, company_id, role_code, role_name, status) VALUES
  ('role_org_admin_001',   'cmp_org_demo_001', 'admin_doanh_nghiep', 'Admin Doanh Nghiệp', 'active'),
  ('role_org_tp_001',      'cmp_org_demo_001', 'truong_phong',        'Trưởng Phòng',       'active'),
  ('role_org_tn_001',      'cmp_org_demo_001', 'truong_nhom',         'Trưởng Tổ/Nhóm',    'active'),
  ('role_org_nv_001',      'cmp_org_demo_001', 'nhan_vien',           'Nhân Viên',          'active')
ON DUPLICATE KEY UPDATE
  role_name = VALUES(role_name),
  status    = VALUES(status);

-- ─── Permissions (upsert by code — IDs may already exist from earlier seeds) ──
-- Only insert codes that may be genuinely new; existing codes keep their IDs.

INSERT INTO permissions (permission_id, permission_code, permission_name, module_name, status) VALUES
  ('perm_org_022', 'recipient.view',                 'View recipients',              'admin',    'active'),
  ('perm_org_023', 'recipient.manage',               'Manage recipients',            'admin',    'active'),
  ('perm_org_026', 'disclosure_type.config.read',    'Read disclosure type config',  'cms',      'active'),
  ('perm_org_027', 'disclosure_type.config.write',   'Write disclosure type config', 'cms',      'active')
ON DUPLICATE KEY UPDATE
  permission_name = VALUES(permission_name),
  module_name     = VALUES(module_name),
  status          = VALUES(status);

-- ─── Role → Permissions ───────────────────────────────────────────────────────
-- Use SELECT JOIN so permission_id is always resolved from the actual permissions
-- table, regardless of which seed or migration created the permission row first.

-- admin_doanh_nghiep: full access
INSERT INTO role_permissions (role_id, permission_id, status)
SELECT 'role_org_admin_001', p.permission_id, 'active'
FROM permissions p
WHERE p.permission_code IN (
  'dashboard.view','company.view','company.edit',
  'disclosure.view','disclosure.create','disclosure.edit','disclosure.approve','disclosure.publish',
  'deadline.view','deadline.create','deadline.manage','deadline.assign',
  'workflow.read','workflow.review','workflow.approve','workflow.confirm',
  'workflow.step.confirm','workflow.step.override',
  'user.view','user.edit','rbac.manage','system.settings',
  'recipient.view','recipient.manage','alert.channels.manage',
  'disclosure_type.config.read','disclosure_type.config.write',
  'template.workflow.override.read','template.workflow.override.write',
  'template.workflow.override.approve','template.workflow.override.reset',
  'ad_hoc_alert.read','ad_hoc_alert.propose','ad_hoc_alert.focal_review','ad_hoc_alert.admin_review',
  'ad_hoc_alert.process_control'
) AND p.status = 'active'
ON DUPLICATE KEY UPDATE status = VALUES(status);

-- truong_phong: disclosure + workflow read/review + ad_hoc propose+focal_review+process_control
INSERT INTO role_permissions (role_id, permission_id, status)
SELECT 'role_org_tp_001', p.permission_id, 'active'
FROM permissions p
WHERE p.permission_code IN (
  'dashboard.view','company.view',
  'disclosure.view','disclosure.create','disclosure.edit','disclosure.approve',
  'deadline.view','workflow.read','workflow.review','workflow.step.confirm',
  'recipient.view','disclosure_type.config.read',
  'template.workflow.override.read','template.workflow.override.write','template.workflow.override.approve',
  'ad_hoc_alert.read','ad_hoc_alert.propose','ad_hoc_alert.focal_review','ad_hoc_alert.process_control'
) AND p.status = 'active'
ON DUPLICATE KEY UPDATE status = VALUES(status);

-- truong_nhom: view + workflow read + ad_hoc read+propose
INSERT INTO role_permissions (role_id, permission_id, status)
SELECT 'role_org_tn_001', p.permission_id, 'active'
FROM permissions p
WHERE p.permission_code IN (
  'dashboard.view','company.view',
  'disclosure.view','disclosure.create','disclosure.edit',
  'deadline.view','workflow.read','workflow.review',
  'ad_hoc_alert.read','ad_hoc_alert.propose'
) AND p.status = 'active'
ON DUPLICATE KEY UPDATE status = VALUES(status);

-- nhan_vien: view only
INSERT INTO role_permissions (role_id, permission_id, status)
SELECT 'role_org_nv_001', p.permission_id, 'active'
FROM permissions p
WHERE p.permission_code IN (
  'dashboard.view','company.view',
  'disclosure.view','deadline.view','workflow.read',
  'ad_hoc_alert.read'
) AND p.status = 'active'
ON DUPLICATE KEY UPDATE status = VALUES(status);

-- ─── Membership → Roles ───────────────────────────────────────────────────────

INSERT INTO membership_roles (membership_id, role_id, status, effective_from) VALUES
  ('mbr_org_admin_001',    'role_org_admin_001', 'active', NOW()),
  ('mbr_org_tp_001',       'role_org_tp_001',    'active', NOW()),
  ('mbr_org_tn_001',       'role_org_tn_001',    'active', NOW()),
  ('mbr_org_nv_nhom_001',  'role_org_nv_001',    'active', NOW()),
  ('mbr_org_nv_phong_001', 'role_org_nv_001',    'active', NOW())
ON DUPLICATE KEY UPDATE
  status         = VALUES(status),
  effective_from = VALUES(effective_from);

-- ─── Org units ────────────────────────────────────────────────────────────────
-- Dept 1: Phòng Pháp chế
--   Team 1a: Tổ Tư vấn Pháp lý
--   Team 1b: Tổ Hồ sơ & Hợp đồng
-- Dept 2: Phòng Kế toán
--   Team 2a: Tổ Kế toán Thuế
--   Team 2b: Tổ Kiểm soát Nội bộ

INSERT INTO org_units (org_unit_id, company_id, parent_org_unit_id, unit_code, unit_name, unit_type, status) VALUES
  ('ou_org_legal_001',    'cmp_org_demo_001', NULL,               'legal',         'Phòng Pháp chế',            'department', 'active'),
  ('ou_org_legal_tv_001', 'cmp_org_demo_001', 'ou_org_legal_001', 'legal-tv',      'Tổ Tư vấn Pháp lý',         'team',       'active'),
  ('ou_org_legal_hd_001', 'cmp_org_demo_001', 'ou_org_legal_001', 'legal-hd',      'Tổ Hồ sơ & Hợp đồng',       'team',       'active'),
  ('ou_org_acct_001',     'cmp_org_demo_001', NULL,               'accounting',    'Phòng Kế toán',              'department', 'active'),
  ('ou_org_acct_tax_001', 'cmp_org_demo_001', 'ou_org_acct_001',  'accounting-tax','Tổ Kế toán Thuế',            'team',       'active'),
  ('ou_org_acct_ic_001',  'cmp_org_demo_001', 'ou_org_acct_001',  'accounting-ic', 'Tổ Kiểm soát Nội bộ',        'team',       'active')
ON DUPLICATE KEY UPDATE
  unit_name          = VALUES(unit_name),
  parent_org_unit_id = VALUES(parent_org_unit_id),
  status             = VALUES(status);

-- ─── Org unit closure ─────────────────────────────────────────────────────────
-- depth=0: self-reference for every node
-- depth=1: parent→child for each team under its department

INSERT INTO org_unit_closure (ancestor_org_unit_id, descendant_org_unit_id, depth) VALUES
  -- self-references
  ('ou_org_legal_001',    'ou_org_legal_001',    0),
  ('ou_org_legal_tv_001', 'ou_org_legal_tv_001', 0),
  ('ou_org_legal_hd_001', 'ou_org_legal_hd_001', 0),
  ('ou_org_acct_001',     'ou_org_acct_001',     0),
  ('ou_org_acct_tax_001', 'ou_org_acct_tax_001', 0),
  ('ou_org_acct_ic_001',  'ou_org_acct_ic_001',  0),
  -- legal dept → its teams (depth 1)
  ('ou_org_legal_001',    'ou_org_legal_tv_001', 1),
  ('ou_org_legal_001',    'ou_org_legal_hd_001', 1),
  -- accounting dept → its teams (depth 1)
  ('ou_org_acct_001',     'ou_org_acct_tax_001', 1),
  ('ou_org_acct_001',     'ou_org_acct_ic_001',  1)
ON DUPLICATE KEY UPDATE depth = VALUES(depth);

-- ─── Org unit memberships ─────────────────────────────────────────────────────
-- admin:           not in any org_unit (company-level role, no dept scope)
-- truong_phong:    heads Phòng Pháp chế → position_code='truong_phong' in legal dept
-- truong_nhom:     heads Tổ Tư vấn Pháp lý → position_code='truong_nhom' in team
-- nv_nhom:         regular member of Tổ Tư vấn Pháp lý → position_code='nhan_vien' in team
-- nv_phong:        regular member of Phòng Pháp chế, no team → position_code='nhan_vien' in dept only

INSERT INTO org_unit_memberships (company_id, membership_id, org_unit_id, position_code, status) VALUES
  ('cmp_org_demo_001', 'mbr_org_tp_001',       'ou_org_legal_001',    'truong_phong', 'active'),
  ('cmp_org_demo_001', 'mbr_org_tn_001',       'ou_org_legal_tv_001', 'truong_nhom',  'active'),
  ('cmp_org_demo_001', 'mbr_org_nv_nhom_001',  'ou_org_legal_tv_001', 'nhan_vien',    'active'),
  ('cmp_org_demo_001', 'mbr_org_nv_phong_001', 'ou_org_legal_001',    'nhan_vien',    'active')
ON DUPLICATE KEY UPDATE
  position_code = VALUES(position_code),
  status        = VALUES(status);

-- ─── Legacy departments (mirrors org_units dept nodes) ────────────────────────

INSERT INTO departments (department_id, company_id, department_code, department_name, status) VALUES
  ('dep_org_legal_001', 'cmp_org_demo_001', 'legal',      'Phòng Pháp chế', 'active'),
  ('dep_org_acct_001',  'cmp_org_demo_001', 'accounting', 'Phòng Kế toán',  'active')
ON DUPLICATE KEY UPDATE
  department_name = VALUES(department_name),
  status          = VALUES(status);

-- ─── Legacy department memberships ────────────────────────────────────────────
-- All users who belong to the Legal dept (via org_unit or direct dept assignment)
-- go into dep_org_legal_001. Admin is added explicitly to show company admin is
-- a member of at least one dept for legacy queries.

INSERT INTO department_memberships (membership_id, department_id, status, effective_from) VALUES
  ('mbr_org_admin_001',    'dep_org_legal_001', 'active', NOW()),
  ('mbr_org_tp_001',       'dep_org_legal_001', 'active', NOW()),
  ('mbr_org_tn_001',       'dep_org_legal_001', 'active', NOW()),
  ('mbr_org_nv_nhom_001',  'dep_org_legal_001', 'active', NOW()),
  ('mbr_org_nv_phong_001', 'dep_org_legal_001', 'active', NOW())
ON DUPLICATE KEY UPDATE
  status         = VALUES(status),
  effective_from = VALUES(effective_from);

SET FOREIGN_KEY_CHECKS = 1;
