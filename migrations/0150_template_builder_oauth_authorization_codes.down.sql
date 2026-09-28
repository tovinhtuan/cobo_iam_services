-- 0150 down: remove only the OAuth authorization-code table created by this migration.

SET NAMES utf8mb4;

DROP TABLE IF EXISTS template_builder_oauth_authorization_codes;
