-- 0121: Add step description ("Mô tả bước") alongside instructions ("Hướng dẫn thực hiện").
-- Additive, nullable, no backfill. Existing instructions stay as-is (Hướng dẫn thực hiện).

SET NAMES utf8mb4;

ALTER TABLE global_workflow_steps
  ADD COLUMN description TEXT NULL AFTER stage;
