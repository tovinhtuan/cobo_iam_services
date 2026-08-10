SELECT r.role_code, p.permission_code
FROM roles r
JOIN role_permissions rp ON rp.role_id=r.role_id
JOIN permissions p ON p.permission_id=rp.permission_id
WHERE r.company_id='c_001' AND p.permission_code LIKE 'ad_hoc%'
ORDER BY r.role_code, p.permission_code;

SELECT m.membership_id, u.login_id, r.role_code
FROM memberships m
JOIN users u ON u.user_id=m.user_id
JOIN membership_roles mr ON mr.membership_id=m.membership_id AND mr.status='active'
JOIN roles r ON r.role_id=mr.role_id
WHERE m.company_id='c_001' AND m.membership_id IN ('m_102','m_103','m_104','m_105');
