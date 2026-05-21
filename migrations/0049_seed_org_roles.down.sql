DELETE rp FROM role_permissions rp
JOIN roles r ON r.role_id = rp.role_id
WHERE r.role_code = 'dept_lead';

DELETE FROM roles       WHERE role_code = 'dept_lead';
DELETE FROM permissions WHERE permission_code IN ('dept.manage', 'company.ownership.transfer');
