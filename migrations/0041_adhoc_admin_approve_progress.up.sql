ALTER TABLE ad_hoc_proposals
  ADD COLUMN approval_idempotency_key VARCHAR(191) NULL AFTER workflow_instance_id,
  ADD COLUMN approval_record_id VARCHAR(36) NULL AFTER approval_idempotency_key,
  ADD COLUMN approval_workflow_instance_id VARCHAR(36) NULL AFTER approval_record_id,
  ADD COLUMN approval_started_at DATETIME(3) NULL AFTER approval_workflow_instance_id,
  ADD COLUMN approval_last_error VARCHAR(255) NULL AFTER approval_started_at,
  ADD KEY idx_adhoc_approval_progress (company_id, status, approval_started_at);
