-- Allow template-level default department codes (up to 64 chars) in global workflow steps.

ALTER TABLE global_workflow_steps
  MODIFY COLUMN department_id VARCHAR(64) NOT NULL DEFAULT '';
