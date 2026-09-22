-- 0141: CMS global template archive lifecycle metadata (Phase 1 soft archive/restore).
-- Additive nullable columns on disclosure_types. No version backfill (legacy archived keep NULL).
-- DEV/test down only; production rollback = forward-fix after restore is live.

SET NAMES utf8mb4;

SET @col_exists := (
  SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'disclosure_types'
    AND column_name = 'archived_from_version_no'
);
SET @sql := IF(
  @col_exists = 0,
  'ALTER TABLE disclosure_types ADD COLUMN archived_from_version_no INT NULL DEFAULT NULL AFTER active_version_no',
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
  @col_exists = 0,
  'ALTER TABLE disclosure_types ADD COLUMN archived_at DATETIME(3) NULL DEFAULT NULL AFTER archived_from_version_no',
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
  @col_exists = 0,
  'ALTER TABLE disclosure_types ADD COLUMN archived_by VARCHAR(64) NULL DEFAULT NULL AFTER archived_at',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists := (
  SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'disclosure_types'
    AND column_name = 'archive_reason'
);
SET @sql := IF(
  @col_exists = 0,
  'ALTER TABLE disclosure_types ADD COLUMN archive_reason VARCHAR(1024) NULL DEFAULT NULL AFTER archived_by',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
