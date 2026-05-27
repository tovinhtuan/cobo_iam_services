-- 0081: User profile avatar storage (self-service /me/avatar; Batch 1 schema).
SET NAMES utf8mb4;

ALTER TABLE users
  ADD COLUMN avatar_object_key VARCHAR(512) NULL AFTER full_name,
  ADD COLUMN avatar_content_type VARCHAR(64) NULL AFTER avatar_object_key,
  ADD COLUMN avatar_updated_at TIMESTAMP NULL AFTER avatar_content_type;

CREATE TABLE IF NOT EXISTS user_media_assets (
  asset_id VARCHAR(64) NOT NULL PRIMARY KEY,
  user_id VARCHAR(64) NOT NULL,
  object_key VARCHAR(512) NOT NULL,
  content_type VARCHAR(64) NOT NULL,
  size_bytes BIGINT NOT NULL,
  state ENUM('pending_upload', 'ready', 'deleted') NOT NULL DEFAULT 'pending_upload',
  expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_user_media_object_key (object_key),
  KEY idx_user_media_user_state (user_id, state),
  CONSTRAINT fk_user_media_assets_user FOREIGN KEY (user_id) REFERENCES users(user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
