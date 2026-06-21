-- 0102 DOWN: reverse mig-R2 (legacy enterprise_workflow backfill).
-- Deletes ONLY rows this migration created, identified by the unique tag
-- created_by='system' AND change_note='backfill v1 (legacy enterprise_workflow)'. Never touches
-- a type's global_workflows row if it pre-existed this migration (different created_by/change_note),
-- and never touches disclosure_template_blocks (the legacy source was read-only throughout).

SET NAMES utf8mb4;

DELETE v FROM global_workflow_versions v
JOIN global_workflows gw
  ON gw.type_id = v.type_id
WHERE gw.created_by = 'system'
  AND gw.change_note = 'backfill v1 (legacy enterprise_workflow)'
  AND v.version_no = 1;

DELETE s FROM global_workflow_steps s
JOIN global_workflows gw
  ON gw.workflow_id = s.workflow_id
WHERE gw.created_by = 'system'
  AND gw.change_note = 'backfill v1 (legacy enterprise_workflow)';

DELETE FROM global_workflows
WHERE created_by = 'system'
  AND change_note = 'backfill v1 (legacy enterprise_workflow)';

DELETE FROM schema_migrations WHERE file_name = '0102_global_workflow_legacy_backfill.up.sql';
