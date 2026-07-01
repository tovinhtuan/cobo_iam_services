SET NAMES utf8mb4;

-- Sprint 5 Batch 2B — M3 pending_admin_changes (configuration approval queue).

CREATE TABLE pending_admin_changes (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  company_id VARCHAR(36) NOT NULL,
  approval_subject_type VARCHAR(64) NOT NULL DEFAULT 'config_snapshot',
  aggregate_type VARCHAR(64) NOT NULL,
  aggregate_id VARCHAR(36) NOT NULL DEFAULT '',
  change_type VARCHAR(128) NOT NULL,
  proposed_snapshot_json JSON NOT NULL,
  base_live_version_no INT NULL,
  status VARCHAR(32) NOT NULL,
  requested_by VARCHAR(36) NOT NULL,
  requested_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  reviewed_by VARCHAR(36) NULL,
  reviewed_at TIMESTAMP NULL,
  reason VARCHAR(512) NULL,
  reject_reason VARCHAR(512) NULL,
  KEY idx_pending_company_status (company_id, status, requested_at),
  KEY idx_pending_aggregate_stream (company_id, aggregate_type, aggregate_id, status),
  CONSTRAINT fk_pending_admin_changes_company FOREIGN KEY (company_id) REFERENCES companies(company_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
