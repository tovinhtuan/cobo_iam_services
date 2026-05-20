SET NAMES utf8mb4;

ALTER TABLE users
  ADD COLUMN email_verified_at TIMESTAMP NULL AFTER email;

CREATE TABLE password_reset_tokens (
  token_id           VARCHAR(36) PRIMARY KEY,
  user_id            VARCHAR(36) NOT NULL,
  token_hash         VARCHAR(255) NOT NULL,
  expires_at         TIMESTAMP NOT NULL,
  used_at            TIMESTAMP NULL,
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_password_reset_tokens_hash (token_hash),
  KEY idx_password_reset_tokens_user (user_id),
  KEY idx_password_reset_tokens_exp (expires_at),
  CONSTRAINT fk_password_reset_tokens_user FOREIGN KEY (user_id) REFERENCES users(user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE email_verification_tokens (
  token_id           VARCHAR(36) PRIMARY KEY,
  user_id            VARCHAR(36) NOT NULL,
  token_hash         VARCHAR(255) NOT NULL,
  expires_at         TIMESTAMP NOT NULL,
  used_at            TIMESTAMP NULL,
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_email_verification_tokens_hash (token_hash),
  KEY idx_email_verification_tokens_user (user_id),
  KEY idx_email_verification_tokens_exp (expires_at),
  CONSTRAINT fk_email_verification_tokens_user FOREIGN KEY (user_id) REFERENCES users(user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
