-- 0150: short-lived, one-time OAuth authorization codes for CoBo Template Builder.
-- Access tokens are signed and intentionally not persisted. This table stores only a SHA-256
-- code hash, never the raw authorization code, user access token, or template JSON.

SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS template_builder_oauth_authorization_codes (
  code_id CHAR(36) NOT NULL,
  code_hash CHAR(64) NOT NULL,
  client_id VARCHAR(128) NOT NULL,
  redirect_uri VARCHAR(2048) NOT NULL,
  scope VARCHAR(128) NOT NULL,
  code_challenge VARCHAR(128) NOT NULL,
  user_id VARCHAR(128) NOT NULL,
  membership_id VARCHAR(128) NOT NULL,
  company_id VARCHAR(128) NOT NULL,
  expires_at DATETIME(3) NOT NULL,
  consumed_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (code_id),
  UNIQUE KEY uq_template_builder_oauth_code_hash (code_hash),
  KEY idx_template_builder_oauth_code_expiry (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
