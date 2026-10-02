ALTER TABLE periodic_cycles
  DROP INDEX idx_pc_materialization_retry,
  DROP COLUMN last_error_message,
  DROP COLUMN last_error_code,
  DROP COLUMN next_attempt_at,
  DROP COLUMN attempt_count,
  DROP COLUMN attempt_record_id,
  DROP COLUMN materialization_state;
