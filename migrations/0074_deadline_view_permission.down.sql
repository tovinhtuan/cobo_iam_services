SET NAMES utf8mb4;

DELETE rp FROM role_permissions rp
INNER JOIN permissions p ON p.permission_id = rp.permission_id AND p.permission_code = 'deadline.view';

DELETE FROM permissions WHERE permission_code = 'deadline.view' AND permission_id = '10000000-0001-4000-8000-000000000019';
