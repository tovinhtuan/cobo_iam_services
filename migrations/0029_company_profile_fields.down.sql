SET NAMES utf8mb4;

ALTER TABLE companies
  DROP INDEX uk_companies_tax_code,
  DROP COLUMN representative_name,
  DROP COLUMN contact_email,
  DROP COLUMN phone,
  DROP COLUMN address,
  DROP COLUMN registration_number,
  DROP COLUMN tax_code;
