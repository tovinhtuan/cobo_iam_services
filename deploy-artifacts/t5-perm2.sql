SELECT p.permission_code FROM roles r
JOIN role_permissions rp ON rp.role_id=r.role_id
JOIN permissions p ON p.permission_id=rp.permission_id
WHERE r.role_code='user_thuong' AND r.company_id='c_001' ORDER BY 1;

SELECT p.permission_code FROM roles r
JOIN role_permissions rp ON rp.role_id=r.role_id
JOIN permissions p ON p.permission_id=rp.permission_id
WHERE r.role_code='truong_phong_ban' AND r.company_id='c_001' AND p.permission_code LIKE 'ad_hoc%' OR (r.role_code='truong_phong_ban' AND r.company_id='c_001' AND p.permission_code LIKE 'workflow%')
ORDER BY 1 LIMIT 40;

SELECT permission_id, permission_code FROM permissions WHERE permission_code IN ('ad_hoc_alert.propose','ad_hoc_alert.read');
