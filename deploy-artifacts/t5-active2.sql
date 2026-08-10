DESCRIBE workflow_tasks;
DESCRIBE workflow_task_assignees;
SELECT wi.workflow_instance_id, wi.record_id, wi.status, wi.current_step_code
FROM workflow_instances wi
WHERE wi.company_id='c_001' AND wi.record_id LIKE '019feb%'
ORDER BY wi.updated_at DESC LIMIT 15;
