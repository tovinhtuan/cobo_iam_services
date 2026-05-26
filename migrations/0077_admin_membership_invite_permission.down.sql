SET NAMES utf8mb4;

DELETE rp FROM role_permissions rp
INNER JOIN permissions p ON p.permission_id = rp.permission_id AND p.permission_code = 'admin.membership.invite';

DELETE FROM permissions WHERE permission_code = 'admin.membership.invite';
