-- Platform catalog: custom workflow assignee roles for global workflow step configuration.
-- Separate from IAM roles/permissions (roles table). Catalog-only metadata.

CREATE TABLE IF NOT EXISTS workflow_assignee_role_catalog (
  role_code   VARCHAR(64)  NOT NULL PRIMARY KEY,
  role_name   VARCHAR(255) NOT NULL,
  description TEXT         NULL,
  status      VARCHAR(16)  NOT NULL DEFAULT 'active',
  is_system   TINYINT(1)   NOT NULL DEFAULT 0,
  created_at  TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at  TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  UNIQUE KEY uq_workflow_assignee_role_name (role_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
