SELECT
  SUM(CASE WHEN resource_id IS NOT NULL AND resource_id <> '' THEN 1 ELSE 0 END) AS with_resource,
  SUM(CASE WHEN resource_id IS NULL OR resource_id = '' THEN 1 ELSE 0 END) AS without_resource,
  COUNT(*) AS total
FROM user_in_app_notifications
WHERE user_id='u_admin_dn' AND company_id='c_001'
  AND (title LIKE '%phê duyệt đến hạn%' OR title LIKE N'%phê duyệt đến hạn%' OR kind LIKE 'reminder%');

SELECT id, LEFT(title,60) title, LEFT(IFNULL(body,''),40) body,
       IFNULL(resource_type,'') rt, IFNULL(resource_id,'') rid, created_at
FROM user_in_app_notifications
WHERE user_id='u_admin_dn' AND company_id='c_001'
ORDER BY created_at DESC
LIMIT 12;
