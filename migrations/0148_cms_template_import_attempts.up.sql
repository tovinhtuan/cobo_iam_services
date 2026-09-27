-- 0148: CMS template import attempt history. Expand-only. No raw payload or token.

SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS cms_template_import_attempts (
  id CHAR(36) NOT NULL,
  company_id VARCHAR(64) NOT NULL,
  actor_user_id VARCHAR(64) NOT NULL,
  actor_membership_id VARCHAR(64) NULL,
  filename VARCHAR(255) NOT NULL,
  file_size_bytes INT UNSIGNED NOT NULL,
  file_sha256 CHAR(64) NOT NULL,
  canonical_payload_sha256 CHAR(64) NULL,
  validation_token_sha256 CHAR(64) NULL,
  schema_version VARCHAR(16) NULL,
  status VARCHAR(32) NOT NULL,
  parse_valid TINYINT(1) NOT NULL,
  domain_valid TINYINT(1) NOT NULL,
  activation_ready TINYINT(1) NOT NULL,
  can_confirm TINYINT(1) NOT NULL,
  mapping_required TINYINT(1) NOT NULL,
  required_mapping_count INT UNSIGNED NOT NULL,
  resolved_mapping_count INT UNSIGNED NOT NULL,
  unresolved_mapping_count INT UNSIGNED NOT NULL,
  error_codes JSON NULL,
  mapping_summary JSON NULL,
  target_type_id VARCHAR(64) NULL,
  created_type_id VARCHAR(128) NULL,
  confirm_error_code VARCHAR(64) NULL,
  row_version BIGINT NOT NULL,
  created_at DATETIME(3) NOT NULL,
  validated_at DATETIME(3) NULL,
  confirmed_at DATETIME(3) NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_cms_import_attempts_company_created (company_id, created_at, id),
  KEY idx_cms_import_attempts_company_status (company_id, status, created_at, id),
  KEY idx_cms_import_attempts_company_filehash (company_id, file_sha256, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
