-- 0057 rollback — remove additive indexes only

SET NAMES utf8mb4;

SET @idx1 = (
  SELECT COUNT(1) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name   = 'company_template_workflow_overrides'
    AND index_name   = 'idx_ctwo_has_workflow_check'
);
SET @sql1 = IF(@idx1 = 1,
  'ALTER TABLE company_template_workflow_overrides DROP INDEX idx_ctwo_has_workflow_check',
  'SELECT 1');
PREPARE stmt FROM @sql1; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @idx2 = (
  SELECT COUNT(1) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name   = 'company_template_workflow_override_versions'
    AND index_name   = 'idx_ctwov_latest_approved'
);
SET @sql2 = IF(@idx2 = 1,
  'ALTER TABLE company_template_workflow_override_versions DROP INDEX idx_ctwov_latest_approved',
  'SELECT 1');
PREPARE stmt FROM @sql2; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @idx3 = (
  SELECT COUNT(1) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name   = 'global_workflow_steps'
    AND index_name   = 'idx_gws_has_workflow_check'
);
SET @sql3 = IF(@idx3 = 1,
  'ALTER TABLE global_workflow_steps DROP INDEX idx_gws_has_workflow_check',
  'SELECT 1');
PREPARE stmt FROM @sql3; EXECUTE stmt; DEALLOCATE PREPARE stmt;
