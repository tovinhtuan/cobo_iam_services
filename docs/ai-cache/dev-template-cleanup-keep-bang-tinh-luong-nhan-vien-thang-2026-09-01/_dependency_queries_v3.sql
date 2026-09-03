SET NAMES utf8mb4;
SET @keep_id = 'bang-tinh-luong-nhan-vien-ban-sao-2' COLLATE utf8mb4_unicode_ci;

SELECT 'would_delete_global_steps' AS metric, COUNT(*) AS cnt
FROM global_workflow_steps gws
JOIN global_workflows gw ON gw.workflow_id = gws.workflow_id
WHERE gw.type_id <> @keep_id;

SELECT 'would_delete_company_prefs' AS metric, COUNT(*) AS cnt FROM company_type_preferences WHERE type_id COLLATE utf8mb4_unicode_ci <> @keep_id;
SELECT 'would_delete_template_blocks' AS metric, COUNT(*) AS cnt FROM disclosure_template_blocks WHERE type_id COLLATE utf8mb4_unicode_ci <> @keep_id;
SELECT 'would_delete_display_groups' AS metric, COUNT(*) AS cnt FROM template_display_groups WHERE template_id COLLATE utf8mb4_unicode_ci <> @keep_id;
SELECT 'would_delete_workflow_overrides' AS metric, COUNT(*) AS cnt FROM company_template_workflow_overrides WHERE type_id COLLATE utf8mb4_unicode_ci <> @keep_id;
SELECT 'would_delete_override_versions' AS metric, COUNT(*) AS cnt
FROM company_template_workflow_override_versions v
JOIN company_template_workflow_overrides o ON o.override_id = v.override_id
WHERE o.type_id COLLATE utf8mb4_unicode_ci <> @keep_id;

SELECT 'delete_candidate_submitted_records' AS metric, COUNT(*) AS cnt FROM disclosure_records WHERE type_id COLLATE utf8mb4_unicode_ci <> @keep_id AND submitted_at IS NOT NULL;
SELECT 'delete_candidate_non_draft_records' AS metric, COUNT(*) AS cnt FROM disclosure_records WHERE type_id COLLATE utf8mb4_unicode_ci <> @keep_id AND status <> 'draft';

SELECT 'would_delete_workflow_instances' AS metric, COUNT(DISTINCT wi.workflow_instance_id) AS cnt
FROM workflow_instances wi JOIN disclosure_records dr ON dr.record_id = wi.record_id WHERE dr.type_id COLLATE utf8mb4_unicode_ci <> @keep_id;

SELECT 'would_delete_workflow_tasks' AS metric, COUNT(*) AS cnt
FROM workflow_tasks wt JOIN workflow_instances wi ON wi.workflow_instance_id = wt.workflow_instance_id
JOIN disclosure_records dr ON dr.record_id = wi.record_id WHERE dr.type_id COLLATE utf8mb4_unicode_ci <> @keep_id;

SELECT rc.REFERENCED_TABLE_NAME, rc.TABLE_NAME, rc.CONSTRAINT_NAME, rc.DELETE_RULE
FROM information_schema.REFERENTIAL_CONSTRAINTS rc
WHERE rc.CONSTRAINT_SCHEMA = DATABASE() AND rc.REFERENCED_TABLE_NAME IN ('disclosure_types','disclosure_type_versions')
ORDER BY rc.REFERENCED_TABLE_NAME, rc.TABLE_NAME;
