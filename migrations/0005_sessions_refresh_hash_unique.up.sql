SET NAMES utf8mb4;

ALTER TABLE sessions
  ADD UNIQUE KEY uk_sessions_refresh_token_hash (refresh_token_hash);
