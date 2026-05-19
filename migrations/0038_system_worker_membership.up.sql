SET NAMES utf8mb4;

-- System worker user: used as created_by for auto-generated disclosure records.
-- FE should display 'm_system_worker' creator as "Hệ thống".
INSERT IGNORE INTO users (user_id, email, full_name, status, created_at, updated_at)
VALUES ('u_system_worker', 'system@cobo.internal', 'System Worker', 'active', NOW(3), NOW(3));

INSERT IGNORE INTO memberships (membership_id, user_id, company_id, status, created_at, updated_at)
VALUES ('m_system_worker', 'u_system_worker', 'SYSTEM', 'active', NOW(3), NOW(3));
