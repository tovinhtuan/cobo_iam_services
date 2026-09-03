SET NAMES utf8mb4;
SET @keep_id = 'bang-tinh-luong-nhan-vien-ban-sao-2' COLLATE utf8mb4_unicode_ci;

SELECT 'would_delete_ad_hoc_proposals' AS metric, COUNT(*) AS cnt FROM ad_hoc_proposals WHERE type_id COLLATE utf8mb4_unicode_ci <> @keep_id;
SELECT 'would_delete_alert_template_configs' AS metric, COUNT(*) AS cnt FROM alert_template_configs WHERE type_id COLLATE utf8mb4_unicode_ci <> @keep_id;
SELECT 'would_delete_workflow_override_conflicts' AS metric, COUNT(*) AS cnt FROM workflow_override_conflicts WHERE type_id COLLATE utf8mb4_unicode_ci <> @keep_id;
SELECT 'would_delete_global_workflow_versions' AS metric, COUNT(*) AS cnt FROM global_workflow_versions WHERE type_id COLLATE utf8mb4_unicode_ci <> @keep_id;
SELECT 'would_delete_deadline_confirmations' AS metric, COUNT(*) AS cnt
FROM deadline_alert_confirmations dac
JOIN disclosure_records dr ON dr.record_id = dac.record_id
WHERE dr.type_id COLLATE utf8mb4_unicode_ci <> @keep_id;

SELECT kcu.TABLE_NAME, kcu.COLUMN_NAME, kcu.CONSTRAINT_NAME, kcu.REFERENCED_TABLE_NAME, rc.DELETE_RULE
FROM information_schema.KEY_COLUMN_USAGE kcu
JOIN information_schema.REFERENTIAL_CONSTRAINTS rc
  ON rc.CONSTRAINT_SCHEMA = kcu.CONSTRAINT_SCHEMA AND rc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME AND rc.TABLE_NAME = kcu.TABLE_NAME
WHERE kcu.TABLE_SCHEMA = 'cobo_iam' AND kcu.REFERENCED_TABLE_NAME = 'disclosure_types'
ORDER BY kcu.TABLE_NAME;
