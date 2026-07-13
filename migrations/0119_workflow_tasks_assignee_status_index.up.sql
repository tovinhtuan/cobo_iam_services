-- Personal ops mine queries filter workflow_tasks by assignee_membership_id + status.
-- Additive index only; no column/data change. Safe for mixed-version deploy.

CREATE INDEX idx_workflow_tasks_assignee_status
  ON workflow_tasks (assignee_membership_id, status);
