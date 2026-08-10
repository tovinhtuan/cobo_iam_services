SELECT p.proposal_id, p.status, p.created_by, p.workflow_instance_id
FROM ad_hoc_proposals p
WHERE p.company_id='c_001' AND p.status='approved'
ORDER BY p.updated_at DESC LIMIT 10;
DESCRIBE ad_hoc_proposals;
