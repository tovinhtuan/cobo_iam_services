-- ⚠️ SAFETY: This rollback is PERMANENTLY NON-EXECUTABLE once override data exists.
-- Generated override_id values are "ovr_{UUID(36)}_{type_id(N)}" = 66+ chars.
-- Shrinking back to VARCHAR(64) will fail with "Data too long" for any existing override rows.
--
-- PRE-CHECK (run before attempting rollback):
-- SELECT COUNT(*) FROM company_template_workflow_overrides WHERE CHAR_LENGTH(override_id) > 64;
-- SELECT COUNT(*) FROM company_template_workflow_override_versions WHERE CHAR_LENGTH(override_id) > 64;
-- If either count > 0, DO NOT run this rollback — it will fail and corrupt primary keys.

ALTER TABLE company_template_workflow_overrides
  MODIFY COLUMN override_id VARCHAR(64) NOT NULL;

ALTER TABLE company_template_workflow_override_versions
  MODIFY COLUMN override_id VARCHAR(64) NOT NULL;
