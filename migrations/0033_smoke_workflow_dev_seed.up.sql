SET NAMES utf8mb4;

-- Portal smoke: workflow execution, ad-hoc permissions, sample in-progress disclosure with workflow instance.

INSERT INTO permissions (permission_id, permission_code, permission_name, module_name, status) VALUES
  ('10000000-0001-4000-8000-000000000012', 'ad_hoc_alert.read', 'Read ad-hoc proposals', 'ad_hoc', 'active'),
  ('10000000-0001-4000-8000-000000000013', 'ad_hoc_alert.propose', 'Propose ad-hoc workflow changes', 'ad_hoc', 'active'),
  ('10000000-0001-4000-8000-000000000014', 'ad_hoc_alert.focal_review', 'Focal review ad-hoc proposals', 'ad_hoc', 'active'),
  ('10000000-0001-4000-8000-000000000015', 'ad_hoc_alert.admin_review', 'Admin review ad-hoc proposals', 'ad_hoc', 'active'),
  ('10000000-0001-4000-8000-000000000020', 'workflow.read', 'Read workflow instance', 'workflow', 'active'),
  ('10000000-0001-4000-8000-000000000021', 'workflow.review', 'Review workflow task', 'workflow', 'active'),
  ('10000000-0001-4000-8000-000000000022', 'workflow.approve', 'Approve workflow task', 'workflow', 'active'),
  ('10000000-0001-4000-8000-000000000023', 'workflow.confirm', 'Confirm workflow task', 'workflow', 'active')
ON DUPLICATE KEY UPDATE
  permission_code = VALUES(permission_code),
  permission_name = VALUES(permission_name),
  module_name = VALUES(module_name),
  status = VALUES(status);

INSERT INTO role_permissions (role_id, permission_id, status)
SELECT r.role_id, p.permission_id, 'active'
FROM roles r
INNER JOIN permissions p ON p.permission_code IN (
  'ad_hoc_alert.read',
  'ad_hoc_alert.propose',
  'ad_hoc_alert.focal_review',
  'ad_hoc_alert.admin_review',
  'workflow.read',
  'workflow.review',
  'workflow.approve',
  'workflow.confirm'
)
WHERE r.company_id = 'c_001'
  AND r.role_code IN ('admin_doanh_nghiep', 'admin_web', 'full_access')
ON DUPLICATE KEY UPDATE status = VALUES(status);

INSERT INTO disclosure_records (
  record_id, company_id, type_id, department_id, title, summary, content,
  planned_date, status, created_by, updated_by
) VALUES (
  'rec_smoke_wf_c001',
  'c_001',
  'dt-custom-obligation',
  'd_legal',
  'Smoke workflow — in progress',
  'Seed record for portal workflow instance smoke tests.',
  'Workflow execution smoke body.',
  '2026-06-01',
  'in_progress',
  'u_admin_dn',
  'u_admin_dn'
)
ON DUPLICATE KEY UPDATE
  title = VALUES(title),
  summary = VALUES(summary),
  content = VALUES(content),
  status = VALUES(status),
  updated_by = VALUES(updated_by);

INSERT INTO workflow_instances (
  workflow_instance_id, company_id, record_id, status, current_step_code, created_by,
  t0_date, t0_policy, workflow_source
) VALUES (
  'wf_smoke_c001_001',
  'c_001',
  'rec_smoke_wf_c001',
  'in_progress',
  'review',
  'u_admin_dn',
  '2026-06-01',
  'system_date',
  'company_override'
)
ON DUPLICATE KEY UPDATE
  status = VALUES(status),
  current_step_code = VALUES(current_step_code),
  t0_date = VALUES(t0_date);

INSERT INTO workflow_tasks (
  task_id, company_id, workflow_instance_id, step_code, assignee_membership_id, status
) VALUES (
  'task_smoke_c001_001',
  'c_001',
  'wf_smoke_c001_001',
  'review',
  'm_102',
  'pending'
)
ON DUPLICATE KEY UPDATE
  step_code = VALUES(step_code),
  assignee_membership_id = VALUES(assignee_membership_id),
  status = VALUES(status);

INSERT INTO workflow_step_milestones (
  milestone_id, company_id, workflow_instance_id, step_id, step_order, milestone_type, scheduled_date
) VALUES
  ('ms_smoke_c001_start', 'c_001', 'wf_smoke_c001_001', 'ws-smoke-1', 1, 'step_start', '2026-06-01'),
  ('ms_smoke_c001_end', 'c_001', 'wf_smoke_c001_001', 'ws-smoke-1', 1, 'step_end', '2026-06-03')
ON DUPLICATE KEY UPDATE scheduled_date = VALUES(scheduled_date);

INSERT INTO ad_hoc_proposals (
  proposal_id, company_id, type_id, status, proposed_workflow_json, change_note, created_by
) VALUES (
  'adhoc_smoke_c001_001',
  'c_001',
  'dt-custom-obligation',
  'ad_hoc_draft',
  JSON_ARRAY(JSON_OBJECT('step_id', 'ws-smoke-1', 'processing_days', 3)),
  'Smoke ad-hoc proposal for portal list/detail',
  'm_102'
)
ON DUPLICATE KEY UPDATE
  status = VALUES(status),
  change_note = VALUES(change_note);
