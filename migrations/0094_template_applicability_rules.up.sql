SET NAMES utf8mb4;

ALTER TABLE disclosure_type_versions
  ADD COLUMN applicability_rules_json JSON NULL COMMENT 'Template applicability rules (3 dimensions)';
