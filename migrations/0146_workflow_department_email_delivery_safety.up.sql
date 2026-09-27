-- 0146: expand reminder_dispatch_resolutions for atomic claim/lease/retry.
-- Schema only. Does not enable workflow-department flags and does not send email.
-- MySQL 8.0 has no ADD COLUMN IF NOT EXISTS; each column is guarded.
-- Does not modify 0144.

SET NAMES utf8mb4;

SET @col_lease_id = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'reminder_dispatch_resolutions'
    AND column_name = 'lease_id'
);
SET @sql = IF(@col_lease_id = 0,
  'ALTER TABLE reminder_dispatch_resolutions ADD COLUMN lease_id CHAR(36) NULL',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_lease_until = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'reminder_dispatch_resolutions'
    AND column_name = 'lease_until'
);
SET @sql = IF(@col_lease_until = 0,
  'ALTER TABLE reminder_dispatch_resolutions ADD COLUMN lease_until DATETIME(3) NULL',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_updated_at = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'reminder_dispatch_resolutions'
    AND column_name = 'updated_at'
);
SET @sql = IF(@col_updated_at = 0,
  'ALTER TABLE reminder_dispatch_resolutions ADD COLUMN updated_at DATETIME(3) NULL',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_attempt = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'reminder_dispatch_resolutions'
    AND column_name = 'attempt_count'
);
SET @sql = IF(@col_attempt = 0,
  'ALTER TABLE reminder_dispatch_resolutions ADD COLUMN attempt_count INT NOT NULL DEFAULT 0',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_next = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'reminder_dispatch_resolutions'
    AND column_name = 'next_retry_at'
);
SET @sql = IF(@col_next = 0,
  'ALTER TABLE reminder_dispatch_resolutions ADD COLUMN next_retry_at DATETIME(3) NULL',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_err = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'reminder_dispatch_resolutions'
    AND column_name = 'last_error_code'
);
SET @sql = IF(@col_err = 0,
  'ALTER TABLE reminder_dispatch_resolutions ADD COLUMN last_error_code VARCHAR(64) NULL',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @idx_claim = (
  SELECT COUNT(1) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'reminder_dispatch_resolutions'
    AND index_name = 'idx_reminder_dispatch_claim'
);
SET @sql = IF(@idx_claim = 0,
  'ALTER TABLE reminder_dispatch_resolutions ADD KEY idx_reminder_dispatch_claim (send_status, next_retry_at, lease_until)',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
