-- Dev / QA: delete one invited user row (login_id == email canonical) plus dependent rows so CMS can invite again.
--
-- docker exec -i cobo-iam-mysql mysql -uroot -proot cobo_iam < scripts/delete_user_by_email_for_reinvite.sql

SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci;

SET @del := CONVERT('tvttthptlvh@gmail.com' USING utf8mb4) COLLATE utf8mb4_unicode_ci;

START TRANSACTION;

DELETE uit
FROM user_invitations uit
JOIN users u ON u.user_id = uit.user_id
WHERE u.login_id COLLATE utf8mb4_unicode_ci = @del
   OR (u.email IS NOT NULL AND u.email COLLATE utf8mb4_unicode_ci = @del);

DELETE pr
FROM password_reset_tokens pr
JOIN users u ON u.user_id = pr.user_id
WHERE u.login_id COLLATE utf8mb4_unicode_ci = @del
   OR (u.email IS NOT NULL AND u.email COLLATE utf8mb4_unicode_ci = @del);

DELETE ev
FROM email_verification_tokens ev
JOIN users u ON u.user_id = ev.user_id
WHERE u.login_id COLLATE utf8mb4_unicode_ci = @del
   OR (u.email IS NOT NULL AND u.email COLLATE utf8mb4_unicode_ci = @del);

DELETE ust
FROM user_subscription_tiers ust
JOIN users u ON u.user_id = ust.user_id
WHERE u.login_id COLLATE utf8mb4_unicode_ci = @del
   OR (u.email IS NOT NULL AND u.email COLLATE utf8mb4_unicode_ci = @del);

DELETE mep
FROM membership_effective_permissions mep
JOIN memberships m ON m.membership_id = mep.membership_id
JOIN users u ON u.user_id = m.user_id
WHERE u.login_id COLLATE utf8mb4_unicode_ci = @del
   OR (u.email IS NOT NULL AND u.email COLLATE utf8mb4_unicode_ci = @del);

DELETE med
FROM membership_effective_departments med
JOIN memberships m ON m.membership_id = med.membership_id
JOIN users u ON u.user_id = m.user_id
WHERE u.login_id COLLATE utf8mb4_unicode_ci = @del
   OR (u.email IS NOT NULL AND u.email COLLATE utf8mb4_unicode_ci = @del);

DELETE mer
FROM membership_effective_responsibilities mer
JOIN memberships m ON m.membership_id = mer.membership_id
JOIN users u ON u.user_id = m.user_id
WHERE u.login_id COLLATE utf8mb4_unicode_ci = @del
   OR (u.email IS NOT NULL AND u.email COLLATE utf8mb4_unicode_ci = @del);

DELETE eas
FROM effective_access_snapshots eas
JOIN memberships mem ON mem.membership_id = eas.membership_id
JOIN users u ON u.user_id = mem.user_id
WHERE u.login_id COLLATE utf8mb4_unicode_ci = @del
   OR (u.email IS NOT NULL AND u.email COLLATE utf8mb4_unicode_ci = @del);

DELETE oum
FROM org_unit_memberships oum
JOIN memberships m ON m.membership_id = oum.membership_id
JOIN users u ON u.user_id = m.user_id
WHERE u.login_id COLLATE utf8mb4_unicode_ci = @del
   OR (u.email IS NOT NULL AND u.email COLLATE utf8mb4_unicode_ci = @del);

DELETE mr
FROM membership_roles mr
JOIN memberships m ON m.membership_id = mr.membership_id
JOIN users u ON u.user_id = m.user_id
WHERE u.login_id COLLATE utf8mb4_unicode_ci = @del
   OR (u.email IS NOT NULL AND u.email COLLATE utf8mb4_unicode_ci = @del);

DELETE dm
FROM department_memberships dm
JOIN memberships m ON m.membership_id = dm.membership_id
JOIN users u ON u.user_id = m.user_id
WHERE u.login_id COLLATE utf8mb4_unicode_ci = @del
   OR (u.email IS NOT NULL AND u.email COLLATE utf8mb4_unicode_ci = @del);

DELETE mt
FROM membership_titles mt
JOIN memberships m ON m.membership_id = mt.membership_id
JOIN users u ON u.user_id = m.user_id
WHERE u.login_id COLLATE utf8mb4_unicode_ci = @del
   OR (u.email IS NOT NULL AND u.email COLLATE utf8mb4_unicode_ci = @del);

DELETE s
FROM sessions s
JOIN users u ON u.user_id = s.user_id
WHERE u.login_id COLLATE utf8mb4_unicode_ci = @del
   OR (u.email IS NOT NULL AND u.email COLLATE utf8mb4_unicode_ci = @del);

DELETE c
FROM credentials c
JOIN users u ON u.user_id = c.user_id
WHERE u.login_id COLLATE utf8mb4_unicode_ci = @del
   OR (u.email IS NOT NULL AND u.email COLLATE utf8mb4_unicode_ci = @del);

DELETE m
FROM memberships m
JOIN users u ON u.user_id = m.user_id
WHERE u.login_id COLLATE utf8mb4_unicode_ci = @del
   OR (u.email IS NOT NULL AND u.email COLLATE utf8mb4_unicode_ci = @del);

DELETE FROM login_attempts
WHERE login_id COLLATE utf8mb4_unicode_ci = @del
   OR user_id IN (
     SELECT uid FROM (
       SELECT user_id AS uid FROM users
       WHERE login_id COLLATE utf8mb4_unicode_ci = @del
          OR (email IS NOT NULL AND email COLLATE utf8mb4_unicode_ci = @del)
     ) t
   );

DELETE FROM users
WHERE login_id COLLATE utf8mb4_unicode_ci = @del
   OR (email IS NOT NULL AND email COLLATE utf8mb4_unicode_ci = @del);

COMMIT;

SELECT 'done' AS status;
