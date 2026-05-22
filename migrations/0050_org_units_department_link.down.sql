SET NAMES utf8mb4;

ALTER TABLE org_units
  DROP FOREIGN KEY fk_org_units_department,
  DROP KEY idx_org_units_department,
  DROP COLUMN department_id;
