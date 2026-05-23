-- 0053 rollback

SET NAMES utf8mb4;

DROP TABLE IF EXISTS global_workflow_steps;
DROP TABLE IF EXISTS global_workflows;
DROP TABLE IF EXISTS template_display_groups;
DROP TABLE IF EXISTS deadline_rule_catalog;

SET @col_exists = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name   = 'disclosure_types'
    AND column_name  = 'is_mandatory'
);
SET @sql = IF(
  @col_exists = 1,
  'ALTER TABLE disclosure_types DROP COLUMN is_mandatory',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
