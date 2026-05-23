-- 0057: DBA-002 — Workflow override versioning additive patch
-- Existing tables company_template_workflow_overrides and
-- company_template_workflow_override_versions (created in 0020) are used.
-- This migration adds:
--   1. Index on company_template_workflow_overrides for has_workflow lookup.
--   2. Index on company_template_workflow_override_versions for latest-saved-version lookup.
--   3. Index on global_workflows (from 0053) for has_workflow derivation.
-- Ref: portal-template-sprint-ready-execution-board-2026-05-23.md § DBA-002
-- Note: has_workflow = EXISTS(active global_workflow OR approved company override)

SET NAMES utf8mb4;

-- Index: quickly check if a (company, type) pair has any workflow override
SET @idx1 = (
  SELECT COUNT(1) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name   = 'company_template_workflow_overrides'
    AND index_name   = 'idx_ctwo_has_workflow_check'
);
SET @sql1 = IF(
  @idx1 = 0,
  'ALTER TABLE company_template_workflow_overrides ADD INDEX idx_ctwo_has_workflow_check (company_id, type_id, active_version_no)',
  'SELECT 1'
);
PREPARE stmt FROM @sql1; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Index: find the latest approved (saved) version for reset target
SET @idx2 = (
  SELECT COUNT(1) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name   = 'company_template_workflow_override_versions'
    AND index_name   = 'idx_ctwov_latest_approved'
);
SET @sql2 = IF(
  @idx2 = 0,
  'ALTER TABLE company_template_workflow_override_versions ADD INDEX idx_ctwov_latest_approved (override_id, state, version_no DESC)',
  'SELECT 1'
);
PREPARE stmt FROM @sql2; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Index: global workflow active lookup for has_workflow derivation (from 0053 tables)
-- Already has idx_gw_type_active in 0053; add reverse-lookup index for batch reads
SET @idx3 = (
  SELECT COUNT(1) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name   = 'global_workflow_steps'
    AND index_name   = 'idx_gws_has_workflow_check'
);
SET @sql3 = IF(
  @idx3 = 0,
  'ALTER TABLE global_workflow_steps ADD INDEX idx_gws_has_workflow_check (type_id, workflow_id)',
  'SELECT 1'
);
PREPARE stmt FROM @sql3; EXECUTE stmt; DEALLOCATE PREPARE stmt;
