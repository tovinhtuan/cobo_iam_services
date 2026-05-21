-- Revert: revoke the backfilled template.workflow.override.read grants added by migration 0045.
-- Only removes grants made by the migration itself (granted_by = 'system_migration_0045').

UPDATE membership_direct_permissions
SET revoked_at = CURRENT_TIMESTAMP, revoked_by = 'system_migration_0045_down'
WHERE permission_code = 'template.workflow.override.read'
  AND granted_by = 'system_migration_0045'
  AND revoked_at IS NULL;
