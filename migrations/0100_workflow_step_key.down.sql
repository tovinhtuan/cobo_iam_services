-- 0100 DOWN: reverse mig-S1 (step_key + step_type on global_workflow_steps).
--
-- Data safety: drops only the columns/index added by 0100.up. step_id and all other data remain.
-- Guarded via information_schema so it is safe to run even if partially applied.

SET NAMES utf8mb4;

-- ─── 1. drop unique (workflow_id, step_key) ───
SET @idx_uq = (
  SELECT COUNT(1) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name   = 'global_workflow_steps'
    AND index_name   = 'uq_gws_workflow_stepkey'
);
SET @sql = IF(@idx_uq > 0,
  'ALTER TABLE global_workflow_steps DROP INDEX uq_gws_workflow_stepkey',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ─── 2. drop step_type ───
SET @col_step_type = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name   = 'global_workflow_steps'
    AND column_name  = 'step_type'
);
SET @sql = IF(@col_step_type > 0,
  'ALTER TABLE global_workflow_steps DROP COLUMN step_type',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- ─── 3. drop step_key ───
SET @col_step_key = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name   = 'global_workflow_steps'
    AND column_name  = 'step_key'
);
SET @sql = IF(@col_step_key > 0,
  'ALTER TABLE global_workflow_steps DROP COLUMN step_key',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

DELETE FROM schema_migrations WHERE file_name = '0100_workflow_step_key.up.sql';
