-- Effective T V1: company submission timestamp + open_at on periodic cycles.
-- Non-destructive. No historical backfill of submitted_at / open_at / due dates.

ALTER TABLE disclosure_records
  ADD COLUMN submitted_at DATETIME(3) NULL DEFAULT NULL
    COMMENT 'First explicit company submit (not materialize). NULL = not submitted.';

CREATE INDEX idx_disclosure_records_submitted_at
  ON disclosure_records (company_id, submitted_at);

ALTER TABLE periodic_cycles
  ADD COLUMN open_at DATE NULL
    COMMENT 'Business OpenAt snapshot (EffectiveT - open_days_before_t). NULL = legacy use cycle_start.';
