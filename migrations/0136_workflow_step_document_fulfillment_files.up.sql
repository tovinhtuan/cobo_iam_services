-- B2: runtime document requirement fulfillment files (append-only replace lineage).
-- ONE_TO_MANY per requirement_snapshot_id. No Complete Step gate.
CREATE TABLE IF NOT EXISTS workflow_step_document_fulfillment_files (
  id VARCHAR(36) NOT NULL,
  company_id VARCHAR(36) NOT NULL,
  disclosure_record_id VARCHAR(36) NOT NULL,
  workflow_instance_id VARCHAR(36) NOT NULL,
  step_code VARCHAR(64) NOT NULL,
  requirement_snapshot_id VARCHAR(36) NOT NULL,
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
  KEY idx_wsdff_req_lifecycle (requirement_snapshot_id, lifecycle_status, uploaded_at),
  KEY idx_wsdff_instance_step_lifecycle (workflow_instance_id, step_code, lifecycle_status),
  KEY idx_wsdff_company_id (company_id, id),
  KEY idx_wsdff_record_step (disclosure_record_id, step_code)
);
