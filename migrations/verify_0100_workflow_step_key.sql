-- verify_0100_workflow_step_key.sql
-- Sprint 0 / Batch 2 — mig-S1 verification (READ-ONLY, SELECT ONLY).
-- Safe on DEV/UAT/PROD: NO DDL, NO DML. Every row's `status` MUST be 'PASS'.
-- Target: MySQL 8.0, run against the application schema (uses DATABASE()).

-- 1. step_key column exists
SELECT '1.step_key_column_exists' AS check_name,
       IF(COUNT(*) = 1, 'PASS', 'FAIL') AS status,
       CONCAT('found=', COUNT(*)) AS detail
FROM information_schema.columns
WHERE table_schema = DATABASE() AND table_name = 'global_workflow_steps' AND column_name = 'step_key'

UNION ALL
-- 2. step_type column exists
SELECT '2.step_type_column_exists',
       IF(COUNT(*) = 1, 'PASS', 'FAIL'),
       CONCAT('found=', COUNT(*))
FROM information_schema.columns
WHERE table_schema = DATABASE() AND table_name = 'global_workflow_steps' AND column_name = 'step_type'

UNION ALL
-- 3. step_id column still exists (not dropped/rewritten)
SELECT '3.step_id_column_still_exists',
       IF(COUNT(*) = 1, 'PASS', 'FAIL'),
       CONCAT('found=', COUNT(*))
FROM information_schema.columns
WHERE table_schema = DATABASE() AND table_name = 'global_workflow_steps' AND column_name = 'step_id'

UNION ALL
-- 4. unique index (workflow_id, step_key) present
SELECT '4.unique_index_present',
       IF(COUNT(*) >= 1, 'PASS', 'FAIL'),
       CONCAT('found=', COUNT(*))
FROM information_schema.statistics
WHERE table_schema = DATABASE() AND table_name = 'global_workflow_steps'
  AND index_name = 'uq_gws_workflow_stepkey'

UNION ALL
-- 5. no null/empty step_key
SELECT '5.no_empty_step_key',
       IF(COUNT(*) = 0, 'PASS', 'FAIL'),
       CONCAT('empty_or_null=', COUNT(*))
FROM global_workflow_steps
WHERE step_key IS NULL OR step_key = ''

UNION ALL
-- 6. step_key unique per workflow (no duplicate workflow_id+step_key)
SELECT '6.step_key_unique_per_workflow',
       IF(COUNT(*) = 0, 'PASS', 'FAIL'),
       CONCAT('duplicate_pairs=', COUNT(*))
FROM (
  SELECT workflow_id, step_key
  FROM global_workflow_steps
  GROUP BY workflow_id, step_key
  HAVING COUNT(*) > 1
) dup

UNION ALL
-- 7. every step still has a step_id (no row lost identity)
SELECT '7.every_step_has_step_id',
       IF(COUNT(*) = 0, 'PASS', 'FAIL'),
       CONCAT('rows_missing_step_id=', COUNT(*))
FROM global_workflow_steps
WHERE step_id IS NULL OR step_id = ''

UNION ALL
-- 8. context row count (informational — compare to pre-migration baseline)
SELECT '8.step_row_count_info',
       'PASS',
       CONCAT('total_steps=', COUNT(*))
FROM global_workflow_steps

ORDER BY check_name;
