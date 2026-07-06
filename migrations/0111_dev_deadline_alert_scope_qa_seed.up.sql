-- DEV QA seed: differentiate deadline alert data-scope for c_001 scoped users.
-- First 10 non-draft records -> d_legal; remainder -> d_ir.
-- Assign m_105 as active task assignee on 3 d_ir records (cross-dept assignee visibility).

SET @rn := 0;

UPDATE disclosure_records dr
INNER JOIN (
  SELECT record_id, (@rn := @rn + 1) AS rn
  FROM disclosure_records
  WHERE company_id = 'c_001'
    AND LOWER(TRIM(status)) <> 'draft'
  ORDER BY created_at ASC, record_id ASC
) ranked ON ranked.record_id = dr.record_id
SET dr.department_id = CASE WHEN ranked.rn <= 10 THEN 'd_legal' ELSE 'd_ir' END
WHERE dr.company_id = 'c_001';

UPDATE workflow_tasks wt
INNER JOIN workflow_instances wi
  ON wi.workflow_instance_id = wt.workflow_instance_id
 AND wi.company_id = wt.company_id
INNER JOIN (
  SELECT record_id
  FROM disclosure_records
  WHERE company_id = 'c_001'
    AND department_id = 'd_ir'
    AND LOWER(TRIM(status)) <> 'draft'
  ORDER BY created_at ASC, record_id ASC
  LIMIT 3
) ir ON ir.record_id = wi.record_id
SET wt.assignee_membership_id = 'm_105'
WHERE wt.company_id = 'c_001'
  AND LOWER(TRIM(wt.status)) NOT IN ('completed', 'done', 'cancelled', 'skipped');
