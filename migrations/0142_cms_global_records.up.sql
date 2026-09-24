-- 0142: Global CMS Records + materialization runs + link on disclosure_records.
-- Additive only. Legacy disclosure_records.cms_record_id stays NULL (no blind backfill).
-- Permissions cms.record.* granted to roles holding platform.cms.view (system CMS).

SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS cms_global_records (
  id VARCHAR(36) NOT NULL,
  template_id VARCHAR(128) NOT NULL,
  cycle_key VARCHAR(128) NOT NULL,
  title VARCHAR(512) NOT NULL,
  summary TEXT NULL,
  content MEDIUMTEXT NOT NULL,
  status VARCHAR(32) NOT NULL,
  template_version_no INT NULL,
  created_by VARCHAR(64) NULL,
  updated_by VARCHAR(64) NULL,
  published_by VARCHAR(64) NULL,
  archived_by VARCHAR(64) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  published_at DATETIME(3) NULL,
  archived_at DATETIME(3) NULL,
  cycle_start DATE NULL,
  due_date DATE NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_cms_global_template_cycle (template_id, cycle_key),
  KEY idx_cms_global_template_status_cycle (template_id, status, cycle_key),
  KEY idx_cms_global_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS cms_materialization_runs (
  run_id VARCHAR(36) NOT NULL,
  cms_record_id VARCHAR(36) NOT NULL,
  actor_user_id VARCHAR(64) NULL,
  actor_membership_id VARCHAR(64) NULL,
  mode VARCHAR(32) NOT NULL,
  dry_run TINYINT(1) NOT NULL DEFAULT 0,
  status VARCHAR(32) NOT NULL,
  requested_count INT NOT NULL DEFAULT 0,
  created_count INT NOT NULL DEFAULT 0,
  exists_count INT NOT NULL DEFAULT 0,
  skipped_count INT NOT NULL DEFAULT 0,
  failed_count INT NOT NULL DEFAULT 0,
  started_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  completed_at DATETIME(3) NULL,
  PRIMARY KEY (run_id),
  KEY idx_cms_mat_runs_record (cms_record_id),
  CONSTRAINT fk_cms_mat_runs_record FOREIGN KEY (cms_record_id) REFERENCES cms_global_records (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS cms_materialization_items (
  run_id VARCHAR(36) NOT NULL,
  company_id VARCHAR(36) NOT NULL,
  outcome VARCHAR(32) NOT NULL,
  company_record_id VARCHAR(36) NULL,
  error_code VARCHAR(64) NULL,
  error_message VARCHAR(1024) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (run_id, company_id),
  KEY idx_cms_mat_items_company (company_id),
  CONSTRAINT fk_cms_mat_items_run FOREIGN KEY (run_id) REFERENCES cms_materialization_runs (run_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

SET @col_exists := (
  SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'disclosure_records'
    AND column_name = 'cms_record_id'
);
SET @sql := IF(
  @col_exists = 0,
  'ALTER TABLE disclosure_records ADD COLUMN cms_record_id VARCHAR(36) NULL DEFAULT NULL AFTER type_id',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @idx_exists := (
  SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'disclosure_records'
    AND index_name = 'idx_disclosure_records_cms_record'
);
SET @sql := IF(
  @idx_exists = 0,
  'ALTER TABLE disclosure_records ADD KEY idx_disclosure_records_cms_record (cms_record_id)',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Unique (cms_record_id, company_id): multiple NULLs allowed in MySQL UNIQUE → legacy OK.
SET @idx_exists := (
  SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'disclosure_records'
    AND index_name = 'uq_disclosure_records_cms_company'
);
SET @sql := IF(
  @idx_exists = 0,
  'ALTER TABLE disclosure_records ADD UNIQUE KEY uq_disclosure_records_cms_company (cms_record_id, company_id)',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

INSERT INTO permissions (permission_id, permission_code, permission_name, module_name, status)
VALUES
  (UUID(), 'cms.record.read', 'Read Global CMS Records', 'cms', 'active'),
  (UUID(), 'cms.record.write', 'Create and update Global CMS Record drafts', 'cms', 'active'),
  (UUID(), 'cms.record.publish', 'Publish or archive Global CMS Records', 'cms', 'active'),
  (UUID(), 'cms.record.materialize', 'Materialize Global CMS Records to companies', 'cms', 'active')
ON DUPLICATE KEY UPDATE
  permission_name = VALUES(permission_name),
  module_name = VALUES(module_name),
  status = VALUES(status);

INSERT IGNORE INTO role_permissions (role_id, permission_id, status)
SELECT rp.role_id, p.permission_id, 'active'
FROM role_permissions rp
INNER JOIN permissions legacy_perm ON legacy_perm.permission_id = rp.permission_id
INNER JOIN permissions p ON p.permission_code IN (
  'cms.record.read', 'cms.record.write', 'cms.record.publish', 'cms.record.materialize'
)
WHERE legacy_perm.permission_code = 'platform.cms.view'
  AND rp.status = 'active';

INSERT IGNORE INTO role_default_grant_permissions (role_id, permission_code)
SELECT DISTINCT rd.role_id, codes.permission_code
FROM role_default_grant_permissions rd
CROSS JOIN (
  SELECT 'cms.record.read' AS permission_code UNION ALL
  SELECT 'cms.record.write' UNION ALL
  SELECT 'cms.record.publish' UNION ALL
  SELECT 'cms.record.materialize'
) codes
WHERE rd.permission_code = 'platform.cms.view';
