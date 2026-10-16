-- MR-3 (PO decision Q1-B): archiving a global template now suspends its cycles without a record in
-- the same transaction. Templates archived before this release kept those cycles PENDING/RETRY,
-- so a restore would revive them all (DL-05). Suspend them the same way, so a restore backfills
-- only the cycles Admin Web picks.
-- Data-only and idempotent; safe before or after the new binary: workers never claim SUSPENDED, and
-- materialized_at stays set as the fence older worker binaries check (see 0153).
UPDATE periodic_cycles pc
INNER JOIN disclosure_types dt ON dt.type_id = pc.type_id
SET pc.materialization_state = 'SUSPENDED',
    pc.materialized_at = COALESCE(pc.materialized_at, NOW(3)),
    pc.next_attempt_at = NULL,
    pc.last_error_code = 'TEMPLATE_ARCHIVED',
    pc.last_error_message = 'template archived; a restore may backfill this cycle'
WHERE LOWER(COALESCE(dt.status, '')) = 'archived'
  AND dt.company_id IS NULL
  AND pc.record_id IS NULL
  AND pc.materialization_state IN ('PENDING', 'RETRY');
