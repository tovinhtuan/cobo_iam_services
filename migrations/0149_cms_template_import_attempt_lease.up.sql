-- 0149: confirm lease columns for import attempts. Expand-only. No raw payload or token.
-- Does not alter 0144-0148 objects other than adding nullable columns on cms_template_import_attempts.

SET NAMES utf8mb4;

SET @col_confirming_at = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'cms_template_import_attempts'
    AND column_name = 'confirming_at'
);
SET @sql = IF(@col_confirming_at = 0,
  'ALTER TABLE cms_template_import_attempts ADD COLUMN confirming_at DATETIME(3) NULL',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_lease_expires_at = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'cms_template_import_attempts'
    AND column_name = 'lease_expires_at'
);
SET @sql = IF(@col_lease_expires_at = 0,
  'ALTER TABLE cms_template_import_attempts ADD COLUMN lease_expires_at DATETIME(3) NULL',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
