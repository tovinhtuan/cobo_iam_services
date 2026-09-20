SELECT record_id, LEFT(title, 80) AS title, company_id
FROM disclosure_records
WHERE title LIKE '%DEF006%' OR title LIKE '%QA%' OR title LIKE '%Milestone%'
ORDER BY updated_at DESC
LIMIT 10;

SELECT u.user_id, u.email, m.membership_id, m.company_id
FROM users u
JOIN memberships m ON m.user_id = u.user_id
WHERE u.email = 'admin.dn@example.com'
LIMIT 5;

SELECT id, title, body, resource_type, resource_id, created_at
FROM user_in_app_notifications
WHERE company_id = 'c_001'
ORDER BY created_at DESC
LIMIT 5;
