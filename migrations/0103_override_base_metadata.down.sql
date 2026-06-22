-- 0103 DOWN: reverse Sprint 3 / Batch 1 (workflow override base metadata).
-- Drops only what 0103.up created. No tenant table behavior touched. No override snapshot
-- content (workflow_json), active_version_no, or status ever modified by this migration in
-- either direction.

SET NAMES utf8mb4;

SET @c6 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_overrides' AND column_name='last_rebase_check_at');
SET @sql = IF(@c6>0, 'ALTER TABLE company_template_workflow_overrides DROP COLUMN last_rebase_check_at', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c5 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_overrides' AND column_name='stale_status');
SET @sql = IF(@c5>0, 'ALTER TABLE company_template_workflow_overrides DROP COLUMN stale_status', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c4 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_overrides' AND column_name='base_hash');
SET @sql = IF(@c4>0, 'ALTER TABLE company_template_workflow_overrides DROP COLUMN base_hash', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c3 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_overrides' AND column_name='base_version_no');
SET @sql = IF(@c3>0, 'ALTER TABLE company_template_workflow_overrides DROP COLUMN base_version_no', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c2 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_overrides' AND column_name='base_workflow_id');
SET @sql = IF(@c2>0, 'ALTER TABLE company_template_workflow_overrides DROP COLUMN base_workflow_id', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c1 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_overrides' AND column_name='base_source');
SET @sql = IF(@c1>0, 'ALTER TABLE company_template_workflow_overrides DROP COLUMN base_source', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

DELETE FROM schema_migrations WHERE file_name = '0103_override_base_metadata.up.sql';
