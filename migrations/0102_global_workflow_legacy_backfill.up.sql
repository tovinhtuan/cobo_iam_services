-- 0102: mig-R2 — Backfill legacy enterprise_workflow content blocks into governed
-- global_workflows / global_workflow_steps / global_workflow_versions (v1, active).
--
-- Context: REMEDIATION_IMPLEMENTATION_PLAN.md Batch R2 / ADR_WORKFLOW_DATA_SOURCE_ALIGNMENT.md
-- Option C. Live DEV count at authoring time: 27 disclosure_types have an enterprise_workflow
-- content block; 25 of those have ZERO presence in global_workflows. Batch R1 made
-- GetEffectiveWorkflow read an active global_workflow_versions row when one exists, but R1 alone
-- cannot help a type that has no global_workflows row at all — this migration creates one for
-- each, replicating the legacy steps verbatim so day-one runtime behavior is unchanged (the
-- backfilled v1 reproduces the same steps the legacy fallback already returned).
--
-- Data safety: ADDITIVE ONLY, idempotent.
--   - Skips any type_id that ALREADY has a global_workflows row (any status — uq_global_workflow_type
--     guarantees at most one row per type anyway), covering both the 4 pre-existing types and any
--     type a Platform Admin has independently started configuring since Sprint 0-2 shipped.
--   - Never touches disclosure_template_blocks (legacy block stays exactly as-is, read-only
--     historical record per ADR) — read-only SELECT only.
--   - Never touches workflow_instances / workflow_tasks / company_template_workflow_overrides*.
--   - Backfilled version is inserted directly as state='active' (mirrors mig-0101's treatment of
--     the original 4 types) — no fake publish event; published_by/activated_by = 'system'.
--
-- Format fix (incidental, scoped to NEW rows only): mig-0101's own v1 backfill wrote
-- steps_manifest_json as a BARE step array, which differs from the {type_id,workflow_id,
-- version_no,steps:[...]} envelope the real Publish() flow writes (confirmed live on DEV: type
-- dt-sys-board-resolution's active v1 row is a bare array). Batch R1's reader
-- (decodeGlobalWorkflowManifestSteps) tolerates both shapes, but THIS migration writes the
-- correct envelope shape for its own new rows so it does not perpetuate the inconsistency.
--
-- Known, disclosed limitation: each step's steps_manifest_json.role keeps only the FIRST id of
-- the legacy step's assignee_role_ids array — this matches the new system's own existing
-- single-role-per-step manifest convention (see BuildManifest() in
-- internal/workflowconfig/infra/mysql/global_workflow_version_repository.go, `st.Role =
-- firstRole(roleJSON)`), not a limitation introduced by this migration. The FULL
-- assignee_role_ids array is preserved verbatim in global_workflow_steps.assignee_role_ids (the
-- editable Builder draft), so a Platform Admin who opens Builder sees and can re-publish all
-- original roles; only the immutable v1 snapshot mirrors the new system's single-role convention.
--
-- Role ids are passed through UNCHANGED (e.g. "role-reviewer"), per ADR known risk #2 — the Role
-- Registry's alias map already resolves these (role_seed.go); rewriting them is unnecessary and
-- would risk losing the originally authored value.
--
-- documents are carried into the existing nullable global_workflow_steps.documents_json column
-- (added in mig-0059; this migration does not add any new column) — ADR known risk #1.
--
-- Idempotency: every INSERT is guarded by NOT EXISTS / a join to step 1's freshly-created rows
-- only (identified by created_by='system' AND change_note='backfill v1 (legacy
-- enterprise_workflow)', a tag unique to this migration). Re-running is a safe no-op.

SET NAMES utf8mb4;

-- ─── 1. one global_workflows row per eligible legacy type ───
INSERT INTO global_workflows
  (workflow_id, type_id, status, change_note, created_by, updated_by, created_at, updated_at,
   published_version_no, active_version_no)
SELECT UUID(), b.type_id, 'active', 'backfill v1 (legacy enterprise_workflow)', 'system', 'system',
       NOW(), NOW(), 1, 1
FROM (
  SELECT DISTINCT tb.type_id
  FROM disclosure_template_blocks tb
  JOIN disclosure_types dt ON dt.type_id = tb.type_id AND dt.active_version_no = tb.version_no
  WHERE tb.block_key = 'enterprise_workflow'
    AND JSON_LENGTH(COALESCE(JSON_EXTRACT(tb.config_json, '$.steps'), JSON_ARRAY())) > 0
) b
WHERE NOT EXISTS (SELECT 1 FROM global_workflows gw WHERE gw.type_id = b.type_id);

-- ─── 2. explode each legacy step into global_workflow_steps (only for workflows just created above) ───
INSERT INTO global_workflow_steps
  (step_id, workflow_id, step_key, stage, department_id, assignee_role_ids, due_rule,
   processing_days, display_order, documents_json, created_at)
SELECT
  COALESCE(jt.step_id_a, jt.step_id_b, UUID())                    AS step_id,
  gw.workflow_id,
  COALESCE(jt.step_id_a, jt.step_id_b, UUID())                    AS step_key,
  COALESCE(jt.stage, '')                                          AS stage,
  COALESCE(jt.department_id, '')                                  AS department_id,
  COALESCE(jt.assignee_role_ids, JSON_ARRAY())                    AS assignee_role_ids,
  COALESCE(jt.due_rule, '')                                       AS due_rule,
  COALESCE(jt.processing_days, 0)                                 AS processing_days,
  COALESCE(jt.display_order, jt.rownum)                           AS display_order,
  jt.documents                                                    AS documents_json,
  NOW()
FROM disclosure_template_blocks tb
JOIN disclosure_types dt ON dt.type_id = tb.type_id AND dt.active_version_no = tb.version_no
JOIN global_workflows gw
  ON gw.type_id = tb.type_id
 AND gw.created_by = 'system'
 AND gw.change_note = 'backfill v1 (legacy enterprise_workflow)'
JOIN JSON_TABLE(
  tb.config_json, '$.steps[*]'
  COLUMNS (
    rownum            FOR ORDINALITY,
    step_id_a         VARCHAR(64) PATH '$.step_id',
    step_id_b         VARCHAR(64) PATH '$.id',
    stage             VARCHAR(255) PATH '$.stage',
    department_id     VARCHAR(64) PATH '$.department_id',
    assignee_role_ids JSON        PATH '$.assignee_role_ids',
    due_rule          VARCHAR(64) PATH '$.due_rule',
    processing_days   INT         PATH '$.processing_days',
    display_order     INT         PATH '$.display_order',
    documents         JSON        PATH '$.documents'
  )
) AS jt
WHERE tb.block_key = 'enterprise_workflow'
  AND NOT EXISTS (SELECT 1 FROM global_workflow_steps gws WHERE gws.workflow_id = gw.workflow_id);

-- ─── 3. immutable v1 manifest snapshot, written in the correct envelope shape ───
INSERT INTO global_workflow_versions
  (type_id, version_no, state, steps_manifest_json, change_note, published_at, published_by,
   activated_at, activated_by)
SELECT
  gw.type_id, 1, 'active',
  JSON_OBJECT(
    'type_id', gw.type_id,
    'workflow_id', gw.workflow_id,
    'version_no', 1,
    'steps', (
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
    )
  ),
  'backfill v1 (legacy enterprise_workflow)', NOW(), 'system', NOW(), 'system'
FROM global_workflows gw
WHERE gw.created_by = 'system'
  AND gw.change_note = 'backfill v1 (legacy enterprise_workflow)'
  AND NOT EXISTS (
    SELECT 1 FROM global_workflow_versions v WHERE v.type_id = gw.type_id AND v.version_no = 1
  );

-- Record in migration tracker (matches 0091/0100/0101 self-recording pattern).
INSERT INTO schema_migrations (file_name) VALUES ('0102_global_workflow_legacy_backfill.up.sql')
  ON DUPLICATE KEY UPDATE executed_at = CURRENT_TIMESTAMP;
