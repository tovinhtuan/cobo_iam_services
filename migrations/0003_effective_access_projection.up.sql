SET NAMES utf8mb4;

CREATE TABLE membership_effective_permissions (
  projection_id      BIGINT PRIMARY KEY AUTO_INCREMENT,
  company_id         VARCHAR(36) NOT NULL,
  membership_id      VARCHAR(36) NOT NULL,
  permission_code    VARCHAR(191) NOT NULL,
  source_type        VARCHAR(32) NOT NULL DEFAULT 'role',
  source_ref_id      VARCHAR(64) NULL,
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_membership_effective_permissions_unique (company_id, membership_id, permission_code),
  KEY idx_membership_effective_permissions_lookup (company_id, membership_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE membership_effective_departments (
  projection_id      BIGINT PRIMARY KEY AUTO_INCREMENT,
  company_id         VARCHAR(36) NOT NULL,
  membership_id      VARCHAR(36) NOT NULL,
  department_id      VARCHAR(36) NOT NULL,
  source_type        VARCHAR(32) NOT NULL DEFAULT 'department_membership',
  source_ref_id      VARCHAR(64) NULL,
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_membership_effective_departments_unique (company_id, membership_id, department_id),
  KEY idx_membership_effective_departments_lookup (company_id, membership_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE membership_effective_responsibilities (
  projection_id       BIGINT PRIMARY KEY AUTO_INCREMENT,
  company_id          VARCHAR(36) NOT NULL,
  membership_id       VARCHAR(36) NOT NULL,
  responsibility_code VARCHAR(191) NOT NULL,
  source_type         VARCHAR(32) NOT NULL DEFAULT 'role',
  source_ref_id       VARCHAR(64) NULL,
  created_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_membership_effective_responsibilities_unique (company_id, membership_id, responsibility_code),
  KEY idx_membership_effective_responsibilities_lookup (company_id, membership_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE effective_access_snapshots (
  snapshot_id         VARCHAR(36) PRIMARY KEY,
  company_id          VARCHAR(36) NOT NULL,
  membership_id       VARCHAR(36) NOT NULL,
  permissions_json    JSON NOT NULL,
  data_scope_json     JSON NOT NULL,
  responsibilities_json JSON NOT NULL,
  version             BIGINT NOT NULL DEFAULT 1,
  refreshed_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  expires_at          TIMESTAMP NULL,
  created_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_effective_access_snapshot_membership (company_id, membership_id),
  KEY idx_effective_access_snapshot_expires (expires_at),
  KEY idx_effective_access_snapshot_refreshed (company_id, refreshed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
