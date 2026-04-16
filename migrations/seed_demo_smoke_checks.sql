-- Demo smoke checks: quick SELECTs to verify seed_demo_package_1.sql and seed_demo_package_2.sql.
-- Usage:
--   mysql -u user -p cobo_iam < migrations/seed_demo_smoke_checks.sql

SET NAMES utf8mb4;

SELECT '01_companies' AS check_name;
SELECT company_id, company_code, company_name, status
FROM companies
WHERE company_id IN ('cmp_demo_001', 'cmp_demo_002')
ORDER BY company_id;

SELECT '02_users_and_credentials' AS check_name;
SELECT u.user_id, u.login_id, u.full_name, u.account_status, c.credential_type, c.status AS credential_status
FROM users u
LEFT JOIN credentials c ON c.user_id = u.user_id
WHERE u.user_id IN ('usr_demo_admin_001', 'usr_demo_reviewer_001')
ORDER BY u.user_id;

SELECT '03_memberships_by_user' AS check_name;
SELECT u.login_id, m.membership_id, m.company_id, co.company_name, m.membership_status
FROM memberships m
INNER JOIN users u ON u.user_id = m.user_id
INNER JOIN companies co ON co.company_id = m.company_id
WHERE m.membership_id IN ('mbr_demo_admin_001', 'mbr_demo_admin_002', 'mbr_demo_reviewer_001')
ORDER BY u.login_id, m.company_id;

SELECT '04_roles_and_permissions_summary' AS check_name;
SELECT
  r.role_id,
  r.role_code,
  COUNT(DISTINCT rp.permission_id) AS permission_count
FROM roles r
LEFT JOIN role_permissions rp ON rp.role_id = r.role_id AND rp.status = 'active'
WHERE r.role_id IN ('role_demo_admin_001', 'role_demo_viewer_002', 'role_demo_reviewer_001')
GROUP BY r.role_id, r.role_code
ORDER BY r.role_code;

SELECT '05_admin_effective_permissions' AS check_name;
SELECT
  m.membership_id,
  GROUP_CONCAT(DISTINCT p.permission_code ORDER BY p.permission_code SEPARATOR ', ') AS permission_codes
FROM memberships m
INNER JOIN membership_roles mr ON mr.membership_id = m.membership_id AND mr.status = 'active'
INNER JOIN roles r ON r.role_id = mr.role_id AND r.status = 'active'
INNER JOIN role_permissions rp ON rp.role_id = r.role_id AND rp.status = 'active'
INNER JOIN permissions p ON p.permission_id = rp.permission_id AND p.status = 'active'
WHERE m.membership_id = 'mbr_demo_admin_001'
GROUP BY m.membership_id;

SELECT '06_departments_and_titles' AS check_name;
SELECT
  m.membership_id,
  GROUP_CONCAT(DISTINCT d.department_name ORDER BY d.department_name SEPARATOR ', ') AS departments,
  GROUP_CONCAT(DISTINCT t.title_name ORDER BY t.title_name SEPARATOR ', ') AS titles
FROM memberships m
LEFT JOIN department_memberships dm ON dm.membership_id = m.membership_id AND dm.status = 'active'
LEFT JOIN departments d ON d.department_id = dm.department_id AND d.status = 'active'
LEFT JOIN membership_titles mt ON mt.membership_id = m.membership_id AND mt.status = 'active'
LEFT JOIN titles t ON t.title_id = mt.title_id AND t.status = 'active'
WHERE m.membership_id IN ('mbr_demo_admin_001', 'mbr_demo_admin_002', 'mbr_demo_reviewer_001')
GROUP BY m.membership_id
ORDER BY m.membership_id;

SELECT '07_effective_responsibilities' AS check_name;
SELECT company_id, membership_id, responsibility_code, source_type, source_ref_id
FROM membership_effective_responsibilities
WHERE membership_id IN ('mbr_demo_admin_001', 'mbr_demo_admin_002', 'mbr_demo_reviewer_001')
ORDER BY company_id, membership_id, responsibility_code;

SELECT '08_disclosure_records' AS check_name;
SELECT record_id, company_id, department_id, title, status, created_by, updated_by
FROM disclosure_records
WHERE record_id IN ('rec_demo_001', 'rec_demo_002', 'rec_demo_003')
ORDER BY record_id;

SELECT '09_workflow_instances_and_tasks' AS check_name;
SELECT
  wi.workflow_instance_id,
  wi.record_id,
  wi.status AS workflow_status,
  wi.current_step_code,
  wt.task_id,
  wt.assignee_membership_id,
  wt.status AS task_status
FROM workflow_instances wi
LEFT JOIN workflow_tasks wt ON wt.workflow_instance_id = wi.workflow_instance_id
WHERE wi.workflow_instance_id IN ('wf_demo_001', 'wf_demo_002')
ORDER BY wi.workflow_instance_id, wt.task_id;

SELECT '10_assignments' AS check_name;
SELECT assignment_id, assignee_ref_id, resource_type, resource_id, assignment_kind, status
FROM assignments
WHERE assignment_id IN ('asg_demo_001', 'asg_demo_002', 'asg_demo_003', 'asg_demo_004')
ORDER BY assignment_id;

SELECT '11_notification_jobs_and_deliveries' AS check_name;
SELECT
  nj.notification_job_id,
  nj.event_type,
  nj.resource_id,
  nj.status AS job_status,
  nd.notification_delivery_id,
  nd.recipient,
  nd.status AS delivery_status
FROM notification_jobs nj
LEFT JOIN notification_deliveries nd ON nd.notification_job_id = nj.notification_job_id
WHERE nj.notification_job_id IN ('noti_job_demo_001', 'noti_job_demo_002')
ORDER BY nj.notification_job_id, nd.notification_delivery_id;

SELECT '12_counts_summary' AS check_name;
SELECT 'companies' AS entity, COUNT(*) AS total
FROM companies
WHERE company_id IN ('cmp_demo_001', 'cmp_demo_002')
UNION ALL
SELECT 'users', COUNT(*) FROM users WHERE user_id IN ('usr_demo_admin_001', 'usr_demo_reviewer_001')
UNION ALL
SELECT 'memberships', COUNT(*) FROM memberships WHERE membership_id IN ('mbr_demo_admin_001', 'mbr_demo_admin_002', 'mbr_demo_reviewer_001')
UNION ALL
SELECT 'roles', COUNT(*) FROM roles WHERE role_id IN ('role_demo_admin_001', 'role_demo_viewer_002', 'role_demo_reviewer_001')
UNION ALL
SELECT 'disclosure_records', COUNT(*) FROM disclosure_records WHERE record_id IN ('rec_demo_001', 'rec_demo_002', 'rec_demo_003')
UNION ALL
SELECT 'workflow_instances', COUNT(*) FROM workflow_instances WHERE workflow_instance_id IN ('wf_demo_001', 'wf_demo_002')
UNION ALL
SELECT 'workflow_tasks', COUNT(*) FROM workflow_tasks WHERE task_id IN ('task_demo_001', 'task_demo_002')
UNION ALL
SELECT 'notification_jobs', COUNT(*) FROM notification_jobs WHERE notification_job_id IN ('noti_job_demo_001', 'noti_job_demo_002')
UNION ALL
SELECT 'notification_deliveries', COUNT(*) FROM notification_deliveries WHERE notification_delivery_id IN ('noti_del_demo_001', 'noti_del_demo_002', 'noti_del_demo_003');
