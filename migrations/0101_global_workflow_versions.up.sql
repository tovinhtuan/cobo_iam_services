-- 0101: mig-S2 — Global Workflow Versioning (publish ≠ activate).
--
-- Baseline: WORKFLOW_CONFIGURATION_BASELINE_V1 / Phase 13 (global_workflow_versions, published≠active).
-- Anchor = type_id (STABLE). Rationale: global_workflows.workflow_id is regenerated on every save
-- (cms_service mints a new UUID + repo DELETE+INSERT), so it cannot anchor versions; type_id is the
-- 1:1 stable key (same anchoring the override system uses).
--
-- Data safety: ADDITIVE only. New table + 2 nullable pointer columns + a guarded v1 backfill.
--   - No row/step deleted. No tenant table touched (company_template_workflow_override*).
--   - No change to workflow_instances / workflow_tasks / runtime snapshot.
-- Idempotency: CREATE TABLE IF NOT EXISTS; columns guarded via information_schema; backfill via NOT EXISTS.

SET NAMES utf8mb4;

-- ─── 1. version table (immutable snapshots; source of truth for state) ───
CREATE TABLE IF NOT EXISTS global_workflow_versions (
  type_id             VARCHAR(64)  NOT NULL,
  version_no          INT          NOT NULL,
  state               VARCHAR(16)  NOT NULL DEFAULT 'draft',  -- draft|published|active|archived
  steps_manifest_json JSON         NOT NULL,
  change_note         TEXT         NULL,
  published_at        DATETIME     NULL,
  published_by        VARCHAR(36)  NULL,
  activated_at        DATETIME     NULL,
  activated_by        VARCHAR(36)  NULL,
  created_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (type_id, version_no),
  KEY idx_gwv_type_state (type_id, state, version_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ─── 2. pointer columns on global_workflows (published ≠ active) ───
SET @c1 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='global_workflows' AND column_name='published_version_no');
SET @sql = IF(@c1=0, 'ALTER TABLE global_workflows ADD COLUMN published_version_no INT NULL', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c2 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='global_workflows' AND column_name='active_version_no');
SET @sql = IF(@c2=0, 'ALTER TABLE global_workflows ADD COLUMN active_version_no INT NULL', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

-- ─── 3. backfill version 1 from current active workflows (immutable snapshot) ───
INSERT INTO global_workflow_versions
  (type_id, version_no, state, steps_manifest_json, change_note, published_at, published_by, activated_at, activated_by)
SELECT
  gw.type_id, 1, 'active',
  COALESCE((
    SELECT JSON_ARRAYAGG(JSON_OBJECT(
      'step_id',         s.step_id,
      'step_key',        s.step_key,
      'stage',           s.stage,
      'name',            s.stage,
      'role',            JSON_UNQUOTE(JSON_EXTRACT(s.assignee_role_ids, '$[0]')),
      'department_id',   s.department_id,
      'due_rule',        s.due_rule,
      'processing_days', s.processing_days,
      'display_order',   s.display_order
    ))
    FROM global_workflow_steps s
    WHERE s.workflow_id = gw.workflow_id
  ), JSON_ARRAY()),
  'backfill v1', NOW(), gw.created_by, NOW(), gw.created_by
FROM global_workflows gw
WHERE gw.status = 'active'
  AND NOT EXISTS (
    SELECT 1 FROM global_workflow_versions v WHERE v.type_id = gw.type_id AND v.version_no = 1
  );

-- ─── 4. set pointers for backfilled active workflows ───
UPDATE global_workflows gw
SET gw.published_version_no = 1, gw.active_version_no = 1
WHERE gw.status = 'active'
  AND (gw.published_version_no IS NULL OR gw.active_version_no IS NULL)
  AND EXISTS (SELECT 1 FROM global_workflow_versions v WHERE v.type_id = gw.type_id AND v.version_no = 1);

-- Record in migration tracker (matches 0091/0100 self-recording pattern).
INSERT INTO schema_migrations (file_name) VALUES ('0101_global_workflow_versions.up.sql')
  ON DUPLICATE KEY UPDATE executed_at = CURRENT_TIMESTAMP;
