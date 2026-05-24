DELETE FROM membership_direct_permissions
WHERE granted_by = 'system_migration_0064'
  AND permission_code = 'ad_hoc_alert.process_control'
  AND membership_id IN ('m_107', 'm_108');
