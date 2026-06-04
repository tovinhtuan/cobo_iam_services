-- 0091: Fix global_workflow schema drift from 0053 ↔ 0059 migration conflict.
--
-- Root cause: migration 0053 created global_workflows + global_workflow_steps with schema A.
-- Migration 0059 used CREATE TABLE IF NOT EXISTS — no-op when tables already exist.
-- The Go code (cms_repository.go) expects schema B (0059). This brings A → B.
--
-- Data safety: 3 workflow rows + 10 step rows preserved with best-effort mapping.
-- NOT idempotent: will fail with duplicate column error if run a second time.
-- Forward-only: no down migration (column drops are irreversible).
--
-- NOTE: MySQL 8.0 does NOT support ADD COLUMN IF NOT EXISTS (MariaDB-only).
-- This migration assumes 0053 schema is in place (as on all known environments).

SET NAMES utf8mb4;

-- ─── global_workflows: add missing columns ────────────────────────────────────

ALTER TABLE global_workflows
  ADD COLUMN status      VARCHAR(32)  NOT NULL DEFAULT 'active' AFTER type_id,
  ADD COLUMN change_note TEXT          NULL                      AFTER status,
  ADD COLUMN updated_by  VARCHAR(36)  NOT NULL DEFAULT ''        AFTER change_note;

-- Migrate is_active → status.
UPDATE global_workflows SET status = IF(is_active = 1, 'active', 'archived');

-- Unique key: each template has at most one workflow header row.
ALTER TABLE global_workflows
  ADD UNIQUE KEY uq_global_workflow_type (type_id);


-- ─── global_workflow_steps: add new columns ───────────────────────────────────

ALTER TABLE global_workflow_steps
  ADD COLUMN stage             VARCHAR(255)  NOT NULL DEFAULT '' AFTER display_order,
  ADD COLUMN department_id     VARCHAR(36)   NOT NULL DEFAULT '' AFTER stage,
  ADD COLUMN assignee_role_ids JSON          NULL               AFTER department_id,
  ADD COLUMN due_rule          VARCHAR(64)   NOT NULL DEFAULT '' AFTER assignee_role_ids,
  ADD COLUMN processing_days   INT           NOT NULL DEFAULT 0  AFTER due_rule,
  ADD COLUMN documents_json    JSON          NULL               AFTER processing_days;

-- Migrate data: old columns → new columns.
UPDATE global_workflow_steps SET
  stage             = stage_name,
  department_id     = LEFT(department, 36),
  assignee_role_ids = JSON_ARRAY(),
  due_rule          = step_deadline,
  processing_days   = 0;

-- Drop indexes on type_id before dropping the column.
ALTER TABLE global_workflow_steps DROP INDEX idx_gws_type;
ALTER TABLE global_workflow_steps DROP INDEX idx_gws_has_workflow_check;

-- Drop old columns.
ALTER TABLE global_workflow_steps
  DROP COLUMN stage_name,
  DROP COLUMN department,
  DROP COLUMN step_deadline,
  DROP COLUMN type_id;

-- Add FK CASCADE (0059 schema requirement).
ALTER TABLE global_workflow_steps
  ADD CONSTRAINT fk_gwf_steps_workflow
    FOREIGN KEY (workflow_id) REFERENCES global_workflows (workflow_id) ON DELETE CASCADE;

-- Record in migration tracker.
INSERT INTO schema_migrations (file_name) VALUES ('0091_fix_global_workflow_schema.up.sql')
  ON DUPLICATE KEY UPDATE executed_at = CURRENT_TIMESTAMP;
