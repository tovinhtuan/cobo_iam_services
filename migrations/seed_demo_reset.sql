-- Demo reset seed: remove data created by seed_demo_package_1.sql and seed_demo_package_2.sql.
-- Run before re-seeding when you want a clean demo dataset without dropping schema.

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

DELETE FROM notification_deliveries
WHERE notification_delivery_id IN (
  'noti_del_demo_001',
  'noti_del_demo_002',
  'noti_del_demo_003'
);

DELETE FROM notification_jobs
WHERE notification_job_id IN (
  'noti_job_demo_001',
  'noti_job_demo_002'
);

DELETE FROM workflow_tasks
WHERE task_id IN (
  'task_demo_001',
  'task_demo_002'
);

DELETE FROM workflow_instances
WHERE workflow_instance_id IN (
  'wf_demo_001',
  'wf_demo_002'
);

DELETE FROM disclosure_records
WHERE record_id IN (
  'rec_demo_001',
  'rec_demo_002',
  'rec_demo_003'
);

DELETE FROM assignments
WHERE assignment_id IN (
  'asg_demo_001',
  'asg_demo_002',
  'asg_demo_003',
  'asg_demo_004'
);

DELETE FROM membership_effective_responsibilities
WHERE (company_id = 'cmp_demo_001' AND membership_id IN ('mbr_demo_admin_001', 'mbr_demo_reviewer_001'))
   OR (company_id = 'cmp_demo_002' AND membership_id = 'mbr_demo_admin_002');

DELETE FROM membership_titles
WHERE membership_id IN (
  'mbr_demo_admin_001',
  'mbr_demo_admin_002',
  'mbr_demo_reviewer_001'
);

DELETE FROM titles
WHERE title_id IN (
  'ttl_demo_head_legal_001',
  'ttl_demo_manager_ops_002',
  'ttl_demo_reviewer_001'
);

DELETE FROM department_memberships
WHERE membership_id IN (
  'mbr_demo_admin_001',
  'mbr_demo_admin_002',
  'mbr_demo_reviewer_001'
);

DELETE FROM departments
WHERE department_id IN (
  'dep_demo_legal_001',
  'dep_demo_ir_001',
  'dep_demo_ops_002',
  'dep_demo_compliance_001'
);

DELETE FROM membership_roles
WHERE membership_id IN (
  'mbr_demo_admin_001',
  'mbr_demo_admin_002',
  'mbr_demo_reviewer_001'
);

DELETE FROM role_permissions
WHERE role_id IN (
  'role_demo_admin_001',
  'role_demo_viewer_002',
  'role_demo_reviewer_001'
);

DELETE FROM roles
WHERE role_id IN (
  'role_demo_admin_001',
  'role_demo_viewer_002',
  'role_demo_reviewer_001'
);

DELETE FROM permissions
WHERE permission_id IN (
  'perm_demo_001',
  'perm_demo_002',
  'perm_demo_003',
  'perm_demo_004',
  'perm_demo_005',
  'perm_demo_006',
  'perm_demo_007',
  'perm_demo_008',
  'perm_demo_009',
  'perm_demo_010',
  'perm_demo_011',
  'perm_demo_012',
  'perm_demo_013',
  'perm_demo_014',
  'perm_demo_015',
  'perm_demo_016',
  'perm_demo_017',
  'perm_demo_018'
);

DELETE FROM memberships
WHERE membership_id IN (
  'mbr_demo_admin_001',
  'mbr_demo_admin_002',
  'mbr_demo_reviewer_001'
);

DELETE FROM credentials
WHERE credential_id IN (
  'cred_demo_admin_001',
  'cred_demo_reviewer_001'
);

DELETE FROM users
WHERE user_id IN (
  'usr_demo_admin_001',
  'usr_demo_reviewer_001'
);

DELETE FROM companies
WHERE company_id IN (
  'cmp_demo_001',
  'cmp_demo_002'
);

SET FOREIGN_KEY_CHECKS = 1;
