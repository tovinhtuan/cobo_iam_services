-- Rollback note (safe additive retention):
-- Do NOT restore assignee_membership_id NOT NULL while v3 tasks may store NULL.
-- Do NOT drop workflow_task_assignees if active v3 assignment rows may exist.
-- Application rollback keeps nullable singular + relation table; old v2 app tolerates additive schema.
-- Destructive down is intentionally omitted beyond a no-op comment for operator safety.

SELECT 1;
