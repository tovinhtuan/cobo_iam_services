-- 0117: Repair mojibake in workflow_template_departments seed names/descriptions.
-- Root cause: 0114 INSERT applied with a non-utf8mb4 client charset, so UTF-8
-- Vietnamese bytes were stored as Windows-1252/latin1 mojibake (e.g. Ban Tá»•ng…).
-- Company departments table is already correct utf8mb4; only the template catalog
-- was corrupted. Source of truth: migrations/0114 + disclosure catalog seed.
-- Safe to re-run: idempotent SET to canonical Vietnamese strings.

SET NAMES utf8mb4;

UPDATE workflow_template_departments
SET
  department_name = 'Phòng Pháp chế',
  description = 'Quản lý các vấn đề pháp lý và tuân thủ của doanh nghiệp.',
  updated_at = CURRENT_TIMESTAMP(6)
WHERE department_code = 'dept-001';

UPDATE workflow_template_departments
SET
  department_name = 'Phòng Quan hệ cổ đông (IR)',
  description = 'Phụ trách công bố thông tin và quan hệ với nhà đầu tư.',
  updated_at = CURRENT_TIMESTAMP(6)
WHERE department_code = 'dept-002';

UPDATE workflow_template_departments
SET
  department_name = 'Phòng Kế toán',
  description = 'Quản lý tài chính, kế toán và lập báo cáo tài chính.',
  updated_at = CURRENT_TIMESTAMP(6)
WHERE department_code = 'dept-003';

UPDATE workflow_template_departments
SET
  department_name = 'Ban Tổng Giám đốc',
  description = 'Ban điều hành cao nhất của công ty.',
  updated_at = CURRENT_TIMESTAMP(6)
WHERE department_code = 'dept-004';
