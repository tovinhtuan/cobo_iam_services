ALTER TABLE company_template_workflow_overrides
  MODIFY COLUMN override_id VARCHAR(64) NOT NULL;

ALTER TABLE company_template_workflow_override_versions
  MODIFY COLUMN override_id VARCHAR(64) NOT NULL;
