-- Per-step runtime state for deadline detail workflow (complete / mark-incomplete / delay shift).

CREATE TABLE IF NOT EXISTS workflow_instance_step_states (
  company_id VARCHAR(64) NOT NULL,
  workflow_instance_id VARCHAR(64) NOT NULL,
  step_code VARCHAR(128) NOT NULL,
  completed_at DATETIME NULL,
  completed_by_membership_id VARCHAR(64) NULL,
  marked_incomplete_at DATETIME NULL,
  marked_incomplete_by_membership_id VARCHAR(64) NULL,
  incomplete_reason VARCHAR(2000) NULL,
  delay_days_applied INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (workflow_instance_id, step_code),
  KEY idx_wiss_company_instance (company_id, workflow_instance_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
