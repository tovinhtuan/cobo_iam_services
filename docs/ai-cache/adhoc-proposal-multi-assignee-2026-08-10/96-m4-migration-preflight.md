# M4 migration 0128 preflight

1. Ledger latest: `0127_adhoc_proposed_deadline_day_type.up.sql`
2. 0128 pending: YES (count=0)
3. Other pending on disk beyond applied: only 0128 (new source); no unrelated pending ledger gap
4. Full `run_dev_migrations` would apply all pending — NOT used
5. Isolated apply: `make push-migration FILE=0128_workflow_task_assignees.up.sql` (single-file SQL + schema_migrations INSERT IGNORE)
6. `workflow_task_assignees` existed unexpectedly? NO
7. `assignee_membership_id` already nullable? NO (IS_NULLABLE=NO)
8. Down migration: compatibility-safe no-op (`SELECT 1`) — does not restore NOT NULL / drop table

Lock-risk: MySQL 8.0.46 InnoDB, 84 rows, DATA_LENGTH≈16KB — MODIFY NULL + CREATE TABLE considered low risk for DEV.
