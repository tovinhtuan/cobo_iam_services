-- verify_0104_workflow_override_conflicts.sql
-- Batch 4 verification (READ-ONLY, SELECT ONLY). Safe on any env: NO DDL/DML.
-- Every row's `status` MUST be 'PASS'. Kept as separate statements (small max_allowed_packet).

-- 1. Table exists.
SELECT '1.table_exists' AS check_name,
       IF(COUNT(*)=1,'PASS','FAIL') AS status, CONCAT('found=',COUNT(*)) AS detail
FROM information_schema.tables
WHERE table_schema=DATABASE() AND table_name='workflow_override_conflicts';

-- 2. All 23 required columns exist.
SELECT '2.all_columns_exist' AS check_name,
       IF(COUNT(*)=23,'PASS','FAIL') AS status, CONCAT('found=',COUNT(*),'/23') AS detail
FROM information_schema.columns
WHERE table_schema=DATABASE() AND table_name='workflow_override_conflicts'
  AND column_name IN ('id','company_id','type_id','override_id','override_version_no',
    'preview_id','base_version_no','target_version_no','conflict_key','step_key','field_path',
    'severity','conflict_type','global_old_json','global_new_json','company_value_json',
    'resolution_status','resolution','resolution_json','created_by','created_at','resolved_by',
    'resolved_at');

-- 3. updated_at column exists (24th, listed separately since it has an ON UPDATE clause unlike the rest).
SELECT '3.updated_at_exists' AS check_name,
       IF(COUNT(*)=1,'PASS','FAIL') AS status, CONCAT('found=',COUNT(*)) AS detail
FROM information_schema.columns
WHERE table_schema=DATABASE() AND table_name='workflow_override_conflicts' AND column_name='updated_at';

-- 4. Unique index on conflict_key exists.
SELECT '4.unique_conflict_key_index' AS check_name,
       IF(COUNT(*)>=1,'PASS','FAIL') AS status, CONCAT('found=',COUNT(*)) AS detail
FROM information_schema.statistics
WHERE table_schema=DATABASE() AND table_name='workflow_override_conflicts'
  AND index_name='uq_workflow_override_conflicts_conflict_key' AND non_unique=0;

-- 5. (company_id, type_id) index exists.
SELECT '5.company_type_index' AS check_name,
       IF(COUNT(*)>=1,'PASS','FAIL') AS status, CONCAT('found=',COUNT(*)) AS detail
FROM information_schema.statistics
WHERE table_schema=DATABASE() AND table_name='workflow_override_conflicts'
  AND index_name='idx_workflow_override_conflicts_company_type';

-- 6. preview_id index exists.
SELECT '6.preview_id_index' AS check_name,
       IF(COUNT(*)>=1,'PASS','FAIL') AS status, CONCAT('found=',COUNT(*)) AS detail
FROM information_schema.statistics
WHERE table_schema=DATABASE() AND table_name='workflow_override_conflicts'
  AND index_name='idx_workflow_override_conflicts_preview_id';

-- 7. resolution_status index exists.
SELECT '7.resolution_status_index' AS check_name,
       IF(COUNT(*)>=1,'PASS','FAIL') AS status, CONCAT('found=',COUNT(*)) AS detail
FROM information_schema.statistics
WHERE table_schema=DATABASE() AND table_name='workflow_override_conflicts'
  AND index_name='idx_workflow_override_conflicts_resolution_status';

-- 8. No FK constraints exist on this table (matches the codebase's existing no-FK convention).
SELECT '8.no_fk_constraints' AS check_name,
       IF(COUNT(*)=0,'PASS','FAIL') AS status, CONCAT('fk_count=',COUNT(*)) AS detail
FROM information_schema.table_constraints
WHERE table_schema=DATABASE() AND table_name='workflow_override_conflicts'
  AND constraint_type='FOREIGN KEY';

-- 9. Sanity: existing tables this migration must never touch are unchanged (row count check
--    here; full content-hash comparison done separately via db-before.json/db-after.json).
SELECT '9.row_count_sanity_overrides' AS check_name,
       'INFO' AS status, CONCAT('current_row_count=',COUNT(*)) AS detail
FROM company_template_workflow_overrides;
