SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

DELETE FROM department_memberships WHERE membership_id IN ('mbr_org_admin_001','mbr_org_tp_001','mbr_org_tn_001','mbr_org_nv_nhom_001','mbr_org_nv_phong_001');
DELETE FROM departments WHERE department_id IN ('dep_org_legal_001','dep_org_acct_001');
DELETE FROM org_unit_memberships WHERE membership_id IN ('mbr_org_tp_001','mbr_org_tn_001','mbr_org_nv_nhom_001','mbr_org_nv_phong_001');
DELETE FROM org_unit_closure WHERE ancestor_org_unit_id IN ('ou_org_legal_001','ou_org_legal_tv_001','ou_org_legal_hd_001','ou_org_acct_001','ou_org_acct_tax_001','ou_org_acct_ic_001');
DELETE FROM org_units WHERE org_unit_id IN ('ou_org_legal_tv_001','ou_org_legal_hd_001','ou_org_acct_tax_001','ou_org_acct_ic_001','ou_org_legal_001','ou_org_acct_001');
DELETE FROM membership_roles WHERE membership_id IN ('mbr_org_admin_001','mbr_org_tp_001','mbr_org_tn_001','mbr_org_nv_nhom_001','mbr_org_nv_phong_001');
DELETE FROM role_permissions WHERE role_id IN ('role_org_admin_001','role_org_tp_001','role_org_tn_001','role_org_nv_001');
DELETE FROM permissions WHERE permission_id LIKE 'perm_org_%';
DELETE FROM roles WHERE role_id IN ('role_org_admin_001','role_org_tp_001','role_org_tn_001','role_org_nv_001');
DELETE FROM memberships WHERE membership_id IN ('mbr_org_admin_001','mbr_org_tp_001','mbr_org_tn_001','mbr_org_nv_nhom_001','mbr_org_nv_phong_001');
DELETE FROM credentials WHERE credential_id IN ('cred_org_admin_001','cred_org_tp_001','cred_org_tn_001','cred_org_nv_nhom_001','cred_org_nv_phong_001');
DELETE FROM users WHERE user_id IN ('usr_org_admin_001','usr_org_tp_001','usr_org_tn_001','usr_org_nv_nhom_001','usr_org_nv_phong_001');
DELETE FROM companies WHERE company_id = 'cmp_org_demo_001';

SET FOREIGN_KEY_CHECKS = 1;
