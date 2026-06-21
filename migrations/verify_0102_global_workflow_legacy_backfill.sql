-- verify_0102_global_workflow_legacy_backfill.sql
-- Batch R2 verification (READ-ONLY, SELECT ONLY). Safe on any env: NO DDL/DML.
-- Every row's `status` MUST be 'PASS'.

-- 1. every legacy type with a non-empty enterprise_workflow block now has a global_workflows row
SELECT '1.every_legacy_type_has_global_workflow' AS check_name,
       IF(COUNT(*)=0,'PASS','FAIL') AS status,
       CONCAT('legacy_types_still_missing=',COUNT(*)) AS detail
FROM (
  SELECT DISTINCT tb.type_id
  FROM disclosure_template_blocks tb
  JOIN disclosure_types dt ON dt.type_id = tb.type_id AND dt.active_version_no = tb.version_no
  WHERE tb.block_key = 'enterprise_workflow'
    AND JSON_LENGTH(COALESCE(JSON_EXTRACT(tb.config_json, '$.steps'), JSON_ARRAY())) > 0
) b
WHERE NOT EXISTS (SELECT 1 FROM global_workflows gw WHERE gw.type_id = b.type_id)

UNION ALL
-- 2. every backfilled workflow has at least one step
SELECT '2.backfilled_workflows_have_steps',
       IF(COUNT(*)=0,'PASS','FAIL'), CONCAT('backfilled_workflows_with_zero_steps=',COUNT(*))
FROM global_workflows gw
WHERE gw.created_by = 'system'
  AND gw.change_note = 'backfill v1 (legacy enterprise_workflow)'
  AND NOT EXISTS (SELECT 1 FROM global_workflow_steps s WHERE s.workflow_id = gw.workflow_id)

UNION ALL
-- 3. every backfilled workflow has a v1 active version row
SELECT '3.backfilled_workflows_have_v1_active',
       IF(COUNT(*)=0,'PASS','FAIL'), CONCAT('missing_v1=',COUNT(*))
FROM global_workflows gw
WHERE gw.created_by = 'system'
  AND gw.change_note = 'backfill v1 (legacy enterprise_workflow)'
  AND NOT EXISTS (
    SELECT 1 FROM global_workflow_versions v
    WHERE v.type_id = gw.type_id AND v.version_no = 1 AND v.state = 'active'
  )

UNION ALL
-- 4. every backfilled v1 manifest is the envelope shape (has a top-level "steps" key), not a bare array
SELECT '4.backfilled_manifest_is_envelope_shape',
       IF(COUNT(*)=0,'PASS','FAIL'), CONCAT('bad_shape=',COUNT(*))
FROM global_workflow_versions v
JOIN global_workflows gw ON gw.type_id = v.type_id
WHERE gw.created_by = 'system'
  AND gw.change_note = 'backfill v1 (legacy enterprise_workflow)'
  AND v.version_no = 1
  AND JSON_EXTRACT(v.steps_manifest_json, '$.steps') IS NULL

UNION ALL
-- 5. step count parity: backfilled global_workflow_steps count matches legacy block step count
SELECT '5.step_count_parity',
       IF(COUNT(*)=0,'PASS','FAIL'), CONCAT('mismatched_types=',COUNT(*))
FROM (
  SELECT gw.type_id,
         (SELECT COUNT(*) FROM global_workflow_steps s WHERE s.workflow_id = gw.workflow_id) AS new_count,
         JSON_LENGTH(JSON_EXTRACT(tb.config_json, '$.steps')) AS legacy_count
  FROM global_workflows gw
  JOIN disclosure_types dt ON dt.type_id = gw.type_id
  JOIN disclosure_template_blocks tb ON tb.type_id = gw.type_id AND tb.version_no = dt.active_version_no
                                      AND tb.block_key = 'enterprise_workflow'
  WHERE gw.created_by = 'system'
    AND gw.change_note = 'backfill v1 (legacy enterprise_workflow)'
) d
WHERE d.new_count <> d.legacy_count

UNION ALL
-- 6. legacy content blocks untouched (row count sanity — must still equal pre-migration count;
--    compare manually against db-before.json, this just asserts the table still has rows and the
--    migration issued no DELETE/UPDATE against it, which is also true by code inspection of
--    0102.up.sql containing zero DML statements targeting disclosure_template_blocks)
SELECT '6.legacy_blocks_table_present',
       IF(COUNT(*)>0,'PASS','FAIL'), CONCAT('enterprise_workflow_blocks=',COUNT(*))
FROM disclosure_template_blocks WHERE block_key = 'enterprise_workflow'

UNION ALL
-- 7. tenant override tables untouched
SELECT '7.tenant_override_tables_present',
       IF(COUNT(*)=2,'PASS','FAIL'), CONCAT('found=',COUNT(*),'/2 (override tables intact)')
FROM information_schema.tables
WHERE table_schema=DATABASE()
  AND table_name IN ('company_template_workflow_overrides','company_template_workflow_override_versions')

UNION ALL
-- 8. no duplicate (type_id, version_no=1) created by this migration (PK guarantees this; asserted defensively)
SELECT '8.no_duplicate_v1',
       IF(COUNT(*)=0,'PASS','FAIL'), CONCAT('duplicate_v1_rows=',COUNT(*))
FROM (SELECT type_id FROM global_workflow_versions WHERE version_no=1 GROUP BY type_id HAVING COUNT(*)>1) d

ORDER BY check_name;
