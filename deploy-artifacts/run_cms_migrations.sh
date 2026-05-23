#!/bin/sh
# Apply CMS migrations 0053-0057 on the dev server.
# Run inside the server: sh /root/cobo_project/run_cms_migrations.sh

set -eu

MP="/root/cobo_project/migrations"
MYSQL="docker exec -i cobo-iam-mysql mysql -uroot -proot cobo_iam"
MYSQL_CMD="docker exec cobo-iam-mysql mysql -uroot -proot cobo_iam"

apply() {
  FILE="$1"
  already=$($MYSQL_CMD -Nse "SELECT COUNT(1) FROM schema_migrations WHERE file_name='$FILE'" 2>/dev/null || echo 0)
  if [ "$already" -gt 0 ]; then
    echo "SKIP (already tracked): $FILE"
    return
  fi
  echo "==> Applying $FILE ..."
  $MYSQL < "$MP/$FILE" && \
    $MYSQL_CMD -e "INSERT IGNORE INTO schema_migrations(file_name) VALUES ('$FILE');" && \
    echo "OK: $FILE" || \
    echo "FAIL: $FILE"
}

apply "0053_cms_portal_template_tables.up.sql"
apply "0054_cms_display_groups_po_seed.up.sql"
apply "0055_cms_system_template_seed.up.sql"
apply "0056_company_template_lifecycle.up.sql"
apply "0057_workflow_override_versioning.up.sql"

echo ""
echo "=== Final state ==="
$MYSQL_CMD -e "SELECT file_name, executed_at FROM schema_migrations WHERE file_name LIKE '005%' ORDER BY file_name;"
echo ""
echo "=== Templates in DB ==="
$MYSQL_CMD -e "SELECT type_id, status, is_mandatory FROM disclosure_types WHERE company_id IS NULL;"
