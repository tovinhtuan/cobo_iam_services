-- PER-01: FAILED cycles keep the legacy claim fence. Worker binaries older than cabbecf select
-- record_id IS NULL AND materialized_at IS NULL and do not know materialization_state, so a FAILED
-- row with materialized_at = NULL would be picked up again and create a Draft every tick.
-- Data-only and idempotent; safe before or after the new binary.
UPDATE periodic_cycles
SET materialized_at = NOW(3)
WHERE materialization_state = 'FAILED'
  AND materialized_at IS NULL
  AND record_id IS NULL;
