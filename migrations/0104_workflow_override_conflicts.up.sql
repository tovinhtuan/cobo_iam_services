-- 0104: Sprint 3 / Batch 4 — Workflow Override Conflict Detection.
--
-- Scope (locked, per docs/ai-cache/workflow-override-foundation-batch4/PREFLIGHT_AUDIT.md):
--   - ONE new table, workflow_override_conflicts. No existing table touched at all by this file.
--   - Conflicts are metadata only: never read by GetEffectiveWorkflow, never gate runtime.
--   - No FK (matches this codebase's existing override-table convention — see
--     company_template_workflow_overrides / migrations/0103, neither of which uses an FK either).
--
-- Data safety: ADDITIVE ONLY.
--   - No existing column/table dropped, renamed, or modified.
--   - company_template_workflow_overrides / _versions, global_workflows / _versions / _steps,
--     workflow_instances, workflow_tasks, disclosure_template_blocks are NEVER touched by this file.
-- Idempotency: CREATE TABLE IF NOT EXISTS — re-running is a safe no-op once the table exists.
-- Reversible: see 0104_workflow_override_conflicts.down.sql (DROP TABLE — no other table
--   references this one, so the drop is unconditionally safe).

SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS workflow_override_conflicts (
  id                  VARCHAR(64)  NOT NULL,
  company_id          VARCHAR(64)  NOT NULL,
  type_id             VARCHAR(64)  NOT NULL,
  override_id         VARCHAR(128) NULL,
  override_version_no INT          NULL,
  preview_id          VARCHAR(64)  NULL,
  base_version_no     INT          NOT NULL,
  target_version_no   INT          NOT NULL,
  conflict_key        VARCHAR(255) NOT NULL,
  step_key            VARCHAR(128) NOT NULL,
  field_path          VARCHAR(128) NOT NULL,
  severity            VARCHAR(16)  NOT NULL,
  conflict_type       VARCHAR(64)  NOT NULL,
  global_old_json     JSON         NULL,
  global_new_json     JSON         NULL,
  company_value_json  JSON         NULL,
  resolution_status    VARCHAR(16)  NOT NULL DEFAULT 'unresolved',
  resolution          VARCHAR(32)  NULL,
  resolution_json      JSON         NULL,
  created_by          VARCHAR(64)  NOT NULL,
  created_at          TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  resolved_by         VARCHAR(64)  NULL,
  resolved_at         DATETIME     NULL,
  updated_at          TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_workflow_override_conflicts_conflict_key (conflict_key),
  KEY idx_workflow_override_conflicts_company_type (company_id, type_id),
  KEY idx_workflow_override_conflicts_preview_id (preview_id),
  KEY idx_workflow_override_conflicts_resolution_status (resolution_status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Record in migration tracker (matches 0091/0100/0101/0102/0103 self-recording pattern).
INSERT INTO schema_migrations (file_name) VALUES ('0104_workflow_override_conflicts.up.sql')
  ON DUPLICATE KEY UPDATE executed_at = CURRENT_TIMESTAMP;
