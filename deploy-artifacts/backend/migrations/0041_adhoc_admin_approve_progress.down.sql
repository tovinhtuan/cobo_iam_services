ALTER TABLE ad_hoc_proposals
  DROP INDEX idx_adhoc_approval_progress,
  DROP COLUMN approval_last_error,
  DROP COLUMN approval_started_at,
  DROP COLUMN approval_workflow_instance_id,
  DROP COLUMN approval_record_id,
  DROP COLUMN approval_idempotency_key;
