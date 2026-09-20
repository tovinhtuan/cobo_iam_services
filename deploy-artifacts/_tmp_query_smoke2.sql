SELECT user_id, email, login_id FROM users WHERE email LIKE '%admin.dn%' OR email LIKE '%example.com%' LIMIT 20;
SELECT membership_id, user_id, company_id, status FROM memberships WHERE company_id='c_001' LIMIT 10;
SELECT occurrence_id, disclosure_id, scope_type, scope_id, status, scheduled_at
FROM reminder_occurrences
WHERE disclosure_id='52698f3f-53c0-53e7-9e40-d99f90774f41' OR disclosure_id LIKE '52698f3f%'
ORDER BY scheduled_at DESC LIMIT 10;
SELECT record_id, title, company_id FROM disclosure_records WHERE record_id='52698f3f-53c0-53e7-9e40-d99f90774f41';
