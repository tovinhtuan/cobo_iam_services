SET NAMES utf8mb4;

-- departments already exists (0001_init_core). Add head_membership_id for trưởng phòng
-- and sort_order for display ordering. Both are nullable/defaulted — safe on existing rows.
ALTER TABLE departments
  ADD COLUMN head_membership_id VARCHAR(36) NULL         AFTER department_name,
  ADD COLUMN sort_order         INT         NOT NULL DEFAULT 0 AFTER head_membership_id,
  ADD KEY idx_departments_head (head_membership_id),
  ADD CONSTRAINT fk_departments_head
      FOREIGN KEY (head_membership_id) REFERENCES memberships(membership_id);
