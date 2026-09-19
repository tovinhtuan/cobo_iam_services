-- 0138 down: DEV/test or operator-confirmed only.
-- Production: use forward-fix; role_permissions has no grant provenance.

SET NAMES utf8mb4;

DELETE rp FROM role_permissions rp
INNER JOIN permissions p ON p.permission_id = rp.permission_id AND p.permission_code = 'deadline.comment';

DELETE FROM permissions
WHERE permission_code = 'deadline.comment'
  AND permission_id = '10000000-0001-4000-8000-000000000028';
