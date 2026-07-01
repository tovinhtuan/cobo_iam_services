SET NAMES utf8mb4;

-- Sprint 5 Batch 3B — delegated administration grants (department-scoped membership).

CREATE TABLE delegated_admin_grants (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  company_id VARCHAR(36) NOT NULL,
  delegatee_membership_id VARCHAR(36) NOT NULL,
  delegator_membership_id VARCHAR(36) NOT NULL,
  scope_type VARCHAR(32) NOT NULL,
  scope_id VARCHAR(36) NOT NULL,
  permission_set_json JSON NOT NULL,
  status VARCHAR(16) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_by VARCHAR(36) NOT NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  updated_by VARCHAR(36) NOT NULL,
  KEY idx_delegation_company_status (company_id, status, created_at),
  KEY idx_delegation_delegatee (company_id, delegatee_membership_id, status),
  KEY idx_delegation_scope (company_id, scope_type, scope_id, status),
  CONSTRAINT fk_delegation_company FOREIGN KEY (company_id) REFERENCES companies(company_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
