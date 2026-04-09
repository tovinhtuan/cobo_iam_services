SET NAMES utf8mb4;

-- Persist admin-created workflow assignee and notification rules (HTTP admin APIs).
CREATE TABLE workflow_assignee_rules (
  workflow_assignee_rule_id VARCHAR(36) PRIMARY KEY,
  company_id         VARCHAR(36) NOT NULL,
  rule_code          VARCHAR(128) NOT NULL,
  payload_json       JSON NOT NULL,
  status             VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_wf_assignee_rules_company_code (company_id, rule_code),
  KEY idx_wf_assignee_rules_company_status (company_id, status),
  CONSTRAINT fk_wf_assignee_rules_company FOREIGN KEY (company_id) REFERENCES companies(company_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE notification_rules (
  notification_rule_id VARCHAR(36) PRIMARY KEY,
  company_id         VARCHAR(36) NOT NULL,
  rule_code          VARCHAR(128) NOT NULL,
  payload_json       JSON NOT NULL,
  status             VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_notification_rules_company_code (company_id, rule_code),
  KEY idx_notification_rules_company_status (company_id, status),
  CONSTRAINT fk_notification_rules_company FOREIGN KEY (company_id) REFERENCES companies(company_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
