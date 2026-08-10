SET NAMES utf8mb4;

SET @col = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 'ad_hoc_proposals'
    AND column_name = 'proposed_deadline_day_type'
);

SET @sql = IF(
  @col > 0,
  'ALTER TABLE ad_hoc_proposals DROP COLUMN proposed_deadline_day_type',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
