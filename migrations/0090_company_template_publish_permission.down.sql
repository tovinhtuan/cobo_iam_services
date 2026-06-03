-- 0090 DOWN: Remove disclosure_type.publish grants added by 0090 backfill.
-- SAFETY: Only removes grants for companies created BEFORE this migration was deployed.
-- Replace {DEPLOY_TIMESTAMP} with the actual deploy datetime before running.
-- DBA review required.

-- DRY-RUN (run this first):
-- SELECT COUNT(*) FROM role_permissions rp
-- JOIN roles r ON r.role_id = rp.role_id
-- JOIN permissions p ON p.permission_id = rp.permission_id
-- WHERE r.role_code = 'admin_doanh_nghiep'
--   AND r.company_id IS NOT NULL
--   AND p.permission_code = 'disclosure_type.publish'
--   AND r.created_at < '{DEPLOY_TIMESTAMP}';

DELETE rp FROM role_permissions rp
JOIN roles r ON r.role_id = rp.role_id
JOIN permissions p ON p.permission_id = rp.permission_id
WHERE r.role_code = 'admin_doanh_nghiep'
  AND r.company_id IS NOT NULL
  AND p.permission_code = 'disclosure_type.publish'
  AND r.created_at < '{DEPLOY_TIMESTAMP}';

DELETE FROM permissions WHERE permission_code = 'disclosure_type.publish';
