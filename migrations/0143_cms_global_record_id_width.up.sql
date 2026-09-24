-- 0143: Widen Global CMS Record / materialization ID columns for prefixed UUIDv7
-- (e.g. cms_rec_<uuid> = 44 chars). Additive ALTER only; no data rewrite.

SET NAMES utf8mb4;

-- Drop FKs that reference id/run_id before widening.
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
  MODIFY COLUMN id VARCHAR(64) NOT NULL;

ALTER TABLE cms_materialization_runs
  MODIFY COLUMN run_id VARCHAR(64) NOT NULL,
  MODIFY COLUMN cms_record_id VARCHAR(64) NOT NULL;

ALTER TABLE cms_materialization_items
  MODIFY COLUMN run_id VARCHAR(64) NOT NULL;

-- disclosure_records.cms_record_id may be NULL (legacy); widen when present.
SET @col_len := (
  SELECT CHARACTER_MAXIMUM_LENGTH FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE()
    AND TABLE_NAME = 'disclosure_records'
    AND COLUMN_NAME = 'cms_record_id'
  LIMIT 1
);
SET @sql := IF(
  @col_len IS NOT NULL AND @col_len < 64,
  'ALTER TABLE disclosure_records MODIFY COLUMN cms_record_id VARCHAR(64) NULL DEFAULT NULL',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

ALTER TABLE cms_materialization_runs
  ADD CONSTRAINT fk_cms_mat_runs_record
  FOREIGN KEY (cms_record_id) REFERENCES cms_global_records (id);

ALTER TABLE cms_materialization_items
  ADD CONSTRAINT fk_cms_mat_items_run
  FOREIGN KEY (run_id) REFERENCES cms_materialization_runs (run_id);
