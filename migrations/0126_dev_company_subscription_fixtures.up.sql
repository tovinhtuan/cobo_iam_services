SET NAMES utf8mb4;

-- DEV fixtures for company Premium smoke (no Admin write API required).
-- Premium: c_001 (Company X). Non-Premium: c_002 has no row → plan:null.
-- Cleanup: migrations/0126_dev_company_subscription_fixtures.down.sql
--   OR DELETE FROM company_subscriptions WHERE origin = 'dev_fixture';

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
