-- M2: multi-assignee workflow tasks (schema source only — apply in M4).
-- v3 authority: workflow_task_assignees; singular assignee_membership_id becomes nullable legacy field.
-- No backfill of historical tasks into the relation table.

ALTER TABLE workflow_tasks
  MODIFY COLUMN assignee_membership_id VARCHAR(36) NULL;

CREATE TABLE IF NOT EXISTS workflow_task_assignees (
  task_id       VARCHAR(36) NOT NULL,
  membership_id VARCHAR(36) NOT NULL,
  created_at    TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (task_id, membership_id),
  KEY idx_workflow_task_assignees_membership (membership_id),
  CONSTRAINT fk_workflow_task_assignees_task
    FOREIGN KEY (task_id) REFERENCES workflow_tasks(task_id) ON DELETE CASCADE,
  CONSTRAINT fk_workflow_task_assignees_membership
    FOREIGN KEY (membership_id) REFERENCES memberships(membership_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
