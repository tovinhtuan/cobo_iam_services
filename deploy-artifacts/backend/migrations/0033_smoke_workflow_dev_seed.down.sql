DELETE FROM ad_hoc_proposals WHERE proposal_id = 'adhoc_smoke_c001_001';
DELETE FROM workflow_step_milestones WHERE workflow_instance_id = 'wf_smoke_c001_001';
DELETE FROM workflow_tasks WHERE workflow_instance_id = 'wf_smoke_c001_001';
DELETE FROM workflow_instances WHERE workflow_instance_id = 'wf_smoke_c001_001';
DELETE FROM disclosure_records WHERE record_id = 'rec_smoke_wf_c001';

DELETE rp FROM role_permissions rp
INNER JOIN permissions p ON p.permission_id = rp.permission_id
WHERE p.permission_code IN (
  'ad_hoc_alert.read',
  'ad_hoc_alert.propose',
  'ad_hoc_alert.focal_review',
  'ad_hoc_alert.admin_review',
  'workflow.read',
  'workflow.review',
  'workflow.approve',
  'workflow.confirm'
);

DELETE FROM permissions WHERE permission_id IN (
  '10000000-0001-4000-8000-000000000012',
  '10000000-0001-4000-8000-000000000013',
  '10000000-0001-4000-8000-000000000014',
  '10000000-0001-4000-8000-000000000015',
  '10000000-0001-4000-8000-000000000020',
  '10000000-0001-4000-8000-000000000021',
  '10000000-0001-4000-8000-000000000022',
  '10000000-0001-4000-8000-000000000023'
);
