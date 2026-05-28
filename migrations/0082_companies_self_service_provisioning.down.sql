SET NAMES utf8mb4;

ALTER TABLE companies
  DROP KEY idx_companies_founder_provisioning,
  DROP COLUMN provisioning_source,
  DROP COLUMN founder_user_id;
