SET NAMES utf8mb4;

ALTER TABLE disclosure_type_versions
  DROP COLUMN deadline_config_json;

ALTER TABLE companies
  DROP COLUMN established_day,
  DROP COLUMN established_month,
  DROP COLUMN established_date;

