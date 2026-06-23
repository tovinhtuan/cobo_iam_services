-- 0104 DOWN: reverse Sprint 3 / Batch 4 (workflow override conflicts).
-- Drops only what 0104.up created. No other table is referenced by or references this one (no
-- FK), so the drop is unconditionally safe. No tenant table behavior touched in either direction.

SET NAMES utf8mb4;

DROP TABLE IF EXISTS workflow_override_conflicts;

DELETE FROM schema_migrations WHERE file_name = '0104_workflow_override_conflicts.up.sql';
