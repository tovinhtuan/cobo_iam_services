# Workflow & Step ID Materialization

## Step ID Strategy
- `STEP_ID_NORMALIZATION_STRATEGY = SERVER_OWNED_UUID`
- `FINAL_STEP_ID_GENERATED_AT = CONFIRM_MATERIALIZATION`
- `STEP_ID_CROSS_REFERENCE_REMAP = NOT_APPLICABLE` (workflow steps are sequential stages without branching step-to-step edge graphs)

Source step IDs are untrusted and discarded. During Confirm materialization, fresh server-owned UUIDs are generated for each step using `s.idg.NewUUID()` (UUIDv7 generator).

## Workflow Business Content Preserved
- `stage`: Name of workflow stage (e.g. "Lập báo cáo", "Phê duyệt").
- `description`: Stage description.
- `instructions`: Guidelines and instructions for assignees.
- `assignee_role_ids`: Roles required for assignment (e.g. `["creator"]`, `["approver"]`).
- `department_id`: Resolved target department code (e.g. `dept-001`).
- `due_rule`: Relative deadline rule (e.g. `T+10`, `48h`).
- `processing_days`: Estimated processing duration in calendar days.
- `display_order`: Preserved sequential ordering 1..N.
- `reminder_config`: Converted to `WorkflowStepReminderConfig` (`mode: "days_before"`, `days_before: offsets`).
- `documents`: Document requirements.

## Runtime Data Discarded
- `tenant_membership_id`: Empty string.
- `tenant_membership_ids`: Nil/empty.
- No runtime tasks, milestones, or execution instances created.
