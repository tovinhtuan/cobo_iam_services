SET NAMES utf8mb4;

ALTER TABLE roles
  ADD COLUMN role_type VARCHAR(32) NOT NULL DEFAULT 'tenant_default' AFTER status,
  ADD COLUMN is_protected TINYINT(1) NOT NULL DEFAULT 0 AFTER role_type,
  ADD COLUMN description TEXT NULL AFTER is_protected,
  ADD COLUMN created_by VARCHAR(36) NULL AFTER description,
  ADD COLUMN updated_by VARCHAR(36) NULL AFTER created_by;

UPDATE roles
SET role_type = 'system_global',
    is_protected = 1
WHERE company_id IS NULL;

UPDATE roles
SET role_type = 'tenant_default',
    is_protected = 1
WHERE company_id IS NOT NULL
  AND role_code IN ('admin_doanh_nghiep', 'user_thuong');

UPDATE roles
SET role_type = 'tenant_default',
    is_protected = 1
WHERE company_id IS NOT NULL
  AND role_code NOT IN ('admin_doanh_nghiep', 'user_thuong')
  AND role_type = 'tenant_default';
