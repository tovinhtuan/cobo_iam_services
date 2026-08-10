DESCRIBE workflow_instances;
SELECT proposal_id, status, created_by FROM ad_hoc_proposals WHERE company_id='c_001' AND status IN ('approved','in_progress','processing','published') ORDER BY updated_at DESC LIMIT 15;
SELECT id, status, current_step_code, schema_version, related_entity_id, related_entity_type FROM workflow_instances ORDER BY updated_at DESC LIMIT 5;
