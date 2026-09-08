# Runtime & Company Data Isolation

## Complete Isolation of Import Materialization
Importing a template is purely an authoring activity within the CMS platform realm. It must never touch runtime execution records or tenant configuration.

## Verification
- `COMPANY_OVERRIDE_WRITE_COUNT = 0`:
  - Zero rows written to `company_type_preferences`.
  - Zero rows written to `company_schedule_overrides`.
  - Zero rows written to `company_workflow_overrides`.
- `RUNTIME_WRITE_COUNT = 0`:
  - Zero rows written to `periodic_cycles`.
  - Zero rows written to `disclosure_records`.
  - Zero rows written to `workflow_instances`.
  - Zero rows written to `workflow_tasks`.
  - Zero rows written to `workflow_step_milestones`.
  - Zero rows written to `deadline_alerts`.
  - Zero rows written to reminder queues or mail history.
- `ASSIGNEE_MEMBERSHIP_IMPORTED = false`:
  - `AssigneeMembershipID` and `AssigneeMembershipIDs` are strictly cleared / empty.
