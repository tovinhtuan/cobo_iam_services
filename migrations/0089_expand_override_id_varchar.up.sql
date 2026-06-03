-- override_id is generated as "ovr_{company_id(36)}_{type_id(N)}" where company_id is a UUID (36 chars)
-- and type_id can be up to 64 chars. Total: 4+36+1+64 = 105 chars — exceeds VARCHAR(64).
ALTER TABLE company_template_workflow_overrides
  MODIFY COLUMN override_id VARCHAR(128) NOT NULL;

ALTER TABLE company_template_workflow_override_versions
  MODIFY COLUMN override_id VARCHAR(128) NOT NULL;
