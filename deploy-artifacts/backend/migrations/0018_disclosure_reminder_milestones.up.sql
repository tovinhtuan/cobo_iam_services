SET NAMES utf8mb4;

ALTER TABLE disclosure_type_versions
  ADD COLUMN reminder_milestones_json JSON NULL AFTER general_info;
