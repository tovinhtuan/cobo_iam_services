SET NAMES utf8mb4;

-- Environment + inventory revalidation
SELECT DATABASE() AS db_name;
SELECT COUNT(*) AS total_roots FROM disclosure_types;

-- Exact KEEP name match
SELECT dt.type_id, dt.status, dt.active_version_no, dtv.name,
       (SELECT COUNT(*) FROM disclosure_type_versions v WHERE v.type_id = dt.type_id) AS version_count
FROM disclosure_types dt
JOIN disclosure_type_versions dtv
  ON dtv.type_id = dt.type_id AND dtv.version_no = dt.active_version_no
WHERE TRIM(dtv.name) = 'Bảng tính lương nhân viên tháng';

-- KEEP baseline
SET @keep_id = 'bang-tinh-luong-nhan-vien-ban-sao-2' COLLATE utf8mb4_unicode_ci;
SELECT 'keep_versions' AS m, COUNT(*) AS c FROM disclosure_type_versions WHERE type_id COLLATE utf8mb4_unicode_ci = @keep_id
UNION ALL SELECT 'keep_cycles', COUNT(*) FROM periodic_cycles WHERE type_id COLLATE utf8mb4_unicode_ci = @keep_id
UNION ALL SELECT 'keep_records', COUNT(*) FROM disclosure_records WHERE type_id COLLATE utf8mb4_unicode_ci = @keep_id
UNION ALL SELECT 'keep_blocks', COUNT(*) FROM disclosure_template_blocks WHERE type_id COLLATE utf8mb4_unicode_ci = @keep_id
UNION ALL SELECT 'keep_display_groups', COUNT(*) FROM template_display_groups WHERE template_id COLLATE utf8mb4_unicode_ci = @keep_id;

-- KEEP identities
SELECT version_no FROM disclosure_type_versions WHERE type_id COLLATE utf8mb4_unicode_ci = @keep_id ORDER BY version_no;
SELECT cycle_id FROM periodic_cycles WHERE type_id COLLATE utf8mb4_unicode_ci = @keep_id ORDER BY cycle_id;
SELECT record_id, status, submitted_at IS NOT NULL AS has_submitted FROM disclosure_records WHERE type_id COLLATE utf8mb4_unicode_ci = @keep_id ORDER BY record_id;
SELECT display_group_code FROM template_display_groups WHERE template_id COLLATE utf8mb4_unicode_ci = @keep_id ORDER BY display_group_code;

-- Current all root IDs
SELECT type_id FROM disclosure_types ORDER BY type_id;

-- Business records on delete candidates (frozen set will be applied in next script)
SELECT record_id, type_id, status, submitted_at IS NOT NULL AS has_submitted
FROM disclosure_records
WHERE type_id COLLATE utf8mb4_unicode_ci <> @keep_id
  AND (status <> 'draft' OR submitted_at IS NOT NULL)
ORDER BY type_id, record_id;

-- FK revalidation
SELECT kcu.TABLE_NAME, kcu.COLUMN_NAME, kcu.REFERENCED_TABLE_NAME, rc.DELETE_RULE
FROM information_schema.KEY_COLUMN_USAGE kcu
JOIN information_schema.REFERENTIAL_CONSTRAINTS rc
  ON rc.CONSTRAINT_SCHEMA = kcu.CONSTRAINT_SCHEMA
 AND rc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME
 AND rc.TABLE_NAME = kcu.TABLE_NAME
WHERE kcu.TABLE_SCHEMA = 'cobo_iam'
  AND kcu.REFERENCED_TABLE_NAME IN ('disclosure_types','disclosure_type_versions','disclosure_records','workflow_instances','workflow_tasks','company_template_workflow_overrides','global_workflows','periodic_cycles')
ORDER BY kcu.REFERENCED_TABLE_NAME, kcu.TABLE_NAME;
