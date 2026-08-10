SELECT proposal_id, company_id, created_by, status FROM ad_hoc_proposals WHERE company_id <> 'c_001' ORDER BY created_at DESC LIMIT 5;
SELECT p.proposal_id, p.status, wi.current_step_code, wi.status AS wi_status
FROM ad_hoc_proposals p
JOIN workflow_instances wi ON wi.entity_id = p.proposal_id
WHERE p.company_id='c_001' AND wi.status IN ('active','running','in_progress')
LIMIT 10;
SELECT DISTINCT status FROM workflow_instances LIMIT 20;
