SELECT file_name FROM schema_migrations WHERE file_name LIKE "%0128%" OR file_name LIKE "%workflow_task_assignees%";
SELECT COLUMN_NAME, IS_NULLABLE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA="cobo_iam" AND TABLE_NAME="workflow_tasks" AND COLUMN_NAME="assignee_membership_id";
SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA="cobo_iam" AND TABLE_NAME="workflow_task_assignees";
SELECT MAX(file_name) FROM schema_migrations;
