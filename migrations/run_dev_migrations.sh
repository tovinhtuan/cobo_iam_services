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
0011_user_subscription_tiers.up.sql
seed_dev_identity_authorization.sql
0012_disclosure_catalog_versions.up.sql
0013_cms_media_assets.up.sql
0014_disclosure_catalog_extra_types.up.sql
0015_disclosure_template_enums.up.sql
0016_disclosure_template_blocks.up.sql
0017_disclosure_template_mandatory_backfill.up.sql
0018_disclosure_reminder_milestones.up.sql
0019_disclosure_legal_bases_checklist_json.up.sql
0020_company_template_workflow_overrides.up.sql
0021_template_workflow_override_permissions.up.sql
0022_reminder_runtime_tables.up.sql
0023_deadline_config_and_company_established_date.up.sql
0024_holiday_calendars.up.sql
0025_user_invitations.up.sql
0009_seed_authz_test_accounts.up.sql
0010_disclosure_contract_c1.up.sql
0026_self_registration_owner_role.up.sql
0027_company_verification_email_otp.up.sql
0029_company_profile_fields.up.sql
0030_disclosure_template_block_display_names.up.sql
0031_seed_dev_invite_accept_fixture.up.sql
0032_customize_workflow_extension.up.sql
0033_smoke_workflow_dev_seed.up.sql
0034_seed_org_structure_demo.up.sql
0035_disclosure_display_groups.up.sql
0036_fix_unicode_mojibake.up.sql
0037_adhoc_admin_approve_final_fields.up.sql
0040_company_type_preferences.up.sql
0041_adhoc_admin_approve_progress.up.sql
0042_adhoc_process_controller.up.sql
0043_membership_direct_permissions.up.sql
0044_role_default_grant_permissions.up.sql
0045_backfill_workflow_read_permission.up.sql
0046_add_is_primary_admin.up.sql
0047_extend_departments.up.sql
0048_extend_titles.up.sql
0049_seed_org_roles.up.sql
0050_org_units_department_link.up.sql
0051_email_notifications.up.sql
0052_email_delivery_attempts.up.sql
0053_cms_portal_template_tables.up.sql
0054_cms_display_groups_po_seed.up.sql
0055_cms_system_template_seed.up.sql
0056_company_template_lifecycle.up.sql
0057_workflow_override_versioning.up.sql
0058_cms_template_permissions.up.sql
0059_global_workflows.up.sql
0060_deadline_rule_catalog.up.sql
0061_dev_ad_hoc_propose_admin_cms.up.sql
0062_admin_doanh_nghiep_process_control.up.sql
0063_dev_platform_tenant_dual_admin.up.sql
0064_platform_tenant_admin_process_control.up.sql
0065_adhoc_proposed_deadline_days.up.sql
0066_dev_company_profile_permissions.up.sql
0067_dev_company_profile_permissions_fix.up.sql
0068_fix_action_policy_company_profile.up.sql
0069_template_display_groups_backfill.up.sql
0070_prune_legacy_display_groups.up.sql
0071_cms_template_write_from_platform_cms_view.up.sql
0072_fix_display_groups_vietnamese_labels.up.sql
0073_self_reg_company_profile_permissions.up.sql
0074_deadline_view_permission.up.sql
0075_fix_unicode_mojibake_disclosure_text.up.sql
0076_deadline_alert_confirmations.up.sql
0077_admin_membership_invite_permission.up.sql
0078_dev_subscription_expiry_seed.up.sql
0079_disclosure_auto_create_manage_permission.up.sql
0080_periodic_cycles_cycle_start.up.sql
0081_user_avatar.up.sql
0082_companies_self_service_provisioning.up.sql
0083_company_type_preference_cycle_anchor.up.sql
0084_alert_template_configs.up.sql
0085_user_in_app_notifications.up.sql
0086_reminder_scope_id_expand.up.sql
0087_backfill_admin_role_disclosure_type_manage.up.sql
0088_revoke_cms_template_perms_from_enterprise_admin.up.sql
0089_expand_override_id_varchar.up.sql
0090_company_template_publish_permission.up.sql
"

mysql_exec() {
  MYSQL_PWD="${DB_PASSWORD}" mysql --default-character-set=utf8mb4 -h "${DB_HOST}" -u"${DB_USER}" "${DB_NAME}" "$@"
}

mysql_server_exec() {
  MYSQL_PWD="${DB_PASSWORD}" mysql --default-character-set=utf8mb4 -h "${DB_HOST}" -u"${DB_USER}" "$@"
}

echo "Ensuring database ${DB_NAME} exists..."
mysql_server_exec <<SQL
CREATE DATABASE IF NOT EXISTS \`${DB_NAME}\`
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;
SQL

echo "Ensuring schema_migrations table exists..."
mysql_exec <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
  file_name VARCHAR(255) PRIMARY KEY,
  executed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
SQL

echo "Ensuring auth plugin is caching_sha2_password..."
if mysql_exec <<'SQL'
ALTER USER IF EXISTS 'root'@'localhost' IDENTIFIED WITH caching_sha2_password BY 'root';
ALTER USER IF EXISTS 'root'@'%' IDENTIFIED WITH caching_sha2_password BY 'root';
ALTER USER IF EXISTS 'cobo'@'%' IDENTIFIED WITH caching_sha2_password BY 'cobo';
SQL
then
  echo "Auth plugin check/update complete."
else
  echo "WARN: skipping auth plugin update; current DB user lacks ALTER USER / CREATE USER privileges."
fi

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
  if ! mysql_exec < "migrations/${file}"; then
    echo "ERROR: migration failed: ${file}" >&2
    exit 1
  fi
  mysql_exec -e "INSERT INTO schema_migrations(file_name) VALUES ('${file}')"
done

echo "All migrations are up to date."
