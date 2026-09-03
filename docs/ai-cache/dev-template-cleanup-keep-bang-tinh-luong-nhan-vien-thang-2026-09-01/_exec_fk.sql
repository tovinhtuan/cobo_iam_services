SELECT CONSTRAINT_NAME, TABLE_NAME, REFERENCED_TABLE_NAME, DELETE_RULE
FROM information_schema.REFERENTIAL_CONSTRAINTS
WHERE CONSTRAINT_SCHEMA='cobo_iam'
  AND REFERENCED_TABLE_NAME IN (
    'disclosure_types','disclosure_type_versions','disclosure_records',
    'workflow_instances','workflow_tasks','company_template_workflow_overrides',
    'global_workflows','periodic_cycles'
  )
ORDER BY REFERENCED_TABLE_NAME, TABLE_NAME;
