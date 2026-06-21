-- 0100: mig-S1 — Add immutable step_key (+ step_type) to global_workflow_steps.
--
-- Baseline: WORKFLOW_CONFIGURATION_BASELINE_V1 / Phase 13 STEP_KEY_SPECIFICATION + DB contract.
-- step_key is the stable, server-minted, immutable identity used by the override/sync model.
--
-- Data safety: ADDITIVE only. No column dropped, no row deleted, no destructive rewrite.
--   - step_key backfilled from the existing stable identity (step_id), which is the PK (unique, non-null).
--   - step_type added nullable (no backfill needed; wired in later sprints).
--   - Unique (workflow_id, step_key) added AFTER backfill.
-- Idempotency: guarded via information_schema checks (MySQL 8.0 has no ADD COLUMN IF NOT EXISTS).
-- Reversible: see 0100_workflow_step_key.down.sql.

SET NAMES utf8mb4;

-- ─── 1. add step_key (NOT NULL DEFAULT '' so existing rows are valid pre-backfill) ───
SET @col_step_key = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name   = 'global_workflow_steps'
    AND column_name  = 'step_key'
);
SET @sql = IF(@col_step_key = 0,
  "ALTER TABLE global_workflow_steps ADD COLUMN step_key VARCHAR(64) NOT NULL DEFAULT '' AFTER step_id",
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ─── 2. add step_type (nullable; wired in later sprints) ───
SET @col_step_type = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name   = 'global_workflow_steps'
    AND column_name  = 'step_type'
);
SET @sql = IF(@col_step_type = 0,
  'ALTER TABLE global_workflow_steps ADD COLUMN step_type VARCHAR(32) NULL AFTER stage',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ─── 3. backfill step_key from existing stable identity (step_id is PK: unique, non-null) ───
UPDATE global_workflow_steps
SET step_key = step_id
WHERE step_key IS NULL OR step_key = '';

-- ─── 4. add unique (workflow_id, step_key) AFTER backfill ───
SET @idx_uq = (
  SELECT COUNT(1) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name   = 'global_workflow_steps'
    AND index_name   = 'uq_gws_workflow_stepkey'
);
SET @sql = IF(@idx_uq = 0,
  'ALTER TABLE global_workflow_steps ADD UNIQUE KEY uq_gws_workflow_stepkey (workflow_id, step_key)',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Record in migration tracker (matches 0091 self-recording pattern).
INSERT INTO schema_migrations (file_name) VALUES ('0100_workflow_step_key.up.sql')
  ON DUPLICATE KEY UPDATE executed_at = CURRENT_TIMESTAMP;
