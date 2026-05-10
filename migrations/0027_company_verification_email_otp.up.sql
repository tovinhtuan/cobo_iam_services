SET NAMES utf8mb4;

-- Tenant verification (self-reg starts unverified; CMS can move to verified later).
ALTER TABLE companies
  ADD COLUMN verification_status VARCHAR(32) NOT NULL DEFAULT 'verified'
  AFTER status;

CREATE TABLE email_verification_otps (
  otp_id        VARCHAR(36) PRIMARY KEY,
  user_id       VARCHAR(36) NOT NULL,
  code_hash     VARCHAR(255) NOT NULL,
  expires_at    TIMESTAMP NOT NULL,
  attempt_count INT NOT NULL DEFAULT 0,
  max_attempts  INT NOT NULL DEFAULT 5,
  consumed_at   TIMESTAMP NULL,
  created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_email_verification_otps_user_created (user_id, created_at),
  KEY idx_email_verification_otps_user_exp (user_id, expires_at),
  CONSTRAINT fk_email_verification_otps_user FOREIGN KEY (user_id) REFERENCES users(user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
