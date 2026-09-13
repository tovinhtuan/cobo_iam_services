#!/bin/sh
set -eu
docker exec -i cobo-iam-mysql mysql -uroot -proot cobo_iam <<'SQL'
INSERT IGNORE INTO schema_migrations(file_name) VALUES ('0136_workflow_step_document_fulfillment_files.up.sql');
SELECT file_name, executed_at FROM schema_migrations WHERE file_name LIKE '%0136%';
DESCRIBE workflow_step_document_fulfillment_files;
SQL
