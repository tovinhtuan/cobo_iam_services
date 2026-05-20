SET NAMES utf8mb4;

ALTER TABLE companies
  ADD COLUMN established_date DATE NULL AFTER status,
  ADD COLUMN established_month TINYINT UNSIGNED NOT NULL DEFAULT 1 AFTER established_date,
  ADD COLUMN established_day TINYINT UNSIGNED NOT NULL DEFAULT 1 AFTER established_month;

ALTER TABLE disclosure_type_versions
  ADD COLUMN deadline_config_json JSON NULL AFTER reminder_milestones_json;

