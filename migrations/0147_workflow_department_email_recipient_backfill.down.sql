-- 0147 down: drop only the unique index this migration owns.
-- Does not null updated_at (that value may now be business data) and does not
-- drop the occurrence unique key created in 0144 or any ciphertext.
-- PRODUCTION_ROLLBACK=forward_fix_or_flag_off
-- DEV_DOWN=DROP_OWNED_INDEX_ONLY

SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS cobo_0147_email_delivery_safety;

SET @idx = (
  SELECT COUNT(1) FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'reminder_dispatch_resolutions'
    AND index_name = 'uk_reminder_dispatch_occurrence_company'
);
SET @sql = IF(@idx > 0,
  'ALTER TABLE reminder_dispatch_resolutions DROP INDEX uk_reminder_dispatch_occurrence_company',
  'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
