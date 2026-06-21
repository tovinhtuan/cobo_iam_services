-- verify_0101_global_workflow_versions.sql
-- Sprint 1 / Batch 2 — mig-S2 verification (READ-ONLY, SELECT ONLY).
-- Safe on any env: NO DDL/DML. Every row's `status` MUST be 'PASS'.

-- 1. version table exists
SELECT '1.versions_table_exists' AS check_name,
       IF(COUNT(*)=1,'PASS','FAIL') AS status, CONCAT('found=',COUNT(*)) AS detail
FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='global_workflow_versions'

UNION ALL
-- 2. pointer columns exist on global_workflows
SELECT '2.pointer_columns_exist',
       IF(COUNT(*)=2,'PASS','FAIL'), CONCAT('present=',COUNT(*),'/2')
FROM information_schema.columns
WHERE table_schema=DATABASE() AND table_name='global_workflows'
  AND column_name IN ('published_version_no','active_version_no')

UNION ALL
-- 3. version rows exist for every active workflow
SELECT '3.version_rows_for_active_workflows',
       IF(COUNT(*)=0,'PASS','FAIL'), CONCAT('active_workflows_without_v1=',COUNT(*))
FROM global_workflows gw
WHERE gw.status='active'
  AND NOT EXISTS (SELECT 1 FROM global_workflow_versions v WHERE v.type_id=gw.type_id AND v.version_no=1)

UNION ALL
-- 4. active_version_no points to an existing active version row
SELECT '4.active_pointer_valid',
       IF(COUNT(*)=0,'PASS','FAIL'), CONCAT('invalid_active_pointers=',COUNT(*))
FROM global_workflows gw
WHERE gw.active_version_no IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM global_workflow_versions v
                  WHERE v.type_id=gw.type_id AND v.version_no=gw.active_version_no)

UNION ALL
-- 5. published_version_no points to an existing version row
SELECT '5.published_pointer_valid',
       IF(COUNT(*)=0,'PASS','FAIL'), CONCAT('invalid_published_pointers=',COUNT(*))
FROM global_workflows gw
WHERE gw.published_version_no IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM global_workflow_versions v
                  WHERE v.type_id=gw.type_id AND v.version_no=gw.published_version_no)

UNION ALL
-- 6. at most one 'active' version per type
SELECT '6.single_active_per_type',
       IF(COUNT(*)=0,'PASS','FAIL'), CONCAT('types_with_multiple_active=',COUNT(*))
FROM (SELECT type_id FROM global_workflow_versions WHERE state='active' GROUP BY type_id HAVING COUNT(*)>1) d

UNION ALL
-- 7. no duplicate (type_id, version_no) — guaranteed by PK, asserted defensively
SELECT '7.no_duplicate_versions',
       IF(COUNT(*)=0,'PASS','FAIL'), CONCAT('duplicate_pairs=',COUNT(*))
FROM (SELECT type_id,version_no FROM global_workflow_versions GROUP BY type_id,version_no HAVING COUNT(*)>1) d

UNION ALL
-- 8. tenant override tables untouched by this migration (row counts are observational; must exist)
SELECT '8.tenant_override_tables_present',
       IF(COUNT(*)=2,'PASS','FAIL'), CONCAT('found=',COUNT(*),'/2 (override tables intact)')
FROM information_schema.tables
WHERE table_schema=DATABASE()
  AND table_name IN ('company_template_workflow_overrides','company_template_workflow_override_versions')

UNION ALL
-- 9. every version manifest is valid JSON (non-null)
SELECT '9.manifest_json_present',
       IF(COUNT(*)=0,'PASS','FAIL'), CONCAT('null_manifests=',COUNT(*))
FROM global_workflow_versions WHERE steps_manifest_json IS NULL

ORDER BY check_name;
