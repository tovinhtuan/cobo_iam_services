-- G2A: generic workflow step evidence files (not document requirement fulfillment).
-- Owner: company + disclosure_record + workflow_instance + step.
-- ONE_TO_MANY per (workflow_instance_id, step_code). Snapshot-independent (no B1 parent).
-- Does not participate in B3 Complete required-document gate. No historical backfill.
CREATE TABLE IF NOT EXISTS workflow_step_evidence_files (
  id VARCHAR(36) NOT NULL,
  company_id VARCHAR(36) NOT NULL,
  disclosure_record_id VARCHAR(36) NOT NULL,
  workflow_instance_id VARCHAR(36) NOT NULL,
  step_code VARCHAR(64) NOT NULL,
  storage_key VARCHAR(512) NOT NULL,
  original_file_name VARCHAR(255) NOT NULL,
  mime_type VARCHAR(128) NOT NULL,
  file_size BIGINT NOT NULL,
  uploaded_by VARCHAR(36) NOT NULL,
  uploaded_at DATETIME(3) NOT NULL,
  lifecycle_status VARCHAR(16) NOT NULL,
  supersedes_file_id VARCHAR(36) NULL,
  superseded_by_file_id VARCHAR(36) NULL,
  deleted_at DATETIME(3) NULL,
  deleted_by VARCHAR(36) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_wsef_instance_step_lifecycle (workflow_instance_id, step_code, lifecycle_status, uploaded_at),
  KEY idx_wsef_company_id (company_id, id),
  KEY idx_wsef_record_step (disclosure_record_id, step_code),
  KEY idx_wsef_supersedes (supersedes_file_id)
);
