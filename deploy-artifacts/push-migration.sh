#!/bin/sh
# Push a single migration file to the dev server and apply it.
# Usage: sh push-migration.sh <migration_file>
# Example: sh push-migration.sh 0043_membership_direct_permissions.up.sql
#
# Dev server: 88.216.208.0:21239  user: root  path: /root/cobo_project/

set -eu

DEV_HOST="88.216.208.0"
DEV_PORT="21239"
DEV_USER="root"
DEV_PATH="/root/cobo_project"
MIGRATIONS_DIR="$(dirname "$0")/../migrations"

MIGRATION_FILE="${1:-}"
if [ -z "$MIGRATION_FILE" ]; then
  echo "Usage: sh push-migration.sh <migration_file>"
  echo "Example: sh push-migration.sh 0043_membership_direct_permissions.up.sql"
  exit 1
fi

LOCAL_FILE="${MIGRATIONS_DIR}/${MIGRATION_FILE}"
if [ ! -f "$LOCAL_FILE" ]; then
  echo "ERROR: File not found: $LOCAL_FILE"
  exit 1
fi

echo "==> Copying ${MIGRATION_FILE} to dev server..."
scp -P "$DEV_PORT" "$LOCAL_FILE" "${DEV_USER}@${DEV_HOST}:${DEV_PATH}/migrations/"

echo "==> Applying migration on dev server..."
ssh -p "$DEV_PORT" "${DEV_USER}@${DEV_HOST}" "
  docker exec -i cobo-iam-mysql mysql -uroot -proot cobo_iam \
    < ${DEV_PATH}/migrations/${MIGRATION_FILE} && \
  docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e \
    \"INSERT IGNORE INTO schema_migrations(file_name) VALUES ('${MIGRATION_FILE}');\" && \
  echo 'Migration applied and tracked.' || \
  echo 'ERROR: Migration failed — check logs above.'
"

echo "==> Verifying..."
ssh -p "$DEV_PORT" "${DEV_USER}@${DEV_HOST}" "
  docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam -e \
    \"SELECT file_name, executed_at FROM schema_migrations WHERE file_name='${MIGRATION_FILE}';\"
"
