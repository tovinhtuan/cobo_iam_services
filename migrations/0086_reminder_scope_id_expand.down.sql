ALTER TABLE reminder_configs
    MODIFY COLUMN scope_id VARCHAR(64) NOT NULL;

ALTER TABLE reminder_occurrences
    MODIFY COLUMN scope_id VARCHAR(64) NOT NULL;
