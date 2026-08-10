# M2 — migration design
1. MODIFY assignee_membership_id NULL
2. CREATE workflow_task_assignees (PK task_id+membership_id, idx membership_id)
3. No backfill
4. Down: safe no-op (keep nullable + table)
Markers: V2_TASK_SINGULAR_AUTHORITY, V3_TASK_RELATION_AUTHORITY, NO_DUAL_TASK_ASSIGNMENT_AUTHORITY, MIGRATION_SOURCE_CREATED_NOT_APPLIED, ROLLBACK_SCHEMA_COMPATIBILITY_DOCUMENTED
