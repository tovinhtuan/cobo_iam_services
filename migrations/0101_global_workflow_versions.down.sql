-- 0101 DOWN: reverse mig-S2 (global workflow versioning).
-- Drops only what 0101.up created. No tenant table touched. No step/workflow data deleted.

SET NAMES utf8mb4;

-- drop pointer columns (guarded)
SET @c1 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='global_workflows' AND column_name='active_version_no');
SET @sql = IF(@c1>0, 'ALTER TABLE global_workflows DROP COLUMN active_version_no', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c2 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='global_workflows' AND column_name='published_version_no');
SET @sql = IF(@c2>0, 'ALTER TABLE global_workflows DROP COLUMN published_version_no', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

DROP TABLE IF EXISTS global_workflow_versions;

DELETE FROM schema_migrations WHERE file_name = '0101_global_workflow_versions.up.sql';
