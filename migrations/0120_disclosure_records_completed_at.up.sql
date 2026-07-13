-- Personal Ops on_time_rate: forward-only outcome timestamp on disclosure_records.
-- No historical backfill. completed_at is set only on terminal transitions after this migration.

ALTER TABLE disclosure_records
  ADD COLUMN completed_at DATETIME(3) NULL DEFAULT NULL,
  ADD COLUMN completed_source VARCHAR(64) NULL DEFAULT NULL;

-- Supports mine on_time queries filtering terminal status + completed_at.
CREATE INDEX idx_disclosure_records_status_completed
  ON disclosure_records (status, completed_at);
