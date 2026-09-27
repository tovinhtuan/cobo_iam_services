-- 0148 down: drop only the import-attempt table created by this migration.

SET NAMES utf8mb4;

DROP TABLE IF EXISTS cms_template_import_attempts;
