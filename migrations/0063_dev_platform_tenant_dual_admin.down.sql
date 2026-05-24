SET NAMES utf8mb4;

DELETE FROM org_unit_memberships
WHERE membership_id IN ('m_107', 'm_108');

DELETE FROM membership_effective_responsibilities
WHERE membership_id IN ('m_107', 'm_108');

DELETE FROM department_memberships
WHERE membership_id IN ('m_107', 'm_108');

DELETE FROM membership_direct_permissions
WHERE granted_by = 'system_migration_0063';

DELETE FROM membership_roles
WHERE membership_id IN ('m_107', 'm_108');

DELETE FROM memberships
WHERE membership_id IN ('m_107', 'm_108');

DELETE FROM credentials
WHERE user_id = 'u_platform_tenant_admin';

DELETE FROM user_subscription_tiers
WHERE user_id = 'u_platform_tenant_admin';

DELETE FROM users
WHERE user_id = 'u_platform_tenant_admin';

DELETE rp
FROM role_permissions rp
WHERE rp.role_id IN (
  'r0000001-0001-4000-8000-000000000017',
  'r0000001-0001-4000-8000-000000000018'
);

DELETE FROM roles
WHERE role_id IN (
  'r0000001-0001-4000-8000-000000000017',
  'r0000001-0001-4000-8000-000000000018'
);
