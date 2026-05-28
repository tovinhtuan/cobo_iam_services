#!/bin/sh
set -eu
echo "=== schema_migrations 008x ==="
docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -Nse \
  "SELECT file_name FROM schema_migrations WHERE file_name LIKE '008%' ORDER BY file_name"
echo "=== companies columns ==="
docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -Nse \
  "SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='cobo_iam' AND TABLE_NAME='companies' AND COLUMN_NAME IN ('founder_user_id','provisioning_source')"
echo "=== index ==="
docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -Nse \
  "SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='cobo_iam' AND TABLE_NAME='companies' AND INDEX_NAME='idx_companies_founder_provisioning'"
