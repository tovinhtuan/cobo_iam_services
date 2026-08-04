-- DEV-only seed: commercial company_subscriptions fixtures (Case C).
-- Apply after: 0125_company_subscriptions.up.sql (and companies seed, e.g. seed_dev_identity_authorization).
-- Listed only in migrations/run_dev_migrations.sh — NOT a numbered staging/production migration.
--
-- Fixtures:
--   c_001 → PREMIUM ACTIVE (origin=dev_fixture) — company Premium smoke
--   c_002 → no row → plan:null
--
-- Idempotent: ON DUPLICATE KEY UPDATE on primary key id.
--
-- Cleanup / rollback (manual; do not ship as numbered down on production-like chains):
--   DELETE FROM company_subscriptions WHERE origin = 'dev_fixture';
--   DELETE FROM company_subscriptions WHERE id = 'cps_dev_c001_premium';

SET NAMES utf8mb4;

INSERT INTO company_subscriptions (
  id, company_id, plan_code, status, effective_from, expires_at, origin
) VALUES (
  'cps_dev_c001_premium',
  'c_001',
  'PREMIUM',
  'ACTIVE',
  '2020-01-01 00:00:00',
  NULL,
  'dev_fixture'
)
ON DUPLICATE KEY UPDATE
  plan_code = VALUES(plan_code),
  status = VALUES(status),
  effective_from = VALUES(effective_from),
  expires_at = VALUES(expires_at),
  origin = VALUES(origin);
