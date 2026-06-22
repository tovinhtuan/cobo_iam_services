-- dryrun_0103_override_base_metadata.sql
-- Batch 1 — SELECT-ONLY preview. NO DDL/DML. Run BEFORE 0103.up.sql and inspect every row.
-- Required gate before the real migration runs.
-- NOTE: kept as separate statements (not one combined UNION ALL) — this DEV instance's
-- max_allowed_packet is unusually small (2048 bytes), and a single large multi-UNION statement
-- can exceed it even though the underlying data is tiny. Each statement below is independently
-- small. (Lesson carried forward from migrations/dryrun_0102's working pattern.)

-- 1. Total override rows that will be touched by the backfill (all of them — base_source
--    defaults to 'unknown' for every row by the column ADD itself; this count is just the
--    current total, for before/after comparison).
SELECT COUNT(*) AS total_override_rows FROM company_template_workflow_overrides;

-- 2. Rows where a global_workflow_versions row is provably active (by activated_at) at or
--    before the override's own created_at — these are the rows the backfill will set to
--    base_source='global_workflow' with a real base_version_no.
SELECT
  o.override_id, o.company_id, o.type_id, o.created_at,
  best.best_version_no AS would_be_base_version_no
FROM company_template_workflow_overrides o
JOIN (
  SELECT o2.override_id AS override_id, MAX(v.version_no) AS best_version_no
  FROM company_template_workflow_overrides o2
  JOIN global_workflow_versions v
    ON v.type_id = o2.type_id
   AND v.activated_at IS NOT NULL
   AND v.activated_at <= o2.created_at
  GROUP BY o2.override_id
) best ON best.override_id = o.override_id;

-- 3. Count of rows that will remain 'unknown' (no determinable global_workflow base).
SELECT COUNT(*) AS rows_remaining_unknown
FROM company_template_workflow_overrides o
WHERE o.override_id NOT IN (
  SELECT o2.override_id
  FROM company_template_workflow_overrides o2
  JOIN global_workflow_versions v
    ON v.type_id = o2.type_id
   AND v.activated_at IS NOT NULL
   AND v.activated_at <= o2.created_at
);

-- 4. Sanity: columns do not already exist (expect 0 — if not 0, the migration may have
--    partially run before; investigate before proceeding).
SELECT COUNT(*) AS columns_already_present
FROM information_schema.columns
WHERE table_schema = DATABASE() AND table_name = 'company_template_workflow_overrides'
  AND column_name IN ('base_source','base_workflow_id','base_version_no','base_hash','stale_status','last_rebase_check_at');
