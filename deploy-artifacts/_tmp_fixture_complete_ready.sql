-- DEV fixture: Draft alert ready for Tenant complete click (do NOT submit)
-- Record stays Draft so ListRows V1 membership keeps it; tasks confirmed for readiness.
SELECT task_id, workflow_instance_id, status, assignee_membership_id
FROM workflow_tasks
WHERE workflow_instance_id = (
  SELECT workflow_instance_id FROM workflow_instances
  WHERE record_id = '01a0ba9c-5c81-7a19-b75a-ac524480fbf7'
  LIMIT 1
);

UPDATE workflow_tasks
SET status = 'confirmed', updated_at = NOW(3)
WHERE workflow_instance_id = (
  SELECT workflow_instance_id FROM workflow_instances
  WHERE record_id = '01a0ba9c-5c81-7a19-b75a-ac524480fbf7'
  LIMIT 1
)
AND status <> 'confirmed';

SELECT task_id, status FROM workflow_tasks
WHERE workflow_instance_id = (
  SELECT workflow_instance_id FROM workflow_instances
  WHERE record_id = '01a0ba9c-5c81-7a19-b75a-ac524480fbf7'
  LIMIT 1
);

SELECT record_id, status, submitted_at FROM disclosure_records
WHERE record_id = '01a0ba9c-5c81-7a19-b75a-ac524480fbf7';
