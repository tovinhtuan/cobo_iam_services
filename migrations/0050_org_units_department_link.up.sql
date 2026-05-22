SET NAMES utf8mb4;

-- Link org_units (teams) to their parent department.
-- For unit_type='team', department_id is NOT NULL and references departments.department_id.
-- For other unit types (department, division) this column is NULL.
ALTER TABLE org_units
  ADD COLUMN department_id VARCHAR(36) NULL AFTER parent_org_unit_id,
  ADD KEY idx_org_units_department (department_id),
  ADD CONSTRAINT fk_org_units_department
      FOREIGN KEY (department_id) REFERENCES departments(department_id);
