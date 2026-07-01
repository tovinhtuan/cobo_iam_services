SET NAMES utf8mb4;

-- Sprint 5 Batch 4B — M4 emergency_access_grants (break glass overlay).

CREATE TABLE emergency_access_grants (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  company_id VARCHAR(36) NOT NULL,
  target_membership_id VARCHAR(36) NOT NULL,
  requester_membership_id VARCHAR(36) NOT NULL,
  approver_membership_id_1 VARCHAR(36) NULL,
  approver_membership_id_2 VARCHAR(36) NULL,
  reason VARCHAR(512) NOT NULL,
  scope VARCHAR(64) NOT NULL DEFAULT 'company',
  capability_set_json JSON NOT NULL,
  requested_duration_seconds INT NOT NULL DEFAULT 14400,
  status VARCHAR(32) NOT NULL,
  requested_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  activated_at TIMESTAMP NULL,
  expires_at TIMESTAMP NULL,
  revoked_at TIMESTAMP NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_emergency_company_status (company_id, status, requested_at),
  KEY idx_emergency_target_active (company_id, target_membership_id, status),
  CONSTRAINT fk_emergency_access_company FOREIGN KEY (company_id) REFERENCES companies(company_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
