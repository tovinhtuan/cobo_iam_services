SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS deadline_alert_confirmations (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  company_id VARCHAR(64) NOT NULL,
  record_id VARCHAR(64) NOT NULL,
  confirmed_by VARCHAR(128) NOT NULL,
  confirmed_at DATETIME(3) NOT NULL,
  confirm_note TEXT NULL,
  idempotency_key VARCHAR(191) NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_deadline_alert_confirm (company_id, record_id),
  KEY idx_deadline_alert_confirm_company_time (company_id, confirmed_at),
  KEY idx_deadline_alert_confirm_idem (company_id, idempotency_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
