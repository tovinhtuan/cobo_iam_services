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
0091_fix_global_workflow_schema.up.sql
0092_dev_premium_subscription_tvttthptlvh.up.sql
0093_company_applicability_profile.up.sql
0094_template_applicability_rules.up.sql
0095_backfill_template_applicability_rules.up.sql
0097_deadline_engine_v2_prepare.up.sql
0100_workflow_step_key.up.sql
0101_global_workflow_versions.up.sql
0102_global_workflow_legacy_backfill.up.sql
0103_override_base_metadata.up.sql
0104_workflow_override_conflicts.up.sql
0105_deactivate_empty_active_overrides.up.sql
0106_config_versioning.up.sql
0107_pending_admin_changes.up.sql
0108_delegated_admin_grants.up.sql
0109_emergency_access_grants.up.sql
0110_cleanup_enterprise_rbac_remaining_blocked_perms.up.sql
0111_dev_deadline_alert_scope_qa_seed.up.sql
0112_workflow_instance_step_runtime.up.sql
0113_department_memberships_focal.up.sql
0114_workflow_template_departments.up.sql
0115_widen_global_workflow_department_id.up.sql
0116_workflow_assignee_role_catalog.up.sql
0117_fix_workflow_template_departments_unicode.up.sql
0118_platform_subscription_upgrade_payment.up.sql
0119_workflow_tasks_assignee_status_index.up.sql
0120_disclosure_records_completed_at.up.sql
0121_workflow_step_description.up.sql
0123_roles_classification.up.sql
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

column_exists() {
  table="$1"
  column="$2"
  mysql_exec -Nse \
    "SELECT COUNT(1) FROM information_schema.columns \
     WHERE table_schema='${DB_NAME}' AND table_name='${table}' AND column_name='${column}'"
}

# Preflight ledger-only when schema already matches target (drift from manual/partial apply).
preflight_schema_drift() {
  file="$1"
  case "${file}" in
    0091_fix_global_workflow_schema.up.sql)
      if [ "$(column_exists global_workflows status)" -gt 0 ]; then
        echo "Preflight: ${file} — global_workflows.status exists; marking ledger only"
        mysql_exec -e "INSERT IGNORE INTO schema_migrations(file_name) VALUES ('${file}')"
        return 0
      fi
      ;;
    0093_company_applicability_profile.up.sql)
      if [ "$(column_exists companies is_listed)" -gt 0 ]; then
        echo "Preflight: ${file} — companies.is_listed exists; marking ledger only"
        mysql_exec -e "INSERT IGNORE INTO schema_migrations(file_name) VALUES ('${file}')"
        return 0
      fi
      ;;
    0094_template_applicability_rules.up.sql)
      if [ "$(column_exists disclosure_type_versions applicability_rules_json)" -gt 0 ]; then
        echo "Preflight: ${file} — disclosure_type_versions.applicability_rules_json exists; marking ledger only"
        mysql_exec -e "INSERT IGNORE INTO schema_migrations(file_name) VALUES ('${file}')"
        return 0
      fi
      ;;
  esac
  return 1
}

for file in ${MIGRATIONS}; do
  applied="$(mysql_exec -Nse "SELECT COUNT(1) FROM schema_migrations WHERE file_name='${file}'")"
  if [ "${applied}" -gt 0 ]; then
    echo "Skipping already applied migration: ${file}"
    continue
  fi

  if preflight_schema_drift "${file}"; then
    continue
  fi

  echo "Applying migration: ${file}"
  if ! mysql_exec < "migrations/${file}"; then
    echo "ERROR: migration failed: ${file}" >&2
    exit 1
  fi
  # IGNORE: 0091 self-records in SQL; push-migration.sh uses the same pattern.
  mysql_exec -e "INSERT IGNORE INTO schema_migrations(file_name) VALUES ('${file}')"
done

echo "All migrations are up to date."
