-- 0099: Add free-text implementation guidance ("Hướng dẫn thực hiện") to global workflow steps.
--
-- Consumed by the email reminder content upgrade: step-scoped reminder emails surface
-- this text in the "Hướng dẫn thực hiện" section. Owned by Platform Admin via the CMS
-- global workflow editor.
--
-- Data safety: additive, nullable, no backfill, no data loss. Reversible (see down).
-- NOTE: MySQL 8.0 does NOT support ADD COLUMN IF NOT EXISTS (MariaDB-only); not idempotent.

SET NAMES utf8mb4;

ALTER TABLE global_workflow_steps
  ADD COLUMN instructions TEXT NULL AFTER stage;
