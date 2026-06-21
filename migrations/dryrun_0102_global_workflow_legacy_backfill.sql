-- dryrun_0102_global_workflow_legacy_backfill.sql
-- Batch R2 — SELECT-ONLY preview. NO DDL/DML. Run BEFORE 0102.up.sql and inspect every row.
-- Required gate per REMEDIATION_IMPLEMENTATION_PLAN.md Batch R2 #2 — must be reviewed before the
-- real migration runs.

-- 1. Per-candidate-type summary: step count to be created, and whether any step is missing a
--    resolvable role (assignee_role_ids empty/null — these steps will backfill with an empty
--    role; not necessarily wrong if the legacy step legitimately has no assignee, but worth a
--    human look before running for real).
SELECT
  b.type_id,
  JSON_LENGTH(JSON_EXTRACT(tb.config_json, '$.steps')) AS step_count,
  (
    SELECT COUNT(*)
    FROM JSON_TABLE(
      tb.config_json, '$.steps[*]'
      COLUMNS (assignee_role_ids JSON PATH '$.assignee_role_ids')
    ) AS jt
    WHERE jt.assignee_role_ids IS NULL OR JSON_LENGTH(jt.assignee_role_ids) = 0
  ) AS steps_missing_role
FROM (
  SELECT DISTINCT tb.type_id
  FROM disclosure_template_blocks tb
  JOIN disclosure_types dt ON dt.type_id = tb.type_id AND dt.active_version_no = tb.version_no
  WHERE tb.block_key = 'enterprise_workflow'
    AND JSON_LENGTH(COALESCE(JSON_EXTRACT(tb.config_json, '$.steps'), JSON_ARRAY())) > 0
) b
JOIN disclosure_types dt ON dt.type_id = b.type_id
JOIN disclosure_template_blocks tb ON tb.type_id = b.type_id AND tb.version_no = dt.active_version_no
                                    AND tb.block_key = 'enterprise_workflow'
WHERE NOT EXISTS (SELECT 1 FROM global_workflows gw WHERE gw.type_id = b.type_id)
ORDER BY b.type_id;

-- 2. Count of types already excluded because they have a global_workflows row (idempotency proof
--    — these must NOT appear in query 1's output).
SELECT COUNT(*) AS types_already_in_new_system_skipped
FROM (SELECT DISTINCT type_id FROM global_workflows) gw_types;

-- 3. Required before/after evidence count: how many types this migration WILL backfill.
SELECT COUNT(*) AS types_to_be_backfilled
FROM (
  SELECT DISTINCT tb.type_id
  FROM disclosure_template_blocks tb
  JOIN disclosure_types dt ON dt.type_id = tb.type_id AND dt.active_version_no = tb.version_no
  WHERE tb.block_key = 'enterprise_workflow'
    AND JSON_LENGTH(COALESCE(JSON_EXTRACT(tb.config_json, '$.steps'), JSON_ARRAY())) > 0
) b
WHERE NOT EXISTS (SELECT 1 FROM global_workflows gw WHERE gw.type_id = b.type_id);
