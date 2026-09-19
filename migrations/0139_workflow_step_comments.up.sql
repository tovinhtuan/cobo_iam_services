-- 0139: workflow_step_comments — Tenant Portal Trao đổi (step-scoped comments).
-- Soft-delete; no FK. Indexes lead with company_id per locked contract.
-- DEV/test down only; production rollback = forward-fix.

SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS workflow_step_comments (
  id VARCHAR(36) NOT NULL,
  company_id VARCHAR(36) NOT NULL,
  disclosure_record_id VARCHAR(36) NOT NULL,
  workflow_instance_id VARCHAR(36) NOT NULL,
  step_code VARCHAR(64) NOT NULL,
  author_user_id VARCHAR(36) NOT NULL,
  author_membership_id VARCHAR(36) NOT NULL,
  body VARCHAR(4000) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  deleted_by_membership_id VARCHAR(36) NULL,
  PRIMARY KEY (id),
  KEY idx_wsc_company_instance_step_alive_created
    (company_id, workflow_instance_id, step_code, deleted_at, created_at, id),
  KEY idx_wsc_company_record_step_alive_created
    (company_id, disclosure_record_id, step_code, deleted_at, created_at, id),
  KEY idx_wsc_company_id (company_id, id)
);
