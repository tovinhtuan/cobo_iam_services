#!/bin/sh
set -eu
docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e "SHOW COLUMNS FROM companies LIKE 'founder_user_id';"
docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e "SHOW COLUMNS FROM companies LIKE 'provisioning_source';"
docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e "SHOW INDEX FROM companies WHERE Key_name='idx_companies_founder_provisioning';"
docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -N -e "SELECT version FROM schema_migrations WHERE version LIKE '%0082%';"
