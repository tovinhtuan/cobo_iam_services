SET NAMES utf8mb4;

-- Unsafe to drop if tenant_custom rows exist (future custom-role feature).
-- Manual rollback: verify `SELECT COUNT(*) FROM roles WHERE role_type = 'tenant_custom'` is 0 first.

ALTER TABLE roles
  DROP COLUMN updated_by,
  DROP COLUMN created_by,
  DROP COLUMN description,
  DROP COLUMN is_protected,
  DROP COLUMN role_type;
