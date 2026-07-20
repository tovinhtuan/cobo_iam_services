-- 0122 down: drop is_released if present.

SET NAMES utf8mb4;

SET @col_exists := (
  SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'disclosure_type_versions'
    AND column_name = 'is_released'
);
SET @sql := IF(
  @col_exists > 0,
  'ALTER TABLE disclosure_type_versions DROP COLUMN is_released',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
