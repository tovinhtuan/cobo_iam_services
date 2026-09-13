#!/bin/sh
set -eu
docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e "SHOW TABLES LIKE 'workflow_step_document_fulfillment_files';"
docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e "SELECT file_name FROM schema_migrations WHERE file_name LIKE '%0136%';"
