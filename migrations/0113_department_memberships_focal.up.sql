SET NAMES utf8mb4;

ALTER TABLE department_memberships
  ADD COLUMN is_department_focal TINYINT(1) NOT NULL DEFAULT 0
    COMMENT '1 = membership is focal point for this department'
    AFTER status,
  ADD KEY idx_department_memberships_focal (department_id, is_department_focal, status);
