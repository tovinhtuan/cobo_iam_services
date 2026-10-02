-- Track periodic worker retries without changing record_id's completed-cycle semantics.
-- Existing rows remain PENDING and are therefore compatible with the old worker behavior.
ALTER TABLE periodic_cycles
  ADD COLUMN materialization_state VARCHAR(16) NOT NULL DEFAULT 'PENDING'
    COMMENT 'PENDING|CLAIMED|RETRY|FAILED|COMPLETED',
  ADD COLUMN attempt_record_id VARCHAR(36) NULL
    COMMENT 'Deterministic record ID reserved for a periodic cycle attempt',
  ADD COLUMN attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  ADD COLUMN next_attempt_at DATETIME(3) NULL,
  ADD COLUMN last_error_code VARCHAR(64) NULL,
  ADD COLUMN last_error_message VARCHAR(1024) NULL,
  ADD KEY idx_pc_materialization_retry (materialization_state, next_attempt_at, due_date);
