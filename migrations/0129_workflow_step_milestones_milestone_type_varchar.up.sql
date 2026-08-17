-- 0129: allow configurable due_minus_Nd reminder offsets on workflow_step_milestones.
-- Live DEV ENUM (before_start_5d/3d/1d, step_start, step_end) truncated due_minus_* to ''.
-- VARCHAR(64) covers due_minus_1d..due_minus_90d without future ENUM migrations.
-- No index on milestone_type (dispatch key is idx_wsm_dispatch). Unique remains uq_wsm_milestone_id.
-- Backfill only rows whose milestone_id encodes a source-proven due_minus_Nd
-- (buildMilestoneID embeds the type). Unknown blanks are left unchanged.

ALTER TABLE workflow_step_milestones
  MODIFY COLUMN milestone_type VARCHAR(64)
    CHARACTER SET utf8mb4
    COLLATE utf8mb4_unicode_ci
    NOT NULL;

UPDATE workflow_step_milestones
SET milestone_type = REGEXP_SUBSTR(milestone_id, 'due_minus_[1-9][0-9]*d')
WHERE milestone_type = ''
  AND milestone_id REGEXP 'due_minus_[1-9][0-9]*d';
