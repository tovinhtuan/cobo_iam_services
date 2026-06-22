-- 0103: Sprint 3 / Batch 1 — Workflow Override Base Metadata Foundation.
--
-- Scope (locked, per docs/ai-cache/workflow-override-foundation-adr/IMPLEMENTATION_BATCH_PLAN.md
-- Batch 1 and docs/ai-cache/workflow-override-foundation-batch1/PREFLIGHT_AUDIT.md):
--   - Additive metadata columns on company_template_workflow_overrides ONLY.
--   - Best-effort backfill of base_source/base_version_no/base_workflow_id where DETERMINISTICALLY
--     correlatable; everything else stays 'unknown'/NULL — never guessed (see BACKFILL_REPORT.md).
--   - base_hash is intentionally left NULL for every row — see HASH_CONTRACT.md §6: a deterministic
--     canonical-JSON hash cannot be guaranteed in SQL, so it is not computed here.
--   - stale_status defaults to 'unknown' for every row, including ones with a determinable base —
--     Batch 2 owns the comparator that turns this into 'current'/'stale'. This migration only adds
--     the column and never sets it to anything but 'unknown'.
--
-- Data safety: ADDITIVE ONLY.
--   - No existing column dropped, renamed, or modified.
--   - workflow_json (on company_template_workflow_override_versions) is NEVER touched by this file.
--   - active_version_no, status on company_template_workflow_overrides are NEVER touched by this file.
--   - No row is deleted. No FK added (avoids any lock/constraint risk on existing data).
-- Idempotency: ALTER TABLE guarded via information_schema (MySQL 8.0 has no ADD COLUMN IF NOT
--   EXISTS). The backfill UPDATE is idempotent by construction — it is a pure function of
--   immutable historical timestamps (override.created_at never changes after creation;
--   global_workflow_versions.activated_at never changes once set), so re-running it converges to
--   the identical result, not just a no-op skip.
-- Reversible: see 0103_override_base_metadata.down.sql.

SET NAMES utf8mb4;

-- ─── 1. additive columns (guarded) ───
SET @c1 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_overrides' AND column_name='base_source');
SET @sql = IF(@c1=0, "ALTER TABLE company_template_workflow_overrides ADD COLUMN base_source VARCHAR(32) NOT NULL DEFAULT 'unknown' AFTER status", 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c2 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_overrides' AND column_name='base_workflow_id');
SET @sql = IF(@c2=0, 'ALTER TABLE company_template_workflow_overrides ADD COLUMN base_workflow_id VARCHAR(64) NULL AFTER base_source', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c3 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_overrides' AND column_name='base_version_no');
SET @sql = IF(@c3=0, 'ALTER TABLE company_template_workflow_overrides ADD COLUMN base_version_no INT NULL AFTER base_workflow_id', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c4 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_overrides' AND column_name='base_hash');
SET @sql = IF(@c4=0, 'ALTER TABLE company_template_workflow_overrides ADD COLUMN base_hash VARCHAR(64) NULL AFTER base_version_no', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c5 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_overrides' AND column_name='stale_status');
SET @sql = IF(@c5=0, "ALTER TABLE company_template_workflow_overrides ADD COLUMN stale_status VARCHAR(16) NOT NULL DEFAULT 'unknown' AFTER base_hash", 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c6 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_overrides' AND column_name='last_rebase_check_at');
SET @sql = IF(@c6=0, 'ALTER TABLE company_template_workflow_overrides ADD COLUMN last_rebase_check_at DATETIME NULL AFTER stale_status', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

-- ─── 2. best-effort backfill: base_source/base_version_no/base_workflow_id, ONLY where a
--    global_workflow_versions row is PROVABLY active (by historical activated_at timestamp) at
--    or before the override's own created_at. This is the override-creation flow's own seeding
--    mechanism (confirmed by source: the FE seeds a new override from GetEffectiveWorkflow's
--    result at the moment of first customization), applied to immutable historical facts — not
--    an inference from current/similar content. See BACKFILL_REPORT.md for the exact rows this
--    resolves and why every other row is intentionally left 'unknown' rather than guessed as
--    'global_template'. ───
UPDATE company_template_workflow_overrides o
JOIN (
  SELECT o2.override_id AS override_id, MAX(v.version_no) AS best_version_no
  FROM company_template_workflow_overrides o2
  JOIN global_workflow_versions v
    ON v.type_id = o2.type_id
   AND v.activated_at IS NOT NULL
   AND v.activated_at <= o2.created_at
  GROUP BY o2.override_id
) best ON best.override_id = o.override_id
JOIN global_workflow_versions v2
  ON v2.type_id = o.type_id AND v2.version_no = best.best_version_no
SET
  o.base_source = 'global_workflow',
  o.base_version_no = best.best_version_no,
  o.base_workflow_id = JSON_UNQUOTE(JSON_EXTRACT(v2.steps_manifest_json, '$.workflow_id'));
-- (no WHERE guard needed for idempotency — see header comment; re-running recomputes the same
--  result from the same immutable historical inputs)

-- Record in migration tracker (matches 0091/0100/0101/0102 self-recording pattern).
INSERT INTO schema_migrations (file_name) VALUES ('0103_override_base_metadata.up.sql')
  ON DUPLICATE KEY UPDATE executed_at = CURRENT_TIMESTAMP;
