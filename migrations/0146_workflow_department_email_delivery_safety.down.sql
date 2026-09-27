-- 0146 down: drop only columns and the claim index this migration added.
-- Does not drop reminder_dispatch_resolutions and does not delete resolution rows.
-- PRODUCTION_ROLLBACK=forward_fix_or_flag_off
-- DEV_DOWN=DROP_OWNED_COLUMNS_ONLY

SET NAMES utf8mb4;

SET @idx_claim = (
  SELECT COUNT(1) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'reminder_dispatch_resolutions'
    AND index_name = 'idx_reminder_dispatch_claim'
);
SET @sql = IF(@idx_claim > 0,
  'ALTER TABLE reminder_dispatch_resolutions DROP INDEX idx_reminder_dispatch_claim',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'reminder_dispatch_resolutions' AND column_name = 'last_error_code'
);
SET @sql = IF(@col > 0, 'ALTER TABLE reminder_dispatch_resolutions DROP COLUMN last_error_code', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'reminder_dispatch_resolutions' AND column_name = 'next_retry_at'
);
SET @sql = IF(@col > 0, 'ALTER TABLE reminder_dispatch_resolutions DROP COLUMN next_retry_at', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'reminder_dispatch_resolutions' AND column_name = 'attempt_count'
);
SET @sql = IF(@col > 0, 'ALTER TABLE reminder_dispatch_resolutions DROP COLUMN attempt_count', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'reminder_dispatch_resolutions' AND column_name = 'updated_at'
);
SET @sql = IF(@col > 0, 'ALTER TABLE reminder_dispatch_resolutions DROP COLUMN updated_at', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'reminder_dispatch_resolutions' AND column_name = 'lease_until'
);
SET @sql = IF(@col > 0, 'ALTER TABLE reminder_dispatch_resolutions DROP COLUMN lease_until', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'reminder_dispatch_resolutions' AND column_name = 'lease_id'
);
SET @sql = IF(@col > 0, 'ALTER TABLE reminder_dispatch_resolutions DROP COLUMN lease_id', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
