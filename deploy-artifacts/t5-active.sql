SELECT wi.workflow_instance_id, wi.record_id, wi.status, wi.current_step_code, wi.company_id
FROM workflow_instances wi
WHERE wi.company_id='c_001' AND wi.record_id IN (
  SELECT proposal_id FROM ad_hoc_proposals WHERE company_id='c_001' AND status='approved'
) AND wi.status NOT IN ('completed','cancelled','canceled')
ORDER BY wi.updated_at DESC LIMIT 10;

SELECT wt.workflow_task_id, wt.workflow_instance_id, wt.step_code, wt.status, wt.assignee_membership_id, wt.schema_version
FROM workflow_tasks wt
JOIN workflow_instances wi ON wi.workflow_instance_id=wt.workflow_instance_id
WHERE wi.company_id='c_001' AND wi.record_id='019feb2f-0de4-7153-8df2-62ed40d4d0ae'
LIMIT 20;

SELECT * FROM workflow_task_assignees WHERE workflow_task_id IN (
  SELECT workflow_task_id FROM workflow_tasks WHERE workflow_instance_id IN (
    SELECT workflow_instance_id FROM workflow_instances WHERE record_id='019feb2f-0de4-7153-8df2-62ed40d4d0ae'
  )
) LIMIT 20;
