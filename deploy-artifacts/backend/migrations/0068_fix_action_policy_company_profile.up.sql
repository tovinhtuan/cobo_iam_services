-- 0068: Fix action_policy_matrix when company.view/company.edit require system.settings (causes 403 despite DB grants).
-- No-op if table does not exist (dev without matrix still uses legacyPolicy in API).

SET NAMES utf8mb4;

SET @has_apm := (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 'action_policy_matrix'
);

SET @sql_view := IF(
  @has_apm > 0,
  'UPDATE action_policy_matrix SET required_permission = ''company.view'', scope_type = ''*'', workflow_state = ''*'', eligible_actor = ''*'', effect_type = ''allow'', deny_reason_code = ''permission_denied'', status = ''active'' WHERE action_code = ''company.view''',
  'SELECT 1'
);
PREPARE stmt_view FROM @sql_view;
EXECUTE stmt_view;
DEALLOCATE PREPARE stmt_view;

SET @sql_edit := IF(
  @has_apm > 0,
  'UPDATE action_policy_matrix SET required_permission = ''company.edit'', scope_type = ''*'', workflow_state = ''*'', eligible_actor = ''*'', effect_type = ''allow'', deny_reason_code = ''permission_denied'', status = ''active'' WHERE action_code = ''company.edit''',
  'SELECT 1'
);
PREPARE stmt_edit FROM @sql_edit;
EXECUTE stmt_edit;
DEALLOCATE PREPARE stmt_edit;
