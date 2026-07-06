SET NAMES utf8mb4;

ALTER TABLE department_memberships
  DROP KEY idx_department_memberships_focal,
  DROP COLUMN is_department_focal;
