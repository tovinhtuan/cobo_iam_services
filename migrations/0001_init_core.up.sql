SET NAMES utf8mb4;

CREATE TABLE users (
  user_id            VARCHAR(36) PRIMARY KEY,
  login_id           VARCHAR(191) NOT NULL,
  full_name          VARCHAR(255) NOT NULL,
  email              VARCHAR(255) NULL,
  phone              VARCHAR(32) NULL,
  account_status     VARCHAR(32) NOT NULL DEFAULT 'active',
  locked_until       TIMESTAMP NULL,
  last_login_at      TIMESTAMP NULL,
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_users_login_id (login_id),
  KEY idx_users_status (account_status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE credentials (
  credential_id      VARCHAR(36) PRIMARY KEY,
  user_id            VARCHAR(36) NOT NULL,
  credential_type    VARCHAR(32) NOT NULL DEFAULT 'password',
  password_hash      VARCHAR(255) NOT NULL,
  password_algo      VARCHAR(32) NOT NULL DEFAULT 'bcrypt',
  password_changed_at TIMESTAMP NULL,
  status             VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_credentials_user_type (user_id, credential_type),
  KEY idx_credentials_status (status),
  CONSTRAINT fk_credentials_user FOREIGN KEY (user_id) REFERENCES users(user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE sessions (
  session_id         VARCHAR(36) PRIMARY KEY,
  user_id            VARCHAR(36) NOT NULL,
  current_company_id VARCHAR(36) NULL,
  current_membership_id VARCHAR(36) NULL,
  refresh_token_hash VARCHAR(255) NOT NULL,
  refresh_expires_at TIMESTAMP NOT NULL,
  revoked_at         TIMESTAMP NULL,
  revoked_reason     VARCHAR(255) NULL,
  ip                 VARCHAR(64) NULL,
  user_agent         VARCHAR(512) NULL,
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_sessions_user (user_id),
  KEY idx_sessions_refresh_expires (refresh_expires_at),
  KEY idx_sessions_revoked (revoked_at),
  CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE login_attempts (
  attempt_id         BIGINT PRIMARY KEY AUTO_INCREMENT,
  login_id           VARCHAR(191) NOT NULL,
  user_id            VARCHAR(36) NULL,
  success            TINYINT(1) NOT NULL DEFAULT 0,
  failure_code       VARCHAR(64) NULL,
  ip                 VARCHAR(64) NULL,
  user_agent         VARCHAR(512) NULL,
  attempted_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_login_attempts_login_time (login_id, attempted_at),
  KEY idx_login_attempts_user_time (user_id, attempted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE companies (
  company_id         VARCHAR(36) PRIMARY KEY,
  company_code       VARCHAR(64) NOT NULL,
  company_name       VARCHAR(255) NOT NULL,
  status             VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_companies_code (company_code),
  KEY idx_companies_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE memberships (
  membership_id      VARCHAR(36) PRIMARY KEY,
  user_id            VARCHAR(36) NOT NULL,
  company_id         VARCHAR(36) NOT NULL,
  membership_status  VARCHAR(32) NOT NULL DEFAULT 'active',
  effective_from     TIMESTAMP NULL,
  effective_to       TIMESTAMP NULL,
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_memberships_user_company (user_id, company_id),
  KEY idx_memberships_company_status (company_id, membership_status),
  KEY idx_memberships_user_status (user_id, membership_status),
  CONSTRAINT fk_memberships_user FOREIGN KEY (user_id) REFERENCES users(user_id),
  CONSTRAINT fk_memberships_company FOREIGN KEY (company_id) REFERENCES companies(company_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE roles (
  role_id            VARCHAR(36) PRIMARY KEY,
  company_id         VARCHAR(36) NULL,
  role_code          VARCHAR(128) NOT NULL,
  role_name          VARCHAR(255) NOT NULL,
  status             VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_roles_company_code (company_id, role_code),
  KEY idx_roles_status (status),
  CONSTRAINT fk_roles_company FOREIGN KEY (company_id) REFERENCES companies(company_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE membership_roles (
  membership_role_id BIGINT PRIMARY KEY AUTO_INCREMENT,
  membership_id      VARCHAR(36) NOT NULL,
  role_id            VARCHAR(36) NOT NULL,
  status             VARCHAR(32) NOT NULL DEFAULT 'active',
  effective_from     TIMESTAMP NULL,
  effective_to       TIMESTAMP NULL,
  granted_by         VARCHAR(36) NULL,
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_membership_roles_unique (membership_id, role_id),
  KEY idx_membership_roles_active_window (membership_id, status, effective_from, effective_to),
  CONSTRAINT fk_membership_roles_membership FOREIGN KEY (membership_id) REFERENCES memberships(membership_id),
  CONSTRAINT fk_membership_roles_role FOREIGN KEY (role_id) REFERENCES roles(role_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE departments (
  department_id      VARCHAR(36) PRIMARY KEY,
  company_id         VARCHAR(36) NOT NULL,
  department_code    VARCHAR(128) NOT NULL,
  department_name    VARCHAR(255) NOT NULL,
  status             VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_departments_company_code (company_id, department_code),
  KEY idx_departments_company_status (company_id, status),
  CONSTRAINT fk_departments_company FOREIGN KEY (company_id) REFERENCES companies(company_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE department_memberships (
  department_membership_id BIGINT PRIMARY KEY AUTO_INCREMENT,
  membership_id      VARCHAR(36) NOT NULL,
  department_id      VARCHAR(36) NOT NULL,
  status             VARCHAR(32) NOT NULL DEFAULT 'active',
  effective_from     TIMESTAMP NULL,
  effective_to       TIMESTAMP NULL,
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_department_memberships_unique (membership_id, department_id),
  KEY idx_department_memberships_active_window (membership_id, status, effective_from, effective_to),
  KEY idx_department_memberships_department_status (department_id, status),
  CONSTRAINT fk_department_memberships_membership FOREIGN KEY (membership_id) REFERENCES memberships(membership_id),
  CONSTRAINT fk_department_memberships_department FOREIGN KEY (department_id) REFERENCES departments(department_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE titles (
  title_id           VARCHAR(36) PRIMARY KEY,
  company_id         VARCHAR(36) NOT NULL,
  title_code         VARCHAR(128) NOT NULL,
  title_name         VARCHAR(255) NOT NULL,
  status             VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_titles_company_code (company_id, title_code),
  KEY idx_titles_company_status (company_id, status),
  CONSTRAINT fk_titles_company FOREIGN KEY (company_id) REFERENCES companies(company_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE membership_titles (
  membership_title_id BIGINT PRIMARY KEY AUTO_INCREMENT,
  membership_id      VARCHAR(36) NOT NULL,
  title_id           VARCHAR(36) NOT NULL,
  status             VARCHAR(32) NOT NULL DEFAULT 'active',
  effective_from     TIMESTAMP NULL,
  effective_to       TIMESTAMP NULL,
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_membership_titles_unique (membership_id, title_id),
  KEY idx_membership_titles_active_window (membership_id, status, effective_from, effective_to),
  CONSTRAINT fk_membership_titles_membership FOREIGN KEY (membership_id) REFERENCES memberships(membership_id),
  CONSTRAINT fk_membership_titles_title FOREIGN KEY (title_id) REFERENCES titles(title_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE permissions (
  permission_id      VARCHAR(36) PRIMARY KEY,
  permission_code    VARCHAR(191) NOT NULL,
  permission_name    VARCHAR(255) NOT NULL,
  module_name        VARCHAR(64) NOT NULL,
  status             VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_permissions_code (permission_code),
  KEY idx_permissions_module_status (module_name, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE role_permissions (
  role_permission_id BIGINT PRIMARY KEY AUTO_INCREMENT,
  role_id            VARCHAR(36) NOT NULL,
  permission_id      VARCHAR(36) NOT NULL,
  status             VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_role_permissions_unique (role_id, permission_id),
  KEY idx_role_permissions_role_status (role_id, status),
  CONSTRAINT fk_role_permissions_role FOREIGN KEY (role_id) REFERENCES roles(role_id),
  CONSTRAINT fk_role_permissions_permission FOREIGN KEY (permission_id) REFERENCES permissions(permission_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE assignments (
  assignment_id      VARCHAR(36) PRIMARY KEY,
  company_id         VARCHAR(36) NOT NULL,
  assignee_type      VARCHAR(32) NOT NULL,
  assignee_ref_id    VARCHAR(36) NOT NULL,
  resource_type      VARCHAR(64) NOT NULL,
  resource_id        VARCHAR(64) NOT NULL,
  assignment_kind    VARCHAR(64) NOT NULL,
  status             VARCHAR(32) NOT NULL DEFAULT 'active',
  effective_from     TIMESTAMP NULL,
  effective_to       TIMESTAMP NULL,
  metadata_json      JSON NULL,
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_assignments_assignee_lookup (company_id, assignee_type, assignee_ref_id, status),
  KEY idx_assignments_resource_lookup (company_id, resource_type, resource_id, status),
  CONSTRAINT fk_assignments_company FOREIGN KEY (company_id) REFERENCES companies(company_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE resource_scope_rules (
  resource_scope_rule_id VARCHAR(36) PRIMARY KEY,
  company_id         VARCHAR(36) NOT NULL,
  rule_code          VARCHAR(128) NOT NULL,
  resource_type      VARCHAR(64) NOT NULL,
  scope_type         VARCHAR(64) NOT NULL,
  subject_type       VARCHAR(32) NOT NULL,
  subject_ref_id     VARCHAR(36) NOT NULL,
  status             VARCHAR(32) NOT NULL DEFAULT 'active',
  metadata_json      JSON NULL,
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_scope_rules_company_code (company_id, rule_code),
  KEY idx_scope_rules_resource_status (company_id, resource_type, status),
  CONSTRAINT fk_scope_rules_company FOREIGN KEY (company_id) REFERENCES companies(company_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE audit_logs (
  event_id           VARCHAR(36) PRIMARY KEY,
  occurred_at        TIMESTAMP NOT NULL,
  actor_user_id      VARCHAR(36) NULL,
  actor_membership_id VARCHAR(36) NULL,
  company_id         VARCHAR(36) NULL,
  action             VARCHAR(128) NOT NULL,
  resource_type      VARCHAR(64) NULL,
  resource_id        VARCHAR(64) NULL,
  decision           VARCHAR(32) NULL,
  request_id         VARCHAR(64) NULL,
  ip                 VARCHAR(64) NULL,
  user_agent         VARCHAR(512) NULL,
  effective_permissions_snapshot JSON NULL,
  effective_scope_snapshot JSON NULL,
  metadata_json      JSON NULL,
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_audit_logs_company_occurred (company_id, occurred_at),
  KEY idx_audit_logs_actor_occurred (actor_user_id, occurred_at),
  KEY idx_audit_logs_resource_occurred (resource_type, resource_id, occurred_at),
  KEY idx_audit_logs_request_id (request_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE outbox_events (
  event_id           VARCHAR(36) PRIMARY KEY,
  aggregate_type     VARCHAR(64) NOT NULL,
  aggregate_id       VARCHAR(64) NOT NULL,
  event_type         VARCHAR(128) NOT NULL,
  payload_json       JSON NOT NULL,
  status             VARCHAR(32) NOT NULL DEFAULT 'pending',
  available_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  processed_at       TIMESTAMP NULL,
  retry_count        INT NOT NULL DEFAULT 0,
  last_error         VARCHAR(1024) NULL,
  KEY idx_outbox_status_available (status, available_at),
  KEY idx_outbox_aggregate (aggregate_type, aggregate_id),
  KEY idx_outbox_event_type (event_type, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE idempotency_keys (
  idempotency_key_id VARCHAR(36) PRIMARY KEY,
  company_id         VARCHAR(36) NULL,
  scope              VARCHAR(64) NOT NULL,
  idempotency_key    VARCHAR(191) NOT NULL,
  request_hash       VARCHAR(255) NULL,
  response_json      JSON NULL,
  status             VARCHAR(32) NOT NULL DEFAULT 'completed',
  expires_at         TIMESTAMP NULL,
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_idempotency_scope_key (scope, idempotency_key),
  KEY idx_idempotency_company_scope (company_id, scope),
  KEY idx_idempotency_expires_at (expires_at),
  CONSTRAINT fk_idempotency_company FOREIGN KEY (company_id) REFERENCES companies(company_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
