SELECT user_id, email, login_id FROM users WHERE email LIKE '%admin.dn%' OR login_id LIKE '%admin.dn%' LIMIT 10;
SELECT user_id, email, login_id FROM users WHERE email LIKE '%@cobo%' OR email LIKE '%admin%' LIMIT 20;
SELECT TABLE_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='cobo_iam' AND COLUMN_NAME='email' AND TABLE_NAME LIKE '%user%' LIMIT 20;
SELECT id, user_id, company_id, kind, title, LEFT(body,80) body, resource_id, created_at
FROM user_in_app_notifications
WHERE title LIKE '%Xác định%' OR title LIKE '%phê duyệt đến hạn%' OR body LIKE '%Deadline:%' OR body LIKE '%Hạn:%'
ORDER BY created_at DESC LIMIT 10;
