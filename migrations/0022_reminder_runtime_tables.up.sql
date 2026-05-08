SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS reminder_configs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  disclosure_id VARCHAR(64) NOT NULL,
  scope_type ENUM('DISCLOSURE','WORKFLOW_STEP') NOT NULL,
  scope_id VARCHAR(64) NOT NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  mode ENUM('DaysBefore','SpecificDate') NOT NULL,
  days_before_json JSON NULL,
  specific_dates_json JSON NULL,
  recipient_type ENUM('Departments','Individuals','Both') NOT NULL,
  departments_json JSON NULL,
  recipients_json JSON NULL,
  version INT UNSIGNED NOT NULL DEFAULT 1,
  updated_by VARCHAR(128) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_reminder_configs_scope (scope_type, scope_id),
  KEY idx_reminder_configs_disclosure (disclosure_id),
  KEY idx_reminder_configs_enabled_mode (enabled, mode)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS reminder_occurrences (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  occurrence_id VARCHAR(80) NOT NULL,
  disclosure_id VARCHAR(64) NOT NULL,
  scope_type ENUM('DISCLOSURE','WORKFLOW_STEP') NOT NULL,
  scope_id VARCHAR(64) NOT NULL,
  scheduled_at DATETIME(3) NOT NULL,
  status ENUM('PENDING','DISPATCHING','RETRY_SCHEDULED','SENT','FAILED') NOT NULL DEFAULT 'PENDING',
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  idempotency_key VARCHAR(160) NOT NULL,
  next_retry_at DATETIME(3) NULL,
  last_attempt_at DATETIME(3) NULL,
  last_error_code VARCHAR(64) NULL,
  last_error_message TEXT NULL,
  provider_message_id VARCHAR(255) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_reminder_occurrence_id (occurrence_id),
  UNIQUE KEY uq_reminder_occurrence_idempotency (idempotency_key),
  KEY idx_reminder_occurrence_disclosure_sched (disclosure_id, scheduled_at),
  KEY idx_reminder_occurrence_status_sched (status, scheduled_at),
  KEY idx_reminder_occurrence_status_retry (status, next_retry_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS reminder_delivery_attempts (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  occurrence_id VARCHAR(80) NOT NULL,
  attempt_no INT UNSIGNED NOT NULL,
  status ENUM('PENDING','DISPATCHING','RETRY_SCHEDULED','SENT','FAILED') NOT NULL,
  provider_message_id VARCHAR(255) NULL,
  error_code VARCHAR(64) NULL,
  error_message TEXT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_reminder_attempt_occurrence_no (occurrence_id, attempt_no),
  KEY idx_reminder_attempt_occurrence (occurrence_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
