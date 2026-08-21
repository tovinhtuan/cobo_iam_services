-- Rollback Effective T V1 additive columns.

ALTER TABLE periodic_cycles DROP COLUMN open_at;

DROP INDEX idx_disclosure_records_submitted_at ON disclosure_records;

ALTER TABLE disclosure_records DROP COLUMN submitted_at;
