-- Revert is best-effort on DEV; restore general department and clear task assignee overrides.

UPDATE disclosure_records
SET department_id = 'general'
WHERE company_id = 'c_001'
  AND department_id IN ('d_legal', 'd_ir');

UPDATE workflow_tasks wt
INNER JOIN workflow_instances wi
  ON wi.workflow_instance_id = wt.workflow_instance_id
 AND wi.company_id = wt.company_id
INNER JOIN disclosure_records dr
  ON dr.company_id = wi.company_id
 AND dr.record_id = wi.record_id
SET wt.assignee_membership_id = 'm_102'
WHERE wt.company_id = 'c_001'
  AND wt.assignee_membership_id = 'm_105'
  AND dr.department_id = 'general';
