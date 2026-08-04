SET NAMES utf8mb4;

-- Rollback DEV company subscription fixtures only.
DELETE FROM company_subscriptions WHERE origin = 'dev_fixture';
DELETE FROM company_subscriptions WHERE id = 'cps_dev_c001_premium';
