SET NAMES utf8mb4;

-- email_delivery_attempts records every Send invocation by the worker so
-- support can answer "did we try to deliver, and what happened?" without
-- grepping logs. Exactly one row per attempt; failure rows include the
-- redacted error code so dashboards can aggregate without leaking PII.
CREATE TABLE email_delivery_attempts (
  email_delivery_attempt_id    VARCHAR(36)  NOT NULL,
  email_notification_id        VARCHAR(36)  NOT NULL,
  attempt_no                   TINYINT UNSIGNED NOT NULL,
  provider                     VARCHAR(32)  NOT NULL,
  status                       ENUM('sent','retry','failed_permanent','render_error') NOT NULL,
  smtp_response_code           SMALLINT     NULL,
  error_code                   VARCHAR(64)  NULL,
  error_message_redacted       VARCHAR(1024) NULL,
  started_at                   DATETIME(3)  NOT NULL,
  finished_at                  DATETIME(3)  NOT NULL,
  next_retry_at                DATETIME(3)  NULL,
  created_at                   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (email_delivery_attempt_id),
  UNIQUE KEY uk_email_delivery_attempts_notif_attempt (email_notification_id, attempt_no),
  KEY idx_email_delivery_attempts_notif_created (email_notification_id, created_at),
  KEY idx_email_delivery_attempts_status_created (status, created_at),
  CONSTRAINT fk_email_delivery_attempts_notif FOREIGN KEY (email_notification_id)
    REFERENCES email_notifications(email_notification_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
