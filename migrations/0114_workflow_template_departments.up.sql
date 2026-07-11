-- Platform catalog: template-level default/recommended departments for global workflow steps.
-- These are NOT tenant company departments; companies may map/override at apply time.

SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS workflow_template_departments (
  department_code VARCHAR(64)  NOT NULL PRIMARY KEY,
  department_name VARCHAR(255) NOT NULL,
  description     TEXT         NULL,
  display_order   INT          NOT NULL DEFAULT 0,
  is_system       TINYINT(1)   NOT NULL DEFAULT 0,
  created_at      TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at      TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

INSERT INTO workflow_template_departments (department_code, department_name, description, display_order, is_system)
VALUES
  ('dept-001', 'Phòng Pháp chế', 'Quản lý các vấn đề pháp lý và tuân thủ của doanh nghiệp.', 1, 1),
  ('dept-002', 'Phòng Quan hệ cổ đông (IR)', 'Phụ trách công bố thông tin và quan hệ với nhà đầu tư.', 2, 1),
  ('dept-003', 'Phòng Kế toán', 'Quản lý tài chính, kế toán và lập báo cáo tài chính.', 3, 1),
  ('dept-004', 'Ban Tổng Giám đốc', 'Ban điều hành cao nhất của công ty.', 4, 1)
ON DUPLICATE KEY UPDATE
  department_name = VALUES(department_name),
  description = VALUES(description),
  display_order = VALUES(display_order),
  is_system = VALUES(is_system);
