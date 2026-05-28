SET NAMES utf8mb4;

ALTER TABLE companies
  ADD COLUMN founder_user_id VARCHAR(36) NULL AFTER company_name,
  ADD COLUMN provisioning_source VARCHAR(32) NULL AFTER founder_user_id,
  ADD KEY idx_companies_founder_provisioning (founder_user_id, provisioning_source);
