-- 0147: fill updated_at only when still NULL. Do not invent email_key_version.
-- Unique index uk_reminder_dispatch_occurrence_company is created only after
-- a zero duplicate (company_id, occurrence_id) count. The occurrence unique key from 0144
-- is left unchanged. No recipient plaintext is selected or logged.
-- SIGNAL is a stored-program statement. MySQL 8 does not allow SIGNAL in PREPARE.
-- Does not enable flags.

SET NAMES utf8mb4;

DROP PROCEDURE IF EXISTS cobo_0147_email_delivery_safety;

DELIMITER $$
CREATE PROCEDURE cobo_0147_email_delivery_safety()
BEGIN
  DECLARE dup_count INT DEFAULT 0;
  DECLARE idx_count INT DEFAULT 0;

  SELECT COUNT(*) INTO dup_count
  FROM (
    SELECT company_id, occurrence_id
    FROM reminder_dispatch_resolutions
    GROUP BY company_id, occurrence_id
    HAVING COUNT(*) > 1
  ) d;

  IF dup_count > 0 THEN
    SIGNAL SQLSTATE '45000'
      SET MESSAGE_TEXT = 'duplicate company_id occurrence_id; unique index not created';
  END IF;

  UPDATE reminder_dispatch_resolutions
  SET updated_at = resolved_at
  WHERE updated_at IS NULL;

  SELECT COUNT(*) INTO idx_count
  FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 'reminder_dispatch_resolutions'
    AND index_name = 'uk_reminder_dispatch_occurrence_company';

  IF idx_count = 0 THEN
    ALTER TABLE reminder_dispatch_resolutions
      ADD UNIQUE KEY uk_reminder_dispatch_occurrence_company (company_id, occurrence_id);
  END IF;
END$$
DELIMITER ;

CALL cobo_0147_email_delivery_safety();
DROP PROCEDURE IF EXISTS cobo_0147_email_delivery_safety;
