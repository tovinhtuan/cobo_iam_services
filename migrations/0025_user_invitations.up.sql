SET NAMES utf8mb4;

CREATE TABLE user_invitations (
  invitation_id          VARCHAR(36) PRIMARY KEY,
  user_id                VARCHAR(36) NOT NULL,
  token_hash             VARCHAR(255) NOT NULL,
  expires_at             TIMESTAMP NOT NULL,
  used_at                TIMESTAMP NULL,
  revoked_at             TIMESTAMP NULL,
  created_by_user_id     VARCHAR(36) NULL,
  created_at             TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  send_count             INT NOT NULL DEFAULT 1,
  last_sent_at           TIMESTAMP NULL,
  UNIQUE KEY uk_user_invitations_hash (token_hash),
  KEY idx_user_invitations_user (user_id),
  KEY idx_user_invitations_exp (expires_at),
  CONSTRAINT fk_user_invitations_user FOREIGN KEY (user_id) REFERENCES users(user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
