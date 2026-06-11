SET NAMES utf8mb4;

ALTER TABLE companies
  DROP COLUMN is_listed,
  DROP COLUMN is_large_public,
  DROP COLUMN is_non_large_public,
  DROP COLUMN has_subsidiaries,
  DROP COLUMN has_subordinate_accounting_units,
  DROP COLUMN business_sector;
