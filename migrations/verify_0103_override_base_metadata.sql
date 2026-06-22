-- verify_0103_override_base_metadata.sql
-- Batch 1 verification (READ-ONLY, SELECT ONLY). Safe on any env: NO DDL/DML.
-- Every row's `status` MUST be 'PASS'.
-- NOTE: kept as separate statements, not one combined UNION ALL — see dryrun file's note on
-- this DEV instance's small max_allowed_packet.

-- 1. all 6 columns exist
SELECT '1.all_columns_exist' AS check_name,
       IF(COUNT(*)=6,'PASS','FAIL') AS status, CONCAT('found=',COUNT(*),'/6') AS detail
FROM information_schema.columns
WHERE table_schema=DATABASE() AND table_name='company_template_workflow_overrides'
  AND column_name IN ('base_source','base_workflow_id','base_version_no','base_hash','stale_status','last_rebase_check_at');

-- 2. stale_status defaults to 'unknown' for every row that wasn't otherwise touched (must be
--    'unknown' for ALL rows in Batch 1 — this migration never sets it to anything else).
SELECT '2.stale_status_always_unknown' AS check_name,
       IF(COUNT(*)=0,'PASS','FAIL') AS status, CONCAT('rows_not_unknown=',COUNT(*)) AS detail
FROM company_template_workflow_overrides WHERE stale_status <> 'unknown';

-- 3. base_hash is NULL for every row (never computed in SQL — see HASH_CONTRACT.md).
SELECT '3.base_hash_always_null' AS check_name,
       IF(COUNT(*)=0,'PASS','FAIL') AS status, CONCAT('rows_with_hash=',COUNT(*)) AS detail
FROM company_template_workflow_overrides WHERE base_hash IS NOT NULL;

-- 4. last_rebase_check_at is NULL for every row (Batch 1 never runs a check).
SELECT '4.last_rebase_check_at_always_null' AS check_name,
       IF(COUNT(*)=0,'PASS','FAIL') AS status, CONCAT('rows_with_check_at=',COUNT(*)) AS detail
FROM company_template_workflow_overrides WHERE last_rebase_check_at IS NOT NULL;

-- 5. every row with base_source='global_workflow' has a non-null base_version_no (internal
--    consistency of the backfill — never one without the other).
SELECT '5.global_workflow_rows_have_version_no' AS check_name,
       IF(COUNT(*)=0,'PASS','FAIL') AS status, CONCAT('inconsistent_rows=',COUNT(*)) AS detail
FROM company_template_workflow_overrides
WHERE base_source='global_workflow' AND base_version_no IS NULL;

-- 6. every row with base_source='unknown' has NULL base_version_no/base_workflow_id (never a
--    partial/guessed value attached to an 'unknown' row).
SELECT '6.unknown_rows_have_no_base_fields' AS check_name,
       IF(COUNT(*)=0,'PASS','FAIL') AS status, CONCAT('inconsistent_rows=',COUNT(*)) AS detail
FROM company_template_workflow_overrides
WHERE base_source='unknown' AND (base_version_no IS NOT NULL OR base_workflow_id IS NOT NULL);

-- 7. base_source only ever takes one of the 3 allowed values.
SELECT '7.base_source_valid_enum' AS check_name,
       IF(COUNT(*)=0,'PASS','FAIL') AS status, CONCAT('invalid_values=',COUNT(*)) AS detail
FROM company_template_workflow_overrides
WHERE base_source NOT IN ('global_workflow','global_template','unknown');

-- 8. existing override content untouched: active_version_no/status row count sanity (full
--    before/after row-by-row comparison is done separately via db-before.json/db-after.json;
--    this is a coarse, fast assertion that the table still has the same row count as before).
SELECT '8.row_count_unchanged_sanity' AS check_name,
       'INFO' AS status, CONCAT('current_row_count=',COUNT(*)) AS detail
FROM company_template_workflow_overrides;
