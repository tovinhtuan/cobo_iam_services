SET NAMES utf8mb4;

-- Sprint 5 Batch 1B — M1 notification_rule_versions, M2 rbac_matrix_snapshots (ADR-012).

CREATE TABLE notification_rule_versions (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  company_id VARCHAR(36) NOT NULL,
  rule_id VARCHAR(36) NOT NULL,
  version_no INT NOT NULL,
  snapshot_json JSON NOT NULL,
  created_by VARCHAR(36) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  reason VARCHAR(512) NULL,
  source VARCHAR(128) NOT NULL,
  UNIQUE KEY uk_notif_rule_versions_company_rule_ver (company_id, rule_id, version_no),
  KEY idx_notif_rule_versions_company_rule (company_id, rule_id, created_at),
  CONSTRAINT fk_notif_rule_versions_company FOREIGN KEY (company_id) REFERENCES companies(company_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE rbac_matrix_snapshots (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  company_id VARCHAR(36) NOT NULL,
  version_no INT NOT NULL,
  snapshot_json JSON NOT NULL,
  created_by VARCHAR(36) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  reason VARCHAR(512) NULL,
  source VARCHAR(128) NOT NULL,
  UNIQUE KEY uk_rbac_matrix_snapshots_company_ver (company_id, version_no),
  KEY idx_rbac_matrix_snapshots_company (company_id, created_at),
  CONSTRAINT fk_rbac_matrix_snapshots_company FOREIGN KEY (company_id) REFERENCES companies(company_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
