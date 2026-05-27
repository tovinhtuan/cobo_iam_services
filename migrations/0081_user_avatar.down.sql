DROP TABLE IF EXISTS user_media_assets;

ALTER TABLE users
  DROP COLUMN avatar_updated_at,
  DROP COLUMN avatar_content_type,
  DROP COLUMN avatar_object_key;
