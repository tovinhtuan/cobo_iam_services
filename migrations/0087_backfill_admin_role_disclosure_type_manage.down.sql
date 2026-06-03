-- 0087 DOWN: Remove disclosure_type.manage grants added by 0087 backfill.
-- SAFETY: Only removes grants for companies created BEFORE the code fix deploy.
-- Replace {DEPLOY_TIMESTAMP} with the actual BE hardening deploy datetime before running.
-- Run dry-run (SELECT COUNT) first. DBA review required.

-- DRY-RUN (run this first, expect count of historical companies only):
-- SELECT COUNT(*) FROM role_permissions rp
-- JOIN roles r ON r.role_id = rp.role_id
-- JOIN permissions p ON p.permission_id = rp.permission_id
-- WHERE r.role_code = 'admin_doanh_nghiep'
--   AND r.company_id IS NOT NULL
--   AND p.permission_code = 'disclosure_type.manage'
--   AND r.created_at < '{DEPLOY_TIMESTAMP}';

DELETE rp FROM role_permissions rp
JOIN roles r ON r.role_id = rp.role_id
JOIN permissions p ON p.permission_id = rp.permission_id
WHERE r.role_code = 'admin_doanh_nghiep'
  AND r.company_id IS NOT NULL
  AND p.permission_code = 'disclosure_type.manage'
  AND r.created_at < '{DEPLOY_TIMESTAMP}';

-- POST-ROLLBACK VERIFY (should match pre-rollback dry-run count → 0 remaining):
-- SELECT COUNT(*) FROM role_permissions rp
-- JOIN roles r ON r.role_id = rp.role_id
-- JOIN permissions p ON p.permission_id = rp.permission_id
-- WHERE r.role_code = 'admin_doanh_nghiep'
--   AND r.company_id IS NOT NULL
--   AND p.permission_code = 'disclosure_type.manage'
--   AND r.created_at < '{DEPLOY_TIMESTAMP}';
