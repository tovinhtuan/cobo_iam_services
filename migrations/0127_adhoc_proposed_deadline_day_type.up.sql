-- 0127: Proposal-owned deadline day kind (WORKING_DAYS | CALENDAR_DAYS).
-- NULL = legacy/current CALENDAR_DAYS semantics (no backfill).
-- Marker: MIGRATION_SOURCE_CREATED_NOT_APPLIED / PROPOSED_DEADLINE_DAY_TYPE_NULLABLE / NO_BLIND_BACKFILL

SET NAMES utf8mb4;

SET @col = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'ad_hoc_proposals'
    AND column_name = 'proposed_deadline_day_type'
);

SET @sql = IF(
  @col = 0,
  'ALTER TABLE ad_hoc_proposals ADD COLUMN proposed_deadline_day_type VARCHAR(32) NULL AFTER proposed_deadline_days',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
