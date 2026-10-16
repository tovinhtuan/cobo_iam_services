-- 0156: MR-4 (PO decision Q5-B) — maker-checker for company workflow overrides by permission.
--
-- A writer submits a draft version for approval (state 'pending_approval'); an approver activates
-- it ('approved') or rejects it ('rejected', with a reason). state is VARCHAR(32), so the new
-- values need no change to the column. These nullable columns record who submitted or rejected a
-- version, when, the submitter's note (submit_note) and the rejection reason (decision_note).
--
-- Data safety: ADDITIVE ONLY. No existing column is changed, no row is touched. Each ADD COLUMN is
-- guarded through information_schema (MySQL 8.0 has no ADD COLUMN IF NOT EXISTS), so a rerun is a
-- no-op. Columns are appended (no AFTER) so MySQL 8.0 can add them in place without a table copy.
-- Reversible: 0156_workflow_override_version_review.down.sql drops the columns.

SET NAMES utf8mb4;

SET @c1 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_override_versions' AND column_name='submitted_by');
SET @sql = IF(@c1=0, 'ALTER TABLE company_template_workflow_override_versions ADD COLUMN submitted_by VARCHAR(64) NULL', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c2 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_override_versions' AND column_name='submitted_at');
SET @sql = IF(@c2=0, 'ALTER TABLE company_template_workflow_override_versions ADD COLUMN submitted_at TIMESTAMP NULL', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c3 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_override_versions' AND column_name='rejected_by');
SET @sql = IF(@c3=0, 'ALTER TABLE company_template_workflow_override_versions ADD COLUMN rejected_by VARCHAR(64) NULL', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c4 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_override_versions' AND column_name='rejected_at');
SET @sql = IF(@c4=0, 'ALTER TABLE company_template_workflow_override_versions ADD COLUMN rejected_at TIMESTAMP NULL', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c5 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_override_versions' AND column_name='decision_note');
SET @sql = IF(@c5=0, 'ALTER TABLE company_template_workflow_override_versions ADD COLUMN decision_note VARCHAR(500) NULL', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @c6 = (SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='company_template_workflow_override_versions' AND column_name='submit_note');
SET @sql = IF(@c6=0, 'ALTER TABLE company_template_workflow_override_versions ADD COLUMN submit_note VARCHAR(500) NULL', 'SELECT 1');
PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
