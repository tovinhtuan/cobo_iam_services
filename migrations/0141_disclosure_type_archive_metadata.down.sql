-- 0141 down: drop archive metadata columns (local/dev only before production restore usage).

SET NAMES utf8mb4;

SET @col_exists := (
  SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'disclosure_types'
    AND column_name = 'archive_reason'
);
SET @sql := IF(
  @col_exists = 1,
  'ALTER TABLE disclosure_types DROP COLUMN archive_reason',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists := (
  SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'disclosure_types'
    AND column_name = 'archived_by'
);
SET @sql := IF(
  @col_exists = 1,
  'ALTER TABLE disclosure_types DROP COLUMN archived_by',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists := (
  SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'disclosure_types'
    AND column_name = 'archived_at'
);
SET @sql := IF(
  @col_exists = 1,
  'ALTER TABLE disclosure_types DROP COLUMN archived_at',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists := (
  SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'disclosure_types'
    AND column_name = 'archived_from_version_no'
);
SET @sql := IF(
  @col_exists = 1,
  'ALTER TABLE disclosure_types DROP COLUMN archived_from_version_no',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
