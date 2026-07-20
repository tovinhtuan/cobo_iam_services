-- 0122: Single open-draft model for CMS template versions.
-- Additive column is_released: released/activated snapshots stay in history;
-- mutable open drafts stay is_released=0 and are overwritten on save.
-- Backward compatible: old code ignores the column; new list filters by it.

SET NAMES utf8mb4;

SET @col_exists := (
  SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'disclosure_type_versions'
    AND column_name = 'is_released'
);
SET @sql := IF(
  @col_exists = 0,
  'ALTER TABLE disclosure_type_versions ADD COLUMN is_released TINYINT(1) NOT NULL DEFAULT 0 AFTER change_note',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

-- Current Portal active versions are released snapshots.
UPDATE disclosure_type_versions v
INNER JOIN disclosure_types t ON t.type_id = v.type_id AND v.version_no = t.active_version_no
SET v.is_released = 1
WHERE t.active_version_no > 0;
