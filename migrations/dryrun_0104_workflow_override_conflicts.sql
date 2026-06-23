-- dryrun_0104_workflow_override_conflicts.sql
-- Batch 4 — SELECT-ONLY preview. NO DDL/DML. Run BEFORE 0104.up.sql.
-- Kept as separate statements (not one combined UNION ALL) — this DEV instance's
-- max_allowed_packet is unusually small (2048 bytes); each statement below is independently small.

-- 1. Sanity: the table does not already exist (expect 0 — if not 0, investigate before proceeding).
SELECT COUNT(*) AS table_already_exists
FROM information_schema.tables
WHERE table_schema = DATABASE() AND table_name = 'workflow_override_conflicts';

-- 2. Sanity: this migration is not already recorded as applied.
SELECT COUNT(*) AS already_recorded
FROM schema_migrations WHERE file_name = '0104_workflow_override_conflicts.up.sql';

-- 3. Row counts of every table this migration must NOT touch (before/after comparison baseline).
SELECT 'company_template_workflow_overrides' AS tbl, COUNT(*) AS row_count FROM company_template_workflow_overrides;

-- 4.
SELECT 'company_template_workflow_override_versions' AS tbl, COUNT(*) AS row_count FROM company_template_workflow_override_versions;

-- 5.
SELECT 'global_workflows' AS tbl, COUNT(*) AS row_count FROM global_workflows;

-- 6.
SELECT 'global_workflow_versions' AS tbl, COUNT(*) AS row_count FROM global_workflow_versions;

-- 7.
SELECT 'global_workflow_steps' AS tbl, COUNT(*) AS row_count FROM global_workflow_steps;

-- 8.
SELECT 'workflow_instances' AS tbl, COUNT(*) AS row_count FROM workflow_instances;

-- 9.
SELECT 'workflow_tasks' AS tbl, COUNT(*) AS row_count FROM workflow_tasks;

-- 10.
SELECT 'disclosure_template_blocks' AS tbl, COUNT(*) AS row_count FROM disclosure_template_blocks;
