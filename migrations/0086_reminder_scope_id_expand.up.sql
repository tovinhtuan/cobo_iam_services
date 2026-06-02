-- 0086: Widen scope_id from VARCHAR(64) to VARCHAR(128)
-- Composite key disclosureID:stepID = 36 + 1 + 29 = 66 chars, exceeds old 64-char limit.
SET NAMES utf8mb4;

ALTER TABLE reminder_configs
    MODIFY COLUMN scope_id VARCHAR(128) NOT NULL;

ALTER TABLE reminder_occurrences
    MODIFY COLUMN scope_id VARCHAR(128) NOT NULL;
