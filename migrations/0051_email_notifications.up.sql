SET NAMES utf8mb4;

-- email_notifications is the phase-2 persistence layer for the new email
-- dispatch pipeline. The shadow rollout writes a row per dispatch while
-- legacy auth/reminder delivery continues unchanged; phase 3 will start
-- consuming these rows via the worker handler.
--
-- Sensitive variables (OTP code, reset link, raw token, setup link) MUST be
-- redacted before being stored in variables_json_sanitized. The renderer
-- still sees the plain values in memory; the on-disk record never does.
CREATE TABLE email_notifications (
  email_notification_id        VARCHAR(36)  NOT NULL,
  company_id                   VARCHAR(36)  NULL,
  recipient_email              VARCHAR(320) NOT NULL,
  template_key                 VARCHAR(128) NOT NULL,
  locale                       VARCHAR(16)  NOT NULL,
  status                       ENUM('pending','sending','sent','retry','failed_permanent','cancelled')
                                            NOT NULL DEFAULT 'pending',
  idempotency_key              VARCHAR(256) NOT NULL,
  triggered_by_user_id         VARCHAR(36)  NULL,
  source_event_type            VARCHAR(128) NULL,
  source_aggregate_type        VARCHAR(64)  NULL,
  source_aggregate_id          VARCHAR(64)  NULL,
  variables_json_sanitized     JSON         NOT NULL,
  scheduled_at                 DATETIME(3)  NULL,
  last_error_code              VARCHAR(64)  NULL,
  last_error_message_redacted  VARCHAR(1024) NULL,
  sent_at                      DATETIME(3)  NULL,
  created_at                   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at                   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
                                            ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (email_notification_id),
  UNIQUE KEY uk_email_notifications_idempotency (idempotency_key),
  KEY idx_email_notifications_status_scheduled (status, scheduled_at, created_at),
  KEY idx_email_notifications_template_created (template_key, created_at),
  KEY idx_email_notifications_company_created (company_id, created_at),
  KEY idx_email_notifications_recipient_created (recipient_email, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
