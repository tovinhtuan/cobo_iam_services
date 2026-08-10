# M2 — task schema audit
- Pre: workflow_tasks.assignee_membership_id VARCHAR(36) NOT NULL; no workflow_task_assignees
- Unique/FKs: task_id PK; assignee NOT NULL historically
- Decision: nullable singular + relation table (NO_V3_FIRST_ASSIGNEE_SHADOW_AUTHORITY)
