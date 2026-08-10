SELECT wi.workflow_instance_id, wi.record_id, wi.status, wi.current_step_code
FROM workflow_instances wi
WHERE wi.company_id='c_001'
ORDER BY wi.updated_at DESC LIMIT 20;

SELECT wt.task_id, wt.workflow_instance_id, wt.step_code, wt.status, wt.assignee_membership_id
FROM workflow_tasks wt
WHERE wt.company_id='c_001' AND wt.status='pending'
ORDER BY wt.updated_at DESC LIMIT 15;

SELECT wta.task_id, wta.membership_id, wt.step_code, wt.status, wi.record_id
FROM workflow_task_assignees wta
JOIN workflow_tasks wt ON wt.task_id=wta.task_id
JOIN workflow_instances wi ON wi.workflow_instance_id=wt.workflow_instance_id
WHERE wi.company_id='c_001'
ORDER BY wta.created_at DESC LIMIT 20;
