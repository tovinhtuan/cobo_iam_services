-- Backfill template.workflow.override.read for all active memberships that don't already have it.
-- Reason: this permission is now auto-granted to every new membership at enrollment time.
-- Existing active members need it added retroactively so they can view "Workflow doanh nghiệp".
--
-- Uses INSERT IGNORE so it is safe to re-run (duplicate key is silently skipped).
-- revoked_at = NULL means the grant is active.

INSERT IGNORE INTO membership_direct_permissions
  (membership_id, company_id, permission_code, granted_by)
SELECT
  m.membership_id,
  m.company_id,
  'template.workflow.override.read',
  'system_migration_0045'
FROM memberships m
WHERE m.membership_status = 'active'
  AND NOT EXISTS (
    SELECT 1
    FROM membership_direct_permissions mdp
    WHERE mdp.membership_id = m.membership_id
      AND mdp.permission_code = 'template.workflow.override.read'
      AND mdp.revoked_at IS NULL
  );
