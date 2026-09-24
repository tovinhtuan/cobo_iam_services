-- 0143 down: narrow IDs back to VARCHAR(36). Only safe when no prefixed IDs exist.

SET NAMES utf8mb4;

SET @fk_runs := (
  SELECT CONSTRAINT_NAME FROM information_schema.TABLE_CONSTRAINTS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'cms_materialization_runs'
    AND CONSTRAINT_TYPE = 'FOREIGN KEY'
    AND CONSTRAINT_NAME = 'fk_cms_mat_runs_record'
  LIMIT 1
);
SET @sql := IF(
  @fk_runs IS NOT NULL,
  'ALTER TABLE cms_materialization_runs DROP FOREIGN KEY fk_cms_mat_runs_record',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @fk_items := (
  SELECT CONSTRAINT_NAME FROM information_schema.TABLE_CONSTRAINTS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'cms_materialization_items'
    AND CONSTRAINT_TYPE = 'FOREIGN KEY'
    AND CONSTRAINT_NAME = 'fk_cms_mat_items_run'
  LIMIT 1
);
SET @sql := IF(
  @fk_items IS NOT NULL,
  'ALTER TABLE cms_materialization_items DROP FOREIGN KEY fk_cms_mat_items_run',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

ALTER TABLE cms_global_records
  MODIFY COLUMN id VARCHAR(36) NOT NULL;

ALTER TABLE cms_materialization_runs
  MODIFY COLUMN run_id VARCHAR(36) NOT NULL,
  MODIFY COLUMN cms_record_id VARCHAR(36) NOT NULL;

ALTER TABLE cms_materialization_items
  MODIFY COLUMN run_id VARCHAR(36) NOT NULL;

SET @col_exists := (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'disclosure_records'
    AND COLUMN_NAME = 'cms_record_id'
);
SET @sql := IF(
  @col_exists > 0,
  'ALTER TABLE disclosure_records MODIFY COLUMN cms_record_id VARCHAR(36) NULL DEFAULT NULL',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

ALTER TABLE cms_materialization_runs
  ADD CONSTRAINT fk_cms_mat_runs_record
  FOREIGN KEY (cms_record_id) REFERENCES cms_global_records (id);

ALTER TABLE cms_materialization_items
  ADD CONSTRAINT fk_cms_mat_items_run
  FOREIGN KEY (run_id) REFERENCES cms_materialization_runs (run_id);
