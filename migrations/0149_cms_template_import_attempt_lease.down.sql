-- 0149 down: drop only the lease columns added by this migration.

SET NAMES utf8mb4;

SET @col_lease_expires_at = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'cms_template_import_attempts'
    AND column_name = 'lease_expires_at'
);
SET @sql = IF(@col_lease_expires_at = 1,
  'ALTER TABLE cms_template_import_attempts DROP COLUMN lease_expires_at',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_confirming_at = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'cms_template_import_attempts'
    AND column_name = 'confirming_at'
);
SET @sql = IF(@col_confirming_at = 1,
  'ALTER TABLE cms_template_import_attempts DROP COLUMN confirming_at',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
