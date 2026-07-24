SET NAMES utf8mb4;

-- Multi business sectors for company applicability (additive; keep legacy business_sector).
ALTER TABLE companies
  ADD COLUMN business_sectors JSON NULL
  COMMENT 'JSON array of commercial|service|manufacturing';

-- Backfill from legacy single column.
UPDATE companies
SET business_sectors = JSON_ARRAY(business_sector)
WHERE business_sectors IS NULL
  AND business_sector IS NOT NULL
  AND TRIM(business_sector) <> '';

UPDATE companies
SET business_sectors = JSON_ARRAY()
WHERE business_sectors IS NULL;
