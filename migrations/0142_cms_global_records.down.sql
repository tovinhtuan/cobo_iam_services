-- 0142 down: remove Global CMS Record schema and permissions (DEV/test).
-- Does not delete disclosure_records rows; only drops cms_record_id column.

SET NAMES utf8mb4;

DELETE rp FROM role_permissions rp
INNER JOIN permissions p ON p.permission_id = rp.permission_id
WHERE p.permission_code IN (
  'cms.record.read', 'cms.record.write', 'cms.record.publish', 'cms.record.materialize'
);

DELETE FROM role_default_grant_permissions
WHERE permission_code IN (
  'cms.record.read', 'cms.record.write', 'cms.record.publish', 'cms.record.materialize'
);

DELETE FROM permissions
WHERE permission_code IN (
  'cms.record.read', 'cms.record.write', 'cms.record.publish', 'cms.record.materialize'
);

SET @idx_exists := (
  SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'disclosure_records'
    AND index_name = 'uq_disclosure_records_cms_company'
);
SET @sql := IF(@idx_exists > 0, 'ALTER TABLE disclosure_records DROP INDEX uq_disclosure_records_cms_company', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @idx_exists := (
  SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'disclosure_records'
    AND index_name = 'idx_disclosure_records_cms_record'
);
SET @sql := IF(@idx_exists > 0, 'ALTER TABLE disclosure_records DROP INDEX idx_disclosure_records_cms_record', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @col_exists := (
  SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'disclosure_records'
    AND column_name = 'cms_record_id'
);
SET @sql := IF(@col_exists > 0, 'ALTER TABLE disclosure_records DROP COLUMN cms_record_id', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

DROP TABLE IF EXISTS cms_materialization_items;
DROP TABLE IF EXISTS cms_materialization_runs;
DROP TABLE IF EXISTS cms_global_records;
