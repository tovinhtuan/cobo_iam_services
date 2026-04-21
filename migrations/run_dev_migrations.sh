#!/bin/sh
set -eu

DB_HOST="${DB_HOST:-mysql}"
DB_USER="${DB_USER:-root}"
DB_PASSWORD="${DB_PASSWORD:-root}"
DB_NAME="${DB_NAME:-cobo_iam}"

MIGRATIONS="
0001_init_core.up.sql
0003_effective_access_projection.up.sql
0004_p1_business_tables.up.sql
0005_sessions_refresh_hash_unique.up.sql
0006_admin_rules_tables.up.sql
0007_auth_recovery_tokens.up.sql
0008_org_units_scope.up.sql
0009_seed_authz_test_accounts.up.sql
seed_dev_identity_authorization.sql
"

mysql_exec() {
  MYSQL_PWD="${DB_PASSWORD}" mysql -h "${DB_HOST}" -u"${DB_USER}" "${DB_NAME}" "$@"
}

echo "Ensuring schema_migrations table exists..."
mysql_exec <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
  file_name VARCHAR(255) PRIMARY KEY,
  executed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
SQL

echo "Ensuring auth plugin is caching_sha2_password..."
mysql_exec <<'SQL'
ALTER USER IF EXISTS 'root'@'localhost' IDENTIFIED WITH caching_sha2_password BY 'root';
ALTER USER IF EXISTS 'root'@'%' IDENTIFIED WITH caching_sha2_password BY 'root';
ALTER USER IF EXISTS 'cobo'@'%' IDENTIFIED WITH caching_sha2_password BY 'cobo';
SQL

existing_count="$(mysql_exec -Nse "SELECT COUNT(1) FROM information_schema.tables WHERE table_schema='${DB_NAME}' AND table_name='users'")"
tracked_count="$(mysql_exec -Nse "SELECT COUNT(1) FROM schema_migrations")"
if [ "${existing_count}" -gt 0 ] && [ "${tracked_count}" -eq 0 ]; then
  echo "Bootstrapped database detected without migration history. Creating baseline..."
  for file in ${MIGRATIONS}; do
    mysql_exec -e "INSERT IGNORE INTO schema_migrations(file_name) VALUES ('${file}')"
  done
fi

for file in ${MIGRATIONS}; do
  applied="$(mysql_exec -Nse "SELECT COUNT(1) FROM schema_migrations WHERE file_name='${file}'")"
  if [ "${applied}" -gt 0 ]; then
    echo "Skipping already applied migration: ${file}"
    continue
  fi

  echo "Applying migration: ${file}"
  mysql_exec < "migrations/${file}"
  mysql_exec -e "INSERT INTO schema_migrations(file_name) VALUES ('${file}')"
done

echo "All migrations are up to date."
