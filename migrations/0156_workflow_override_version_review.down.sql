-- Reverts 0156. Run only after rolling the BE back and moving pending_approval / rejected versions
-- back to draft (runbook in docs/ai-cache/mr4-override-maker-checker-2026-10-16/02-contract.md).

SET NAMES utf8mb4;

SET @c1 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_override_versions' AND column_name='submitted_by');
SET @sql = IF(@c1=1, 'ALTER TABLE company_template_workflow_override_versions DROP COLUMN submitted_by', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c2 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_override_versions' AND column_name='submitted_at');
SET @sql = IF(@c2=1, 'ALTER TABLE company_template_workflow_override_versions DROP COLUMN submitted_at', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c3 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_override_versions' AND column_name='rejected_by');
SET @sql = IF(@c3=1, 'ALTER TABLE company_template_workflow_override_versions DROP COLUMN rejected_by', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c4 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_override_versions' AND column_name='rejected_at');
SET @sql = IF(@c4=1, 'ALTER TABLE company_template_workflow_override_versions DROP COLUMN rejected_at', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c5 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_override_versions' AND column_name='decision_note');
SET @sql = IF(@c5=1, 'ALTER TABLE company_template_workflow_override_versions DROP COLUMN decision_note', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c6 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_override_versions' AND column_name='submit_note');
SET @sql = IF(@c6=1, 'ALTER TABLE company_template_workflow_override_versions DROP COLUMN submit_note', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
