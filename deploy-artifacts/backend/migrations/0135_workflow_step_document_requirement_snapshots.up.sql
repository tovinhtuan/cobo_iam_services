-- B1: immutable document requirement snapshots frozen at workflow instance creation.
-- No backfill of existing instances. No fulfillment/evidence files.
CREATE TABLE IF NOT EXISTS workflow_step_document_requirement_snapshots (
  id VARCHAR(36) NOT NULL,
  company_id VARCHAR(36) NOT NULL,
  disclosure_record_id VARCHAR(36) NOT NULL,
  workflow_instance_id VARCHAR(36) NOT NULL,
  step_code VARCHAR(64) NOT NULL,
  source_doc_id VARCHAR(128) NOT NULL,
  requirement_key VARCHAR(128) NOT NULL,
  name VARCHAR(512) NOT NULL,
  required TINYINT(1) NOT NULL DEFAULT 0,
  template_file_id VARCHAR(64) NULL,
  template_file_name VARCHAR(255) NULL,
  ordinal INT NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_wsdrs_instance_step_doc (workflow_instance_id, step_code, source_doc_id),
  KEY idx_wsdrs_instance_step (workflow_instance_id, step_code),
  KEY idx_wsdrs_company_record_step (company_id, disclosure_record_id, step_code)
);
