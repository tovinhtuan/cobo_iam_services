SET NAMES utf8mb4;
SET @keep_id = 'bang-tinh-luong-nhan-vien-ban-sao-2';

-- KEEP baseline
SELECT 'KEEP_BASELINE' AS section;
SELECT dt.type_id, dt.status, dt.active_version_no, dtv.name
FROM disclosure_types dt
JOIN disclosure_type_versions dtv ON dtv.type_id=dt.type_id AND dtv.version_no=dt.active_version_no
WHERE dt.type_id = @keep_id;

SELECT 'keep_versions' AS metric, COUNT(*) AS cnt FROM disclosure_type_versions WHERE type_id=@keep_id;
SELECT 'keep_periodic_cycles' AS metric, COUNT(*) AS cnt FROM periodic_cycles WHERE type_id=@keep_id;
SELECT 'keep_records' AS metric, COUNT(*) AS cnt FROM disclosure_records WHERE type_id=@keep_id;
SELECT 'keep_global_workflows' AS metric, COUNT(*) AS cnt FROM global_workflows WHERE type_id=@keep_id;
SELECT 'keep_company_prefs' AS metric, COUNT(*) AS cnt FROM company_type_preferences WHERE type_id=@keep_id;
SELECT 'keep_template_blocks' AS metric, COUNT(*) AS cnt FROM disclosure_template_blocks WHERE type_id=@keep_id;
SELECT 'keep_display_groups' AS metric, COUNT(*) AS cnt FROM template_display_groups WHERE template_id=@keep_id;
SELECT 'keep_workflow_overrides' AS metric, COUNT(*) AS cnt FROM company_template_workflow_overrides WHERE type_id=@keep_id;

-- DELETE set size
SELECT 'delete_roots' AS metric, COUNT(*) AS cnt FROM disclosure_types WHERE type_id <> @keep_id;

-- WOULD DELETE aggregates (all non-keep roots)
SELECT 'would_delete_versions' AS metric, COUNT(*) AS cnt
FROM disclosure_type_versions v
WHERE v.type_id <> @keep_id;

SELECT 'would_delete_periodic_cycles' AS metric, COUNT(*) AS cnt
FROM periodic_cycles WHERE type_id <> @keep_id;

SELECT 'would_delete_records' AS metric, COUNT(*) AS cnt
FROM disclosure_records WHERE type_id <> @keep_id;

SELECT 'would_delete_global_workflows' AS metric, COUNT(*) AS cnt
FROM global_workflows WHERE type_id <> @keep_id;

SELECT 'would_delete_global_steps' AS metric, COUNT(*) AS cnt
FROM global_workflow_steps WHERE type_id <> @keep_id;

SELECT 'would_delete_company_prefs' AS metric, COUNT(*) AS cnt
FROM company_type_preferences WHERE type_id <> @keep_id;

SELECT 'would_delete_template_blocks' AS metric, COUNT(*) AS cnt
FROM disclosure_template_blocks WHERE type_id <> @keep_id;

SELECT 'would_delete_display_groups' AS metric, COUNT(*) AS cnt
FROM template_display_groups WHERE template_id <> @keep_id;

SELECT 'would_delete_workflow_overrides' AS metric, COUNT(*) AS cnt
FROM company_template_workflow_overrides WHERE type_id <> @keep_id;

-- Records with business status (non-draft) for delete candidates
SELECT 'delete_candidate_submitted_records' AS metric, COUNT(*) AS cnt
FROM disclosure_records
WHERE type_id <> @keep_id AND submitted_at IS NOT NULL;

SELECT 'delete_candidate_non_draft_records' AS metric, COUNT(*) AS cnt
FROM disclosure_records
WHERE type_id <> @keep_id AND status <> 'draft';

-- Workflow instances tied to delete-candidate records
SELECT 'would_delete_workflow_instances' AS metric, COUNT(DISTINCT wi.workflow_instance_id) AS cnt
FROM workflow_instances wi
JOIN disclosure_records dr ON dr.record_id = wi.record_id
WHERE dr.type_id <> @keep_id;

SELECT 'would_delete_workflow_tasks' AS metric, COUNT(*) AS cnt
FROM workflow_tasks wt
JOIN workflow_instances wi ON wi.workflow_instance_id = wt.workflow_instance_id
JOIN disclosure_records dr ON dr.record_id = wi.record_id
WHERE dr.type_id <> @keep_id;

-- Deadline alert confirmations (if table exists)
SELECT 'would_delete_deadline_confirmations' AS metric, COUNT(*) AS cnt
FROM deadline_alert_confirmations dac
JOIN disclosure_records dr ON dr.record_id = dac.record_id
WHERE dr.type_id <> @keep_id;

-- FK constraints referencing disclosure_types
SELECT 'FK_AUDIT' AS section;
SELECT TABLE_NAME, COLUMN_NAME, CONSTRAINT_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME
FROM information_schema.KEY_COLUMN_USAGE
WHERE TABLE_SCHEMA = DATABASE()
  AND REFERENCED_TABLE_NAME = 'disclosure_types'
ORDER BY TABLE_NAME;

-- FK referencing disclosure_type_versions
SELECT TABLE_NAME, COLUMN_NAME, CONSTRAINT_NAME, REFERENCED_TABLE_NAME
FROM information_schema.KEY_COLUMN_USAGE
WHERE TABLE_SCHEMA = DATABASE()
  AND REFERENCED_TABLE_NAME = 'disclosure_type_versions'
ORDER BY TABLE_NAME;

-- ON DELETE rules for disclosure_types children
SELECT rc.CONSTRAINT_NAME, rc.TABLE_NAME, rc.REFERENCED_TABLE_NAME, rc.DELETE_RULE, rc.UPDATE_RULE
FROM information_schema.REFERENTIAL_CONSTRAINTS rc
WHERE rc.CONSTRAINT_SCHEMA = DATABASE()
  AND rc.REFERENCED_TABLE_NAME IN ('disclosure_types','disclosure_type_versions')
ORDER BY rc.REFERENCED_TABLE_NAME, rc.TABLE_NAME;

-- Worker container status hint
SELECT 'worker_running' AS note;
