SET NAMES utf8mb4;

ALTER TABLE companies
  ADD COLUMN tax_code VARCHAR(64) NULL AFTER company_name,
  ADD COLUMN registration_number VARCHAR(128) NULL AFTER tax_code,
  ADD COLUMN address VARCHAR(512) NULL AFTER registration_number,
  ADD COLUMN phone VARCHAR(64) NULL AFTER address,
  ADD COLUMN contact_email VARCHAR(255) NULL AFTER phone,
  ADD COLUMN representative_name VARCHAR(255) NULL AFTER contact_email,
  ADD UNIQUE KEY uk_companies_tax_code (tax_code);
