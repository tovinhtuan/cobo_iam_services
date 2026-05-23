-- 0056 rollback

SET NAMES utf8mb4;

SET @idx_exists = (
  SELECT COUNT(1) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name   = 'disclosure_types'
    AND index_name   = 'idx_dt_company_review_status'
);
SET @sql = IF(
  @idx_exists = 1,
  'ALTER TABLE disclosure_types DROP INDEX idx_dt_company_review_status',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name   = 'disclosure_types'
    AND column_name  = 'review_status'
);
SET @sql = IF(
  @col_exists = 1,
  'ALTER TABLE disclosure_types DROP COLUMN review_status',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
