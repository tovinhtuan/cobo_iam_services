-- Per-role default permission grants shown as pre-checked checkboxes at invite time.
-- Enables data-driven configuration of which grantable permissions are on-by-default
-- for a given role, without hardcoding tier logic in application code.

-- Insert disclosure_type.manage permission (company-scoped disclosure type CRUD).
INSERT IGNORE INTO permissions (permission_id, permission_code, permission_name, module_name, status)
VALUES (UUID(), 'disclosure_type.manage',
        'Manage company disclosure types',
        'cms',
        'active');

CREATE TABLE role_default_grant_permissions (
  id              BIGINT PRIMARY KEY AUTO_INCREMENT,
  role_id         VARCHAR(36)  NOT NULL,
  permission_code VARCHAR(191) NOT NULL,
  created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_rdgp (role_id, permission_code),
  CONSTRAINT fk_rdgp_role FOREIGN KEY (role_id) REFERENCES roles(role_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
