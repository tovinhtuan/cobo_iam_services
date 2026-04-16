-- Demo smoke checks (minimal): 5 quick queries for QA/frontend.
-- Usage:
--   mysql -u user -p cobo_iam < migrations/seed_demo_smoke_checks_min.sql

SET NAMES utf8mb4;

SELECT '01_core_counts_should_match' AS check_name;
SELECT 'companies=2' AS expected, COUNT(*) AS actual
FROM companies
WHERE company_id IN ('cmp_demo_001', 'cmp_demo_002')
UNION ALL
SELECT 'users=2', COUNT(*) FROM users WHERE user_id IN ('usr_demo_admin_001', 'usr_demo_reviewer_001')
UNION ALL
SELECT 'memberships=3', COUNT(*) FROM memberships WHERE membership_id IN ('mbr_demo_admin_001', 'mbr_demo_admin_002', 'mbr_demo_reviewer_001')
UNION ALL
SELECT 'roles=3', COUNT(*) FROM roles WHERE role_id IN ('role_demo_admin_001', 'role_demo_viewer_002', 'role_demo_reviewer_001')
UNION ALL
SELECT 'disclosure_records=3', COUNT(*) FROM disclosure_records WHERE record_id IN ('rec_demo_001', 'rec_demo_002', 'rec_demo_003')
UNION ALL
SELECT 'workflow_instances=2', COUNT(*) FROM workflow_instances WHERE workflow_instance_id IN ('wf_demo_001', 'wf_demo_002')
UNION ALL
SELECT 'workflow_tasks=2', COUNT(*) FROM workflow_tasks WHERE task_id IN ('task_demo_001', 'task_demo_002')
UNION ALL
SELECT 'notification_jobs=2', COUNT(*) FROM notification_jobs WHERE notification_job_id IN ('noti_job_demo_001', 'noti_job_demo_002')
UNION ALL
SELECT 'notification_deliveries=3', COUNT(*) FROM notification_deliveries WHERE notification_delivery_id IN ('noti_del_demo_001', 'noti_del_demo_002', 'noti_del_demo_003');

SELECT '02_login_and_company_context_should_exist' AS check_name;
SELECT
  u.login_id,
  COUNT(DISTINCT m.membership_id) AS membership_count,
  GROUP_CONCAT(DISTINCT c.company_code ORDER BY c.company_code SEPARATOR ', ') AS companies
FROM users u
LEFT JOIN memberships m ON m.user_id = u.user_id AND m.membership_status = 'active'
LEFT JOIN companies c ON c.company_id = m.company_id
WHERE u.user_id = 'usr_demo_admin_001'
GROUP BY u.login_id;

SELECT '03_admin_permissions_should_be_non_empty' AS check_name;
SELECT
  m.membership_id,
  COUNT(DISTINCT p.permission_id) AS permission_count,
  MAX(CASE WHEN p.permission_code = 'view_dashboard' THEN 1 ELSE 0 END) AS has_view_dashboard,
  MAX(CASE WHEN p.permission_code = 'disclosure.view' THEN 1 ELSE 0 END) AS has_disclosure_view,
  MAX(CASE WHEN p.permission_code = 'rbac.manage' THEN 1 ELSE 0 END) AS has_rbac_manage
FROM memberships m
INNER JOIN membership_roles mr ON mr.membership_id = m.membership_id AND mr.status = 'active'
INNER JOIN role_permissions rp ON rp.role_id = mr.role_id AND rp.status = 'active'
INNER JOIN permissions p ON p.permission_id = rp.permission_id AND p.status = 'active'
WHERE m.membership_id = 'mbr_demo_admin_001'
GROUP BY m.membership_id;

SELECT '04_disclosure_statuses_should_cover_draft_submitted_approved' AS check_name;
SELECT status, COUNT(*) AS total
FROM disclosure_records
WHERE record_id IN ('rec_demo_001', 'rec_demo_002', 'rec_demo_003')
GROUP BY status
ORDER BY status;

SELECT '05_workflow_and_notification_should_exist' AS check_name;
SELECT
  (SELECT COUNT(*) FROM workflow_tasks WHERE task_id IN ('task_demo_001', 'task_demo_002')) AS workflow_task_count,
  (SELECT COUNT(*) FROM workflow_tasks WHERE task_id = 'task_demo_001' AND status = 'pending') AS pending_task_count,
  (SELECT COUNT(*) FROM notification_jobs WHERE notification_job_id IN ('noti_job_demo_001', 'noti_job_demo_002')) AS notification_job_count,
  (SELECT COUNT(*) FROM notification_deliveries WHERE notification_delivery_id IN ('noti_del_demo_001', 'noti_del_demo_002', 'noti_del_demo_003')) AS notification_delivery_count;
